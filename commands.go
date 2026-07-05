package main

import (
	"github.com/bwmarrin/discordgo"
)

var commands = []*discordgo.ApplicationCommand{
	{Name: "ping", Description: "Ping the bot"},
	{Name: "whoami", Description: "Show your Discord user information"},
	{Name: "upload", Description: "Upload a file with users to the server",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "file",
				Description: "The file to upload (JSON-config)",
				Required:    true,
			},
		},
	},
	{Name: "players", Description: "Show the players on the server (admin only)"},
	{Name: "join", Description: "Join the player to the nickname",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "nickname",
				Description: "Your player nickname",
				Required:    true,
			},
		},
	},
	{Name: "generate", Description: "Generate character card for all the players (without sending, for admin only)"},
	{Name: "preview", Description: "Preview character card for all the players (without sending, for admin only)"},
	{Name: "regenerate", Description: "Regenerate character card for all the players (without sending, for admin only)"},
	{Name: "setstat", Description: "Set player`s stats (admin only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "nickname",
				Description: "Player`s nickname",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "stat",
				Description: "Player`s stat",
				Required:    true,
				Choices:     statChoices(),
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "value",
				Description: "Player`s stat value",
				Required:    true,
			},
		},
	},
	{Name: "send", Description: "Send character card to the player`s dm (admin only)"},
	{Name: "reroll", Description: "Reroll player`s stats and send to the dm (admin only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "target",
				Description: "Target stat to reroll",
				Required:    true,
				Choices:     statChoices(),
			},
		},
	},
	{Name: "start", Description: "Start a new game (admin only)"},
	{Name: "reset", Description: "Reset the game (admin only)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "confirm",
				Description: "Confirm the action",
				Required:    true,
			},
		},
	},

	{Name: "mystats", Description: "Show my stats (player only)"},
}

func targetChoices() []*discordgo.ApplicationCommandOptionChoice {
	return []*discordgo.ApplicationCommandOptionChoice{
		{Name: "Биология (пол/возраст/фертильность)", Value: "biology"},
		{Name: "Здоровье", Value: "health"},
		{Name: "Профессия", Value: "profession"},
		{Name: "Хобби", Value: "hobby"},
		{Name: "Багаж", Value: "baggage"},
		{Name: "Факты ", Value: "facts"},
	}
}

func statChoices() []*discordgo.ApplicationCommandOptionChoice {
	return []*discordgo.ApplicationCommandOptionChoice{
		{Name: "Пол", Value: "gender"},
		{Name: "Возраст", Value: "age"},
		{Name: "fertility", Value: "fertility"},
		{Name: "Здоровье", Value: "health"},
		{Name: "Профессия", Value: "profession"},
		{Name: "Хобби", Value: "hobby"},
		{Name: "Багаж", Value: "baggage"},
		{Name: "Факт 1", Value: "fact1"},
		{Name: "Факт 2", Value: "fact2"},
	}
}

func registerCommands(s *discordgo.Session, appID string, guildID string) error {
	for _, command := range commands {
		if _, err := s.ApplicationCommandCreate(appID, guildID, command); err != nil {
			return err
		}
	}
	return nil
}
