package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Error in onInteraction:", r)
		}
	}()

	command := i.ApplicationCommandData()
	commandName := command.Name

	switch commandName {
	//case "ping":
	//b.onPing(s, i)
	case "whoami":
		b.onWhoami(s, i)
	case "upload":
		b.onUpload(s, i)
	case "players":
		b.onPlayers(s, i)
	case "mystats":
		b.onMyStats(s, i)
	case "join":
		b.onJoin(s, i)
	case "generate":
		b.onGenerate(s, i)
	case "preview":
		b.onPreview(s, i)
	case "regenerate":
		b.onRegenerate(s, i)
		//case "reroll":
		//b.onReroll(s, i)
		//case "start":
		//b.onStart(s, i)
		//case "reset":
		//b.onReset(s, i)
	}
}

func (b *Bot) isAdmin(i *discordgo.InteractionCreate) bool {
	if i.Member == nil {
		return false
	}

	if i.Member.Permissions&discordgo.PermissionAdministrator != 0 {
		return true
	}
	return b.Config.AdminUserIds[i.Member.User.ID]
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func deferResponse(s *discordgo.Session, i *discordgo.InteractionCreate) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
}

func editResponse(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
}

func options(i *discordgo.InteractionCreate) map[string]*discordgo.ApplicationCommandInteractionDataOption {
	m := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, option := range i.ApplicationCommandData().Options {
		m[option.Name] = option
	}
	return m
}

func interactionUser(i *discordgo.InteractionCreate) *discordgo.User {
	if i.Member == nil {
		return i.User
	}
	return i.Member.User
}

func (b *Bot) onWhoami(s *discordgo.Session, i *discordgo.InteractionCreate) {
	user := interactionUser(i)
	respond(s, i, fmt.Sprintf("Your Discord user information:\nUsername: %s\nID: %s", user.Username, user.ID))
}

func (b *Bot) onUpload(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.isAdmin(i) {
		respond(s, i, "You are not authorized to use this command")
		return
	}

	deferResponse(s, i)

	attID := options(i)["file"].Value.(string)
	att := i.ApplicationCommandData().Resolved.Attachments[attID]
	data, err := downloadAttachment(att.URL)
	if err != nil {
		respond(s, i, "Error downloading attachment: "+err.Error())
		return
	}

	cf, err := parseConfigFile(data)
	if err != nil {
		respond(s, i, "Error parsing config: "+err.Error())
		return
	}

	if err := b.store.Update(i.GuildID, func(g *Game) error {
		g.Players = cf.Players
		g.Pools = cf.Pools
		g.Started = false
		return nil
	}); err != nil {
		respond(s, i, "Error updating store: "+err.Error())
		return
	}

	respond(s, i, "Attachment downloaded successfully")
}

func (b *Bot) onPlayers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.isAdmin(i) {
		respond(s, i, "You are not authorized to use this command")
		return
	}

	deferResponse(s, i)

	var text string

	b.store.Read(i.GuildID, func(g *Game) error {
		var sb strings.Builder
		fmt.Fprintf(&sb, "Players:\n")
		for _, player := range g.Players {
			fmt.Fprintf(&sb, "%s: %s\n", player.Nickname, player.Stats)
		}
		text = sb.String()
		return nil
	})

	editResponse(s, i, text)
}

func (b *Bot) onMyStats(s *discordgo.Session, i *discordgo.InteractionCreate) {
	u := interactionUser(i)
	var msg string
	b.store.Read(i.GuildID, func(g *Game) error {
		for _, player := range g.Players {
			if player.DiscordID == u.ID {
				if len(player.Stats) == 0 {
					msg = "You have not set your stats yet"
					return nil
				}
				msg = formatStats(player)
				return nil
			}
		}
		msg = "You are not a player in this game"
		return nil
	})
	editResponse(s, i, msg)
}

func (b *Bot) onJoin(s *discordgo.Session, i *discordgo.InteractionCreate) {
	nick := options(i)["nickname"].StringValue()
	u := interactionUser(i)

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
		respond(s, i, "Error: "+err.Error())
		return
	}

	if found {
		respond(s, i, "You have joined the game as "+nick)
	} else {
		respond(s, i, "You are not a player in this game")
	}
}

func (b *Bot) onGenerate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.isAdmin(i) {
		respond(s, i, "You are not authorized to use this command")
		return
	}

	if err := b.store.Update(i.GuildID, func(g *Game) error {
		if len(g.Players) == 0 {
			return fmt.Errorf("no players in the game")
		}
		if len(g.Pools) == 0 {
			return fmt.Errorf("no pools in the game")
		}
		return dealAll(g.Players, g.Pools, rand.New(rand.NewSource(time.Now().UnixNano())))
	}); err != nil {
		respond(s, i, "Error: "+err.Error())
		return
	}
	respond(s, i, "Character cards generated successfully. Use /preview to preview the cards.")
}

func (b *Bot) onPreview(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.isAdmin(i) {
		respond(s, i, "You are not authorized to use this command")
		return
	}

	deferResponse(s, i)

	var text string

	b.store.Read(i.GuildID, func(g *Game) error {
		text = formatBoard(g.Players)
		return nil
	})
	editResponse(s, i, text)
}

func (b *Bot) onRegenerate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.isAdmin(i) {
		respond(s, i, "You are not authorized to use this command")
		return
	}

	if err := b.store.Update(i.GuildID, func(g *Game) error {
		if len(g.Players) == 0 {
			return fmt.Errorf("no players in the game")
		}
		if len(g.Pools) == 0 {
			return fmt.Errorf("no pools in the game")
		}
		return rerollGroup(g.Players, "all", g.Pools, rand.New(rand.NewSource(time.Now().UnixNano())))
	}); err != nil {
	}

}
