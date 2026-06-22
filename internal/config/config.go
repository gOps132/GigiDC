package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env                       string
	HTTPAddr                  string
	DatabaseURL               string
	MigrationsDir             string
	DiscordEnabled            bool
	DiscordSyncCommands       bool
	DiscordGuildID            string
	DiscordToken              string
	DiscordClientID           string
	OpenAIAPIKey              string
	LLMSecretKeyBase64        string
	LLMSecretKeyID            string
	WebSearchProvider         string
	WebSearchFallbackProvider string
	BraveSearchAPIKey         string
}

func Load() (Config, error) {
	braveSearchAPIKey := strings.TrimSpace(os.Getenv("BRAVE_SEARCH_API_KEY"))
	webSearchProvider := strings.ToLower(strings.TrimSpace(os.Getenv("GIGI_WEB_SEARCH_PROVIDER")))
	if webSearchProvider == "" {
		if braveSearchAPIKey != "" {
			webSearchProvider = "brave"
		} else {
			webSearchProvider = "duckduckgo"
		}
	}

	cfg := Config{
		Env:                       envOrDefault("GIGI_ENV", "development"),
		HTTPAddr:                  envOrDefault("GIGI_HTTP_ADDR", ":8080"),
		DatabaseURL:               strings.TrimSpace(os.Getenv("GIGI_DATABASE_URL")),
		MigrationsDir:             envOrDefault("GIGI_MIGRATIONS_DIR", "db/migrations"),
		DiscordEnabled:            boolEnv("GIGI_DISCORD_ENABLED"),
		DiscordSyncCommands:       boolEnv("GIGI_DISCORD_SYNC_COMMANDS"),
		DiscordGuildID:            strings.TrimSpace(os.Getenv("GIGI_DISCORD_GUILD_ID")),
		DiscordToken:              strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DiscordClientID:           strings.TrimSpace(os.Getenv("DISCORD_CLIENT_ID")),
		OpenAIAPIKey:              strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		LLMSecretKeyBase64:        strings.TrimSpace(os.Getenv("GIGI_LLM_SECRET_KEY_BASE64")),
		LLMSecretKeyID:            envOrDefault("GIGI_LLM_SECRET_KEY_ID", "local-v1"),
		WebSearchProvider:         webSearchProvider,
		WebSearchFallbackProvider: strings.ToLower(strings.TrimSpace(os.Getenv("GIGI_WEB_SEARCH_FALLBACK"))),
		BraveSearchAPIKey:         braveSearchAPIKey,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("GIGI_DATABASE_URL is required")
	}
	if cfg.WebSearchProvider != "duckduckgo" && cfg.WebSearchProvider != "brave" {
		return Config{}, fmt.Errorf("GIGI_WEB_SEARCH_PROVIDER must be duckduckgo or brave, got %q", cfg.WebSearchProvider)
	}
	if cfg.WebSearchFallbackProvider != "" && cfg.WebSearchFallbackProvider != "duckduckgo" {
		return Config{}, fmt.Errorf("GIGI_WEB_SEARCH_FALLBACK must be blank or duckduckgo, got %q", cfg.WebSearchFallbackProvider)
	}
	if cfg.WebSearchProvider == "brave" && cfg.BraveSearchAPIKey == "" && cfg.WebSearchFallbackProvider != "duckduckgo" {
		return Config{}, errors.New("BRAVE_SEARCH_API_KEY is required when GIGI_WEB_SEARCH_PROVIDER is brave without duckduckgo fallback")
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

func (cfg Config) DecodedLLMSecretKey() ([]byte, error) {
	value := strings.TrimSpace(cfg.LLMSecretKeyBase64)
	if value == "" {
		return nil, nil
	}

	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("GIGI_LLM_SECRET_KEY_BASE64 must be standard base64: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("GIGI_LLM_SECRET_KEY_BASE64 must decode to exactly 32 bytes, got %d", len(key))
	}

	return key, nil
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
