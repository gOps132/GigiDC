package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env                 string
	HTTPAddr            string
	DatabaseURL         string
	MigrationsDir       string
	DiscordEnabled      bool
	DiscordSyncCommands bool
	DiscordGuildID      string
	DiscordToken        string
	DiscordClientID     string
	OpenAIAPIKey        string
}

func Load() (Config, error) {
	cfg := Config{
		Env:                 envOrDefault("GIGI_ENV", "development"),
		HTTPAddr:            envOrDefault("GIGI_HTTP_ADDR", ":8080"),
		DatabaseURL:         strings.TrimSpace(os.Getenv("GIGI_DATABASE_URL")),
		MigrationsDir:       envOrDefault("GIGI_MIGRATIONS_DIR", "db/migrations"),
		DiscordEnabled:      boolEnv("GIGI_DISCORD_ENABLED"),
		DiscordSyncCommands: boolEnv("GIGI_DISCORD_SYNC_COMMANDS"),
		DiscordGuildID:      strings.TrimSpace(os.Getenv("GIGI_DISCORD_GUILD_ID")),
		DiscordToken:        strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DiscordClientID:     strings.TrimSpace(os.Getenv("DISCORD_CLIENT_ID")),
		OpenAIAPIKey:        strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("GIGI_DATABASE_URL is required")
	}

	if !strings.HasPrefix(cfg.HTTPAddr, ":") && !strings.Contains(cfg.HTTPAddr, ":") {
		return Config{}, fmt.Errorf("GIGI_HTTP_ADDR must include a port, got %q", cfg.HTTPAddr)
	}

	if cfg.DiscordEnabled {
		if cfg.DiscordToken == "" {
			return Config{}, errors.New("DISCORD_TOKEN is required when GIGI_DISCORD_ENABLED is true")
		}
		if cfg.DiscordClientID == "" {
			return Config{}, errors.New("DISCORD_CLIENT_ID is required when GIGI_DISCORD_ENABLED is true")
		}
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func boolEnv(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
