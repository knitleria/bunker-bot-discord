package main

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Token        string
	GuildID      string
	AdminUserIds map[string]bool
	DataDir      string
}

func loadConfig() Config {
	_ = godotenv.Load()
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN is not set")
	}
	admins := map[string]bool{}
	for _, id := range strings.Split(os.Getenv("ADMIN_USER_IDS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			admins[id] = true
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	guildID := os.Getenv("GUILD_ID")
	if guildID == "" {
		log.Fatal("GUILD_ID is not set")
	}

	return Config{
		Token:        token,
		GuildID:      guildID,
		AdminUserIds: admins,
		DataDir:      dataDir,
	}
}
