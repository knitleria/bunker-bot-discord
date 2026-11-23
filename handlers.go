package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// Response state belongs to one interaction, never to the shared Bot.
type interactionResponder struct {
	session      *discordgo.Session
	interaction  *discordgo.InteractionCreate
	acknowledged bool
}

func (r *interactionResponder) respond(content string) error {
	if r.acknowledged {
		return r.editResponse(content)
	}
	err := r.session.InteractionRespond(r.interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		log.Printf("Discord initial response failed: %v", err)
		return err
	}
	r.acknowledged = true
	return nil
}

func (r *interactionResponder) deferResponse() error {
	if r.acknowledged {
		return fmt.Errorf("interaction already acknowledged")
	}
	err := r.session.InteractionRespond(r.interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
	if err != nil {
		log.Printf("Discord deferred response failed: %v", err)
		return err
	}
	r.acknowledged = true
	return nil
}

func (r *interactionResponder) editResponse(content string) error {
	if !r.acknowledged {
		return fmt.Errorf("interaction has not been acknowledged")
	}
	_, err := r.session.InteractionResponseEdit(r.interaction.Interaction, &discordgo.WebhookEdit{Content: &content})
	if err != nil {
		log.Printf("Discord response edit failed: %v", err)
	}
	return err
}

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil || i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	r := &interactionResponder{session: s, interaction: i}
	defer func() {
		if reason := recover(); reason != nil {
			log.Printf("Panic handling interaction %s: %v", i.ID, reason)
			r.respond("An internal error occurred. Please try again.")
		}
	}()

	name := i.ApplicationCommandData().Name
	if name != "whoami" && i.GuildID == "" {
		r.respond("This command can only be used in a server.")
		return
	}
	switch name {
	case "whoami":
		b.onWhoami(r)
	case "upload":
		b.onUpload(r)
	case "players":
		b.onPlayers(r)
	case "mystats":
		b.onMyStats(r)
	case "join":
		b.onJoin(r)
	case "generate":
		b.onGenerate(r)
	case "preview":
		b.onPreview(r)
	case "regenerate":
		b.onRegenerate(r)
	default:
		r.respond("Unknown or unavailable command.")
	}
}

func (b *Bot) isAdmin(i *discordgo.InteractionCreate) bool {
	if i.Member == nil || i.Member.User == nil {
		return false
	}
	return i.Member.Permissions&discordgo.PermissionAdministrator != 0 || b.Config.AdminUserIds[i.Member.User.ID]
}

func (b *Bot) requireAdmin(r *interactionResponder) bool {
	if !b.isAdmin(r.interaction) {
		r.respond("You are not authorized to use this command")
		return false
	}
	return true
}

func options(i *discordgo.InteractionCreate) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, option := range i.ApplicationCommandData().Options {
		if option != nil {
			m[option.Name] = option
		}
	}
	return m
}

func interactionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member != nil {
		return i.Member.User
	}
	return i.User
}

func (b *Bot) onWhoami(r *interactionResponder) {
	user := interactionUser(r.interaction)
	if user == nil {
		r.respond("Unable to identify your Discord user.")
		return
	}
	r.respond(fmt.Sprintf("Your Discord user information:\nUsername: %s\nID: %s", user.Username, user.ID))
}

func (b *Bot) onUpload(r *interactionResponder) {
	if !b.requireAdmin(r) {
		return
	}
	i := r.interaction
	option := options(i)["file"]
	if option == nil || option.Type != discordgo.ApplicationCommandOptionAttachment {
		r.respond("Please attach a JSON config file.")
		return
	}
	attID, ok := option.Value.(string)
	resolved := i.ApplicationCommandData().Resolved
	if !ok || attID == "" || resolved == nil || resolved.Attachments[attID] == nil || resolved.Attachments[attID].URL == "" {
		r.respond("The config attachment is missing.")
		return
	}
	if err := r.deferResponse(); err != nil {
		return
	}
	data, err := downloadAttachment(resolved.Attachments[attID].URL)
	if err != nil {
		r.editResponse("Error downloading attachment: " + err.Error())
		return
	}
	cf, err := parseConfigFile(data)
	if err != nil {
		r.editResponse("Error parsing config: " + err.Error())
		return
	}
	if err := b.store.Update(i.GuildID, func(g *Game) error {
		g.Players = cf.Players
		g.Pools = cf.Pools
		g.Started = false
		return nil
	}); err != nil {
		r.editResponse("Error updating store: " + err.Error())
		return
	}
	r.editResponse("Attachment downloaded successfully")
}

func (b *Bot) onPlayers(r *interactionResponder) {
	if !b.requireAdmin(r) {
		return
	}
	if err := r.deferResponse(); err != nil {
		return
	}
	var text string
	if err := b.store.Read(r.interaction.GuildID, func(g *Game) error {
		var sb strings.Builder
		fmt.Fprintln(&sb, "Players:")
		for _, player := range g.Players {
			fmt.Fprintf(&sb, "%s: %s\n", player.Nickname, player.Stats)
		}
		text = sb.String()
		return nil
	}); err != nil {
		r.editResponse("Error: " + err.Error())
		return
	}
	r.editResponse(text)
}

func (b *Bot) onMyStats(r *interactionResponder) {
	u := interactionUser(r.interaction)
	if u == nil {
		r.respond("Unable to identify your Discord user.")
		return
	}
	if err := r.deferResponse(); err != nil {
		return
	}
	msg := "You are not a player in this game"
	if err := b.store.Read(r.interaction.GuildID, func(g *Game) error {
		for _, player := range g.Players {
			if player.DiscordID == u.ID {
				if len(player.Stats) == 0 {
					msg = "You have not set your stats yet"
				} else {
					msg = formatStats(player)
				}
				return nil
			}
		}
		return nil
	}); err != nil {
		r.editResponse("Error: " + err.Error())
		return
	}
	r.editResponse(msg)
}

func (b *Bot) onJoin(r *interactionResponder) {
	i := r.interaction
	option := options(i)["nickname"]
	if option == nil || option.Type != discordgo.ApplicationCommandOptionString {
		r.respond("Please provide a nickname.")
		return
	}
	nick, ok := option.Value.(string)
	nick = strings.TrimSpace(nick)
	if !ok || nick == "" {
		r.respond("Please provide a nonempty nickname.")
		return
	}
	u := interactionUser(i)
	if u == nil {
		r.respond("Unable to identify your Discord user.")
		return
	}
	if err := r.deferResponse(); err != nil {
		return
	}
	var found bool
	if err := b.store.Update(i.GuildID, func(g *Game) error {
		for _, p := range g.Players {
			if strings.EqualFold(p.Nickname, nick) {
				p.DiscordID = u.ID
				found = true
				break
			}
		}
		return nil
	}); err != nil {
		r.editResponse("Error: " + err.Error())
		return
	}
	if found {
		r.editResponse("You have joined the game as " + nick)
	} else {
		r.editResponse("You are not a player in this game")
	}
}

func (b *Bot) onGenerate(r *interactionResponder) {
	b.generateCards(r, false)
}

func (b *Bot) onRegenerate(r *interactionResponder) {
	b.generateCards(r, true)
}

func (b *Bot) generateCards(r *interactionResponder, regenerate bool) {
	if !b.requireAdmin(r) {
		return
	}
	if err := r.deferResponse(); err != nil {
		return
	}
	if err := b.store.Update(r.interaction.GuildID, func(g *Game) error {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		if regenerate {
			return rerollGroup(g.Players, "all", g.Pools, rng)
		}
		return dealAll(g.Players, g.Pools, rng)
	}); err != nil {
		r.editResponse("Error: " + err.Error())
		return
	}
	r.editResponse("Character cards generated successfully. Use /preview to preview the cards.")
}

func (b *Bot) onPreview(r *interactionResponder) {
	if !b.requireAdmin(r) {
		return
	}
	if err := r.deferResponse(); err != nil {
		return
	}
	var text string
	if err := b.store.Read(r.interaction.GuildID, func(g *Game) error {
		text = formatBoard(g.Players)
		return nil
	}); err != nil {
		r.editResponse("Error: " + err.Error())
		return
	}
	r.editResponse(text)
}
