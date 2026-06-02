package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	DiscordToken    string
	DiscordClientID string
	OpenAIAPIKey    string
}

func Load() (Config, error) {
	cfg := Config{
		Env:             envOrDefault("GIGI_ENV", "development"),
		HTTPAddr:        envOrDefault("GIGI_HTTP_ADDR", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("GIGI_DATABASE_URL")),
		DiscordToken:    strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DiscordClientID: strings.TrimSpace(os.Getenv("DISCORD_CLIENT_ID")),
		OpenAIAPIKey:    strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("GIGI_DATABASE_URL is required")
	}

	if !strings.HasPrefix(cfg.HTTPAddr, ":") && !strings.Contains(cfg.HTTPAddr, ":") {
		return Config{}, fmt.Errorf("GIGI_HTTP_ADDR must include a port, got %q", cfg.HTTPAddr)
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
