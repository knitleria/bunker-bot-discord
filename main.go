package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Config Config
	store  *Store
}

func main() {
	config := loadConfig()
	discord, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatal(err)
	}
	discord.Identify.Intents = discordgo.IntentsGuilds
	store, err := NewStore(config.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	bot := &Bot{Config: config, store: store}
	discord.AddHandler(bot.onInteraction)
	discord.AddHandler(bot.onReady)

	if err := discord.Open(); err != nil {
		log.Fatal(err)
	}
	defer discord.Close()
	fmt.Println("Bot is now running. Press CTRL-C to exit.")
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("Bot is shutting down...")
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	fmt.Println("Logged in as", r.User.Username)
	if err := registerCommands(s, s.State.User.ID, b.Config.GuildID); err != nil {
		log.Fatal(err)
		return
	}
	fmt.Println("Commands registered")
}
