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
