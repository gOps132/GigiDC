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
		"public_dispatch_allowed",
		"source_kind",
		"plugins_discord_application_id_idx",
		"plugins_discord_bot_user_id_idx",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("plugin catalog migration missing %q", want)
		}
	}
}

func TestPluginPublicDispatchApprovalMigrationExists(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000008_plugin_public_dispatch_approval.sql")
	if err != nil {
		t.Fatalf("public dispatch approval migration must exist: %v", err)
	}
	if !strings.Contains(string(sqlBytes), "public_dispatch_allowed boolean not null default false") {
		t.Fatalf("public dispatch approval migration missing approval column")
	}
}

func TestAgentRunsMigrationExists(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000009_agent_runs.sql")
	if err != nil {
		t.Fatalf("agent runs migration must exist: %v", err)
	}
	sql := string(sqlBytes)
	for _, want := range []string{
		"create table if not exists agent_runs",
		"create table if not exists agent_run_steps",
		"create table if not exists agent_run_confirmations",
		"termination_reason",
		"max_input_tokens",
		"steps_used",
		"cancel_requested_at",
		"cancel_requested_by_user_id",
		"canceled_at",
		"agent_run_steps_run_step_idx",
		"resolved_by_user_id",
		"confirmed_at",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("agent runs migration missing %q", want)
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
		"llm_credentials_active_owner_label_idx",
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

func TestAssistantConversationMigrationStoresMetadataOnly(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000004_assistant_conversation_turns.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"create table if not exists assistant_conversation_turns",
		"request_id text not null",
		"surface text not null",
		"actor_user_id text not null",
		"role text not null",
		"content_storage text not null default 'metadata_only'",
		"content_chars integer not null",
		"assistant_conversation_turns_guild_channel_idx",
		"assistant_conversation_turns_request_idx",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("assistant conversation migration missing %q", want)
		}
	}

	for _, forbidden := range []string{
		"prompt",
		"completion",
		"message_text",
		"content text",
		"raw_content",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("assistant conversation migration must not include raw text column %q", forbidden)
		}
	}
}

func TestLLMGuildPoliciesMigrationDefaultsPersonalKeysOff(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000005_llm_guild_policies.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"create table if not exists llm_guild_policies",
		"guild_id text primary key",
		"personal_keys_mode text not null default 'off'",
		"check (personal_keys_mode in ('off', 'dm-only', 'guild-allowed'))",
		"updated_by_user_id text",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("llm guild policies migration missing %q", want)
		}
	}
}

func TestLLMToolRoutingPolicyMigrationDefaultsOff(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000007_llm_tool_routing_policy.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"add column if not exists tool_routing_mode text not null default 'off'",
		"check (tool_routing_mode in ('off', 'dry-run', 'enabled'))",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("llm tool routing policy migration missing %q", want)
		}
	}
}

func TestGuildMemoryMigrationDefinesPolicyAndRetrievalSchema(t *testing.T) {
	sqlBytes, err := os.ReadFile("../../db/migrations/000006_guild_memory.sql")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	sql := string(sqlBytes)

	for _, want := range []string{
		"create table if not exists guild_memory_policies",
		"default_retention_days integer not null default 90",
		"raw_storage_mode text not null default 'metadata'",
		"check (raw_storage_mode in ('off', 'metadata', 'full'))",
		"create table if not exists guild_memory_channels",
		"primary key (guild_id, channel_id)",
		"check (mode in ('off', 'metadata', 'full'))",
		"create table if not exists guild_memory_messages",
		"content_ciphertext bytea",
		"content_hash text",
		"retention_until timestamptz not null",
		"create table if not exists guild_memory_segments",
		"search_text text not null",
		"create table if not exists guild_memory_embeddings",
		"embedding vector",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("guild memory migration missing %q", want)
		}
	}
}
