package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/plugins"
)

func TestExternalAppDryRunHandlerPlansEnabledManifest(t *testing.T) {
	registry := &fakeExternalAppRegistry{manifests: []plugins.Manifest{dryRunManifest()}}
	checker := &fakeCapabilityChecker{
		decision: capability.Decision{Allowed: true, Capability: "plugin.install", Reason: capability.ReasonRoleGrant},
	}
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(registry, checker, recorder, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface:          MessageSurfaceGuildMention,
		GuildID:          "guild-id",
		ChannelID:        "channel-id",
		UserID:           "user-id",
		RoleIDs:          []string{"role-id"},
		HasAdministrator: false,
		Text:             "play never gonna give you up",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if !strings.Contains(response.Content, "Matched external app: `Jockie Music`.") ||
		!strings.Contains(response.Content, "Planned command: `!play never gonna give you up`.") ||
		!strings.Contains(response.Content, "Dispatch is not live yet.") {
		t.Fatalf("response = %q, want dry-run plan", response.Content)
	}
	if registry.guildID != "guild-id" {
		t.Fatalf("registry guild ID = %q, want guild-id", registry.guildID)
	}
	if checker.subject.GuildID != "guild-id" || checker.subject.UserID != "user-id" || len(checker.subject.RoleIDs) != 1 {
		t.Fatalf("checker subject = %+v, want guild role subject", checker.subject)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.dry_run" || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want succeeded dry-run event", recorder.events)
	}
	if recorder.events[0].Metadata["plugin_id"] != "jockie-music" || recorder.events[0].Metadata["trigger"] != "!play" {
		t.Fatalf("audit metadata = %+v, want plugin and trigger only", recorder.events[0].Metadata)
	}
	if _, ok := recorder.events[0].Metadata["command"]; ok {
		t.Fatalf("audit metadata = %+v, must not store raw planned command", recorder.events[0].Metadata)
	}
}

func TestExternalAppDryRunHandlerDeniesMissingCapability(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{dryRunManifest()}},
		&fakeCapabilityChecker{decision: capability.Decision{Allowed: false, Capability: "plugin.install", Reason: capability.ReasonMissingCapability}},
		recorder,
		CoreMessageHandler(),
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "play never gonna give you up",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "Permission denied for external app action." {
		t.Fatalf("response = %q, want permission denial", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusDenied || recorder.events[0].Reason != string(capability.ReasonMissingCapability) {
		t.Fatalf("audit events = %+v, want denied event", recorder.events)
	}
}

func TestExternalAppDryRunHandlerFallsBackWhenNoManifestMatches(t *testing.T) {
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{dryRunManifest()}},
		&fakeCapabilityChecker{},
		nil,
		MessageHandlerFunc(func(context.Context, Message) (MessageResponse, error) {
			return MessageResponse{Content: "fallback-ok"}, nil
		}),
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "hello",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "fallback-ok" {
		t.Fatalf("response = %q, want fallback", response.Content)
	}
}

type fakeExternalAppRegistry struct {
	guildID   string
	manifests []plugins.Manifest
	err       error
}

func (r *fakeExternalAppRegistry) EnabledForGuild(_ context.Context, guildID string) ([]plugins.Manifest, error) {
	r.guildID = guildID
	return r.manifests, r.err
}

func dryRunManifest() plugins.Manifest {
	return plugins.Manifest{
		ID:                   "jockie-music",
		Name:                 "Jockie Music",
		Version:              "1.0.0",
		Source:               "uploaded",
		SourceKind:           plugins.SourceKindUploadedFile,
		DiscordApplicationID: "1511678703963209813",
		Capabilities: []plugins.Capability{{
			Name:        "plugin.install",
			Description: "Manage external app integration.",
		}},
		Triggers:     []plugins.Trigger{{Kind: "prefix", Value: "!play"}},
		Surfaces:     []string{"guild_text"},
		Permissions:  []string{"plugin.install"},
		AuditEvents:  []string{"plugin.jockie_music.plan"},
		Attribution:  []plugins.Resource{{Name: "Jockie Music", Use: "Music bot command reference.", Source: "https://example.com"}},
		ConfigSchema: "",
	}
}
