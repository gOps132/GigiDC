package config

import "testing"

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing database URL error")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_HTTP_ADDR", "")
	t.Setenv("GIGI_ENV", "")
	t.Setenv("GIGI_DISCORD_ENABLED", "")
	t.Setenv("GIGI_DISCORD_SYNC_COMMANDS", "")

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
}

func TestLoadMigrationDir(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
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
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_DISCORD_ENABLED", "true")
	t.Setenv("DISCORD_TOKEN", "")
	t.Setenv("DISCORD_CLIENT_ID", "client-id")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing Discord token error")
	}
}

func TestLoadRequiresDiscordClientIDWhenEnabled(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
	t.Setenv("GIGI_DISCORD_ENABLED", "true")
	t.Setenv("DISCORD_TOKEN", "token")
	t.Setenv("DISCORD_CLIENT_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing Discord client ID error")
	}
}

func TestLoadEnablesDiscordWithCredentials(t *testing.T) {
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
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
	t.Setenv("GIGI_DATABASE_URL", "postgres://gigi:gigi@localhost:5432/gigi?sslmode=disable")
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
