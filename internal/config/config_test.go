package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing database URL error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_HTTP_ADDR", "")
	t.Setenv("GIGI_ENV", "")
	t.Setenv("GIGI_DISCORD_ENABLED", "")
	t.Setenv("GIGI_DISCORD_SYNC_COMMANDS", "")
	t.Setenv("GIGI_LLM_SECRET_KEY_BASE64", "")
	t.Setenv("GIGI_LLM_SECRET_KEY_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.Env != "development" {
		t.Fatalf("Env = %q, want development", cfg.Env)
	}
	if cfg.MigrationsDir != "db/migrations" {
		t.Fatalf("MigrationsDir = %q, want db/migrations", cfg.MigrationsDir)
	}
	if cfg.DiscordEnabled {
		t.Fatal("DiscordEnabled = true, want false")
	}
	if cfg.DiscordSyncCommands {
		t.Fatal("DiscordSyncCommands = true, want false")
	}
	if cfg.LLMSecretKeyBase64 != "" {
		t.Fatalf("LLMSecretKeyBase64 = %q, want blank", cfg.LLMSecretKeyBase64)
	}
	if cfg.LLMSecretKeyID != "local-v1" {
		t.Fatalf("LLMSecretKeyID = %q, want local-v1", cfg.LLMSecretKeyID)
	}
	if cfg.WebSearchProvider != "duckduckgo" {
		t.Fatalf("WebSearchProvider = %q, want duckduckgo", cfg.WebSearchProvider)
	}
	if cfg.WebSearchFallbackProvider != "" {
		t.Fatalf("WebSearchFallbackProvider = %q, want blank", cfg.WebSearchFallbackProvider)
	}
}

func TestLoadMigrationDir(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_MIGRATIONS_DIR", "/app/db/migrations")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MigrationsDir != "/app/db/migrations" {
		t.Fatalf("MigrationsDir = %q, want /app/db/migrations", cfg.MigrationsDir)
	}
}

func TestLoadRequiresDiscordTokenWhenEnabled(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_DISCORD_ENABLED", "true")
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("DISCORD_CLIENT_ID", "client-id")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing Discord token error")
	}
}

func TestLoadRequiresDiscordClientIDWhenEnabled(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_DISCORD_ENABLED", "true")
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("DISCORD_CLIENT_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing Discord client ID error")
	}
}

func TestLoadEnablesDiscordWithCredentials(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_DISCORD_ENABLED", "yes")
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("DISCORD_CLIENT_ID", "client-id")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.DiscordEnabled {
		t.Fatal("DiscordEnabled = false, want true")
	}
}

func TestLoadDiscordCommandSyncSettings(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_DISCORD_ENABLED", "true")
	t.Setenv("GIGI_DISCORD_SYNC_COMMANDS", "on")
	t.Setenv("GIGI_DISCORD_GUILD_ID", "guild-id")
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("DISCORD_CLIENT_ID", "client-id")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !cfg.DiscordSyncCommands {
		t.Fatal("DiscordSyncCommands = false, want true")
	}
	if cfg.DiscordGuildID != "guild-id" {
		t.Fatalf("DiscordGuildID = %q, want guild-id", cfg.DiscordGuildID)
	}
}

func TestLoadLLMSecretKeySettings(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_LLM_SECRET_KEY_BASE64", "  c2VjcmV0  ")
	t.Setenv("GIGI_LLM_SECRET_KEY_ID", "  prod-v2  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMSecretKeyBase64 != "c2VjcmV0" {
		t.Fatalf("LLMSecretKeyBase64 = %q, want c2VjcmV0", cfg.LLMSecretKeyBase64)
	}
	if cfg.LLMSecretKeyID != "prod-v2" {
		t.Fatalf("LLMSecretKeyID = %q, want prod-v2", cfg.LLMSecretKeyID)
	}
}

func TestLoadWebSearchSettings(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "  brave  ")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "  searxng  ")
	t.Setenv("BRAVE_SEARCH_API_KEY", "  search-key  ")
	t.Setenv("SEARXNG_BASE_URL", "  https://searx.test  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebSearchProvider != "brave" {
		t.Fatalf("WebSearchProvider = %q, want brave", cfg.WebSearchProvider)
	}
	if cfg.WebSearchFallbackProvider != "searxng" {
		t.Fatalf("WebSearchFallbackProvider = %q, want searxng", cfg.WebSearchFallbackProvider)
	}
	if cfg.BraveSearchAPIKey != "search-key" {
		t.Fatalf("BraveSearchAPIKey = %q, want search-key", cfg.BraveSearchAPIKey)
	}
	if cfg.SearXNGBaseURL != "https://searx.test" {
		t.Fatalf("SearXNGBaseURL = %q, want https://searx.test", cfg.SearXNGBaseURL)
	}
}

func TestLoadDefaultsWebSearchProviderToBraveWhenKeyExists(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "search-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebSearchProvider != "brave" {
		t.Fatalf("WebSearchProvider = %q, want brave", cfg.WebSearchProvider)
	}
}

func TestLoadDefaultsWebSearchProviderToBraveWhenBothBraveAndSearXNGConfigured(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "search-key")
	t.Setenv("SEARXNG_BASE_URL", "https://searx.test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebSearchProvider != "brave" {
		t.Fatalf("WebSearchProvider = %q, want brave", cfg.WebSearchProvider)
	}
}

func TestLoadRejectsUnknownWebSearchProvider(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "brvae")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GIGI_WEB_SEARCH_PROVIDER") {
		t.Fatalf("expected provider validation error, got %v", err)
	}
}

func TestLoadRejectsUnknownWebSearchFallback(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "brave")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GIGI_WEB_SEARCH_FALLBACK") {
		t.Fatalf("expected fallback validation error, got %v", err)
	}
}

func TestLoadDefaultsWebSearchProviderToSearXNGWhenBaseURLExists(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")
	t.Setenv("SEARXNG_BASE_URL", "https://searx.test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.WebSearchProvider != "searxng" {
		t.Fatalf("WebSearchProvider = %q, want searxng", cfg.WebSearchProvider)
	}
}

func TestLoadRequiresBraveSearchKeyWithoutFallback(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "brave")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "BRAVE_SEARCH_API_KEY") {
		t.Fatalf("expected Brave API key validation error, got %v", err)
	}
}

func TestLoadAllowsBraveSearchWithoutKeyWhenDuckDuckGoFallbackExists(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "brave")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "duckduckgo")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")

	if _, err := Load(); err != nil {
		t.Fatalf("Load returned error with DuckDuckGo fallback: %v", err)
	}
}

func TestLoadAllowsBraveSearchWithoutKeyWhenSearXNGFallbackExists(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "brave")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "searxng")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")
	t.Setenv("SEARXNG_BASE_URL", "https://searx.test")

	if _, err := Load(); err != nil {
		t.Fatalf("Load returned error with SearXNG fallback: %v", err)
	}
}

func TestLoadRequiresSearXNGBaseURLWithoutFallback(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "searxng")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "")
	t.Setenv("SEARXNG_BASE_URL", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SEARXNG_BASE_URL") {
		t.Fatalf("expected SearXNG base URL validation error, got %v", err)
	}
}

func TestLoadRequiresSearXNGBaseURLForFallback(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_WEB_SEARCH_PROVIDER", "brave")
	t.Setenv("BRAVE_SEARCH_API_KEY", "")
	t.Setenv("GIGI_WEB_SEARCH_FALLBACK", "searxng")
	t.Setenv("SEARXNG_BASE_URL", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SEARXNG_BASE_URL") {
		t.Fatalf("expected SearXNG fallback validation error, got %v", err)
	}
}

func TestDecodedLLMSecretKeyReturnsNilWhenBlank(t *testing.T) {
	cfg := Config{}

	key, err := cfg.DecodedLLMSecretKey()
	if err != nil {
		t.Fatalf("DecodedLLMSecretKey returned error: %v", err)
	}
	if key != nil {
		t.Fatalf("DecodedLLMSecretKey = %v, want nil", key)
	}
}

func TestDecodedLLMSecretKeyDecodesValid32ByteKey(t *testing.T) {
	want := []byte(strings.Repeat("a", 32))
	cfg := Config{LLMSecretKeyBase64: base64.StdEncoding.EncodeToString(want)}

	got, err := cfg.DecodedLLMSecretKey()
	if err != nil {
		t.Fatalf("DecodedLLMSecretKey returned error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("DecodedLLMSecretKey = %q, want %q", got, want)
	}
}

func TestDecodedLLMSecretKeyErrorsOnInvalidBase64(t *testing.T) {
	cfg := Config{LLMSecretKeyBase64: "not base64"}

	_, err := cfg.DecodedLLMSecretKey()
	if err == nil {
		t.Fatal("expected invalid base64 error")
	}
	if !strings.Contains(err.Error(), "GIGI_LLM_SECRET_KEY_BASE64 must be standard base64") {
		t.Fatalf("error = %q, want helpful base64 error", err.Error())
	}
}

func TestDecodedLLMSecretKeyErrorsOnWrongLength(t *testing.T) {
	cfg := Config{LLMSecretKeyBase64: base64.StdEncoding.EncodeToString([]byte("too-short"))}

	_, err := cfg.DecodedLLMSecretKey()
	if err == nil {
		t.Fatal("expected wrong length error")
	}
	if !strings.Contains(err.Error(), "must decode to exactly 32 bytes") {
		t.Fatalf("error = %q, want helpful length error", err.Error())
	}
}
