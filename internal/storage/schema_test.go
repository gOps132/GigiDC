package storage

import (
	"os"
	"strings"
	"testing"
)

func TestInitialSchemaIncludesPermissionAndAuditTables(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000001_initial_schema.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"create table if not exists guilds",
		"create table if not exists role_capability_grants",
		"create table if not exists user_capability_grants",
		"create table if not exists audit_logs",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
	if strings.Contains(sql, "role_name") {
		t.Fatal("schema must map capabilities by role ID, not role name")
	}
}

func TestPluginCatalogMigrationAddsDiscordIdentityLookup(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000002_plugin_catalog.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"discord_application_id",
		"discord_bot_user_id",
		"manifest_url",
		"source_kind",
		"plugins_discord_application_id_idx",
		"plugins_discord_bot_user_id_idx",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("plugin catalog migration missing %q", want)
		}
	}
}

func TestLLMProviderCredentialsMigrationExists(t *testing.T) {
	if _, err := os.Stat("../../db/migrations/000003_llm_provider_credentials.sql"); err != nil {
		t.Fatalf("llm provider credentials migration must exist: %v", err)
	}
}

func TestLLMProviderCredentialsMigrationDefinesEncryptedMultiOwnerSchema(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000003_llm_provider_credentials.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"create table if not exists llm_credentials",
		"create table if not exists llm_model_profiles",
		"create table if not exists llm_usage_events",
		"owner_type text not null",
		"check (owner_type in ('guild', 'user', 'tenant'))",
		"purpose text not null",
		"check (purpose in ('chat', 'reasoning', 'embedding', 'routing'))",
		"credential_ciphertext bytea not null",
		"credential_nonce bytea not null",
		"credential_fingerprint text not null",
		"unique (id, provider_id)",
		"foreign key (credential_id, provider_id) references llm_credentials(id, provider_id)",
		"billing_owner_type text not null",
		"billing_owner_id text not null",
		"actor_user_id text not null",
		"where revoked_at is null",
		"where enabled",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("llm provider credentials migration missing %q", want)
		}
	}

	for _, forbidden := range []string{
		"api_key",
		"provider_secret",
		"plaintext",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("llm provider credentials migration must not include plaintext credential column %q", forbidden)
		}
	}
}
