package audit

import (
	"strings"
	"testing"
)

func TestEventRejectsMissingRequiredFields(t *testing.T) {
	event := Event{Kind: "discord.permission.check", Status: StatusAllowed}

	if err := event.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEventAcceptsPermissionDecision(t *testing.T) {
	event := Event{
		Kind:      "discord.permission.check",
		GuildID:   "guild-id",
		ActorID:   "user-id",
		Status:    StatusDenied,
		Reason:    "missing_capability",
		Metadata:  map[string]string{"capability": "plugin.install"},
		RequestID: "request-id",
	}

	if err := event.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestSanitizeMetadataKeepsSafeFieldsAndRemovesSecrets(t *testing.T) {
	input := map[string]string{
		"capability":    "job.admin",
		"source":        "discord",
		"run_id":        "run-123",
		"api_key":       "raw-api-key",
		"authorization": "Bearer raw-token",
		"fingerprint":   "Bearer raw-token",
		"model":         "sk-live-secret",
	}

	got := SanitizeMetadata(input)

	for key, want := range map[string]string{
		"capability": "job.admin",
		"source":     "discord",
		"run_id":     "run-123",
	} {
		if got[key] != want {
			t.Fatalf("metadata[%q] = %q, want %q", key, got[key], want)
		}
	}
	for _, key := range []string{"api_key", "authorization"} {
		if _, ok := got[key]; ok {
			t.Fatalf("metadata contains sensitive key %q: %+v", key, got)
		}
	}
	for key, value := range got {
		if strings.Contains(strings.ToLower(value), "bearer") || strings.Contains(value, "sk-live-secret") {
			t.Fatalf("metadata[%q] contains sensitive value %q", key, value)
		}
	}
	if _, ok := input["api_key"]; !ok {
		t.Fatal("SanitizeMetadata mutated input")
	}
}

func TestEventRejectsSecretLikeMetadataKeys(t *testing.T) {
	for _, key := range []string{"api_key", "secret", "token", "authorization", "provider_secret", "client_secret", "private_key", "refresh_token"} {
		event := Event{
			Kind:     "llm.provider.update",
			GuildID:  "guild-id",
			ActorID:  "user-id",
			Status:   StatusAllowed,
			Metadata: map[string]string{key: "raw-value"},
		}

		err := event.Validate()
		if err == nil {
			t.Fatalf("expected validation error for key %q", key)
		}
		if !strings.Contains(err.Error(), "sensitive metadata") {
			t.Fatalf("error = %v, want sensitive metadata", err)
		}
	}
}

func TestEventRejectsSecretLikeMetadataValues(t *testing.T) {
	event := Event{
		Kind:     "llm.provider.update",
		GuildID:  "guild-id",
		ActorID:  "user-id",
		Status:   StatusAllowed,
		Metadata: map[string]string{"fingerprint": "Bearer raw-value"},
	}

	err := event.Validate()
	if err == nil {
		t.Fatal("expected secret-like value error")
	}
	if !strings.Contains(err.Error(), "sensitive metadata") {
		t.Fatalf("error = %v, want sensitive metadata", err)
	}
}
