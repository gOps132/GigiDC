package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/assistant"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/llm"
	"github.com/gOps132/GigiDC/internal/llm/provider"
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
		!strings.Contains(response.Content, "Dry-run only; no command sent.") {
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

func TestExternalAppDryRunHandlerAllowsPublicManifestWithoutChecker(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Permissions = nil
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		nil,
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
	if !strings.Contains(response.Content, "Matched external app: `Jockie Music`.") {
		t.Fatalf("response = %q, want public dry-run plan", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want succeeded public dry-run", recorder.events)
	}
	if _, ok := recorder.events[0].Metadata["capability"]; ok {
		t.Fatalf("audit metadata = %+v, want no capability for public action", recorder.events[0].Metadata)
	}
}

func TestExternalAppDryRunHandlerDispatchesPublicSendMessageManifest(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Permissions = nil
	manifest.Dispatch = plugins.DispatchModeSendMessage
	manifest.PublicDispatchAllowed = true
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		nil,
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
	if response.Content != "!play never gonna give you up" {
		t.Fatalf("response = %q, want dispatched command", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.dispatch" || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want succeeded dispatch", recorder.events)
	}
	if _, ok := recorder.events[0].Metadata["command"]; ok {
		t.Fatalf("audit metadata = %+v, must not store raw dispatched command", recorder.events[0].Metadata)
	}
}

func TestExternalAppDryRunHandlerDispatchesActionPublicSendMessage(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Permissions = []string{"plugin.install"}
	manifest.Dispatch = plugins.DispatchModeDryRun
	manifest.PublicDispatchAllowed = true
	manifest.Triggers = nil
	manifest.Actions = []plugins.Action{{
		ID:       "play",
		Trigger:  plugins.Trigger{Kind: "prefix", Value: "!play", Aliases: []string{"play"}},
		Surfaces: []string{"guild_text"},
		Safety:   plugins.SafetyClassPublic,
		Dispatch: plugins.DispatchModeSendMessage,
		Adapter:  plugins.DispatchAdapterPrefixCommand,
	}}
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		nil,
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
	if response.Content != "!play never gonna give you up" {
		t.Fatalf("response = %q, want action dispatched command", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.dispatch" || recorder.events[0].Metadata["action_id"] != "play" {
		t.Fatalf("audit events = %+v, want action dispatch audit", recorder.events)
	}
}

func TestExternalAppDryRunHandlerDoesNotDispatchPublicManifestWithoutImportApproval(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Permissions = nil
	manifest.Dispatch = plugins.DispatchModeSendMessage
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		nil,
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
	if response.Content == "!play never gonna give you up" || !strings.Contains(response.Content, "Dry-run only; no command sent.") {
		t.Fatalf("response = %q, want dry-run without import approval", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.dry_run" {
		t.Fatalf("audit events = %+v, want dry-run audit", recorder.events)
	}
}

func TestExternalAppDryRunHandlerDoesNotDispatchRestrictedManifest(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Dispatch = plugins.DispatchModeSendMessage
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		&fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "plugin.install", Reason: capability.ReasonRoleGrant}},
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
	if !strings.Contains(response.Content, "Dry-run only; no command sent.") {
		t.Fatalf("response = %q, want dry-run for restricted manifest", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.dry_run" {
		t.Fatalf("audit events = %+v, want dry-run audit", recorder.events)
	}
}

func TestExternalAppDryRunHandlerDoesNotDispatchRestrictedAction(t *testing.T) {
	manifest := dryRunManifest()
	manifest.PublicDispatchAllowed = true
	manifest.Triggers = nil
	manifest.Actions = []plugins.Action{{
		ID:          "skip",
		Trigger:     plugins.Trigger{Kind: "prefix", Value: "!skip"},
		Surfaces:    []string{"guild_text"},
		Permissions: []string{"plugin.install"},
		Safety:      plugins.SafetyClassRestricted,
		Dispatch:    plugins.DispatchModeSendMessage,
		Adapter:     plugins.DispatchAdapterPrefixCommand,
	}}
	handler := ExternalAppDryRunHandler(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		&fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "plugin.install", Reason: capability.ReasonRoleGrant}},
		nil,
		CoreMessageHandler(),
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "!skip",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content == "!skip" || !strings.Contains(response.Content, "Dry-run only; no command sent.") {
		t.Fatalf("response = %q, want restricted action dry-run", response.Content)
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

func TestExternalAppDryRunHandlerUsesSemanticDryRunWhenNoPrefixMatches(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandlerWithSemantic(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{dryRunManifest()}},
		&fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "plugin.install", Reason: capability.ReasonRoleGrant}},
		recorder,
		CoreMessageHandler(),
		assistant.SemanticPluginPlanner{Runtime: &fakeSemanticRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!play","arguments":"never gonna give you up"}`}}},
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "please play never gonna give you up",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if !strings.Contains(response.Content, "Matched external app: `Jockie Music`.") ||
		!strings.Contains(response.Content, "Planned command: `!play never gonna give you up`.") ||
		!strings.Contains(response.Content, "Dry-run only; no command sent.") {
		t.Fatalf("response = %q, want semantic dry-run plan", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.semantic_dry_run" || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want semantic dry-run audit", recorder.events)
	}
	if _, ok := recorder.events[0].Metadata["command"]; ok {
		t.Fatalf("audit metadata = %+v, must not store raw planned command", recorder.events[0].Metadata)
	}
}

func TestExternalAppSemanticRoutingSkipsWhenPolicyOff(t *testing.T) {
	handler := ExternalAppDryRunHandlerWithSemanticPolicy(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{dryRunManifest()}},
		&fakeCapabilityChecker{},
		nil,
		MessageHandlerFunc(func(context.Context, Message) (MessageResponse, error) {
			return MessageResponse{Content: "fallback-ok"}, nil
		}),
		assistant.SemanticPluginPlanner{Runtime: &fakeSemanticRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!play","arguments":"song"}`}}},
		&fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingOff}},
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "could you play song",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "fallback-ok" {
		t.Fatalf("response = %q, want fallback when semantic routing off", response.Content)
	}
}

func TestExternalAppSemanticRoutingEnabledDispatchesPublicManifest(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Permissions = nil
	manifest.Dispatch = plugins.DispatchModeSendMessage
	manifest.PublicDispatchAllowed = true
	recorder := &fakeAuditRecorder{}
	handler := ExternalAppDryRunHandlerWithSemanticPolicy(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		nil,
		recorder,
		CoreMessageHandler(),
		assistant.SemanticPluginPlanner{Runtime: &fakeSemanticRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!play","arguments":"song"}`}}},
		&fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}},
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "could you play song",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "!play song" {
		t.Fatalf("response = %q, want semantic public dispatch", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.external_app.semantic_dispatch" || recorder.events[0].Status != audit.StatusSucceeded {
		t.Fatalf("audit events = %+v, want semantic dispatch audit", recorder.events)
	}
}

func TestExternalAppSemanticDryRunNeverDispatchesPublicManifest(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Permissions = nil
	manifest.Dispatch = plugins.DispatchModeSendMessage
	handler := ExternalAppDryRunHandlerWithSemantic(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		nil,
		nil,
		CoreMessageHandler(),
		assistant.SemanticPluginPlanner{Runtime: &fakeSemanticRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!play","arguments":"song"}`}}},
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "could you play song",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content == "!play song" || !strings.Contains(response.Content, "Dry-run only; no command sent.") {
		t.Fatalf("response = %q, want dry-run not dispatch", response.Content)
	}
}

func TestExternalAppSemanticRoutingEnabledStillDryRunsRestrictedManifest(t *testing.T) {
	manifest := dryRunManifest()
	manifest.Dispatch = plugins.DispatchModeSendMessage
	handler := ExternalAppDryRunHandlerWithSemanticPolicy(
		&fakeExternalAppRegistry{manifests: []plugins.Manifest{manifest}},
		&fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "plugin.install", Reason: capability.ReasonRoleGrant}},
		nil,
		CoreMessageHandler(),
		assistant.SemanticPluginPlanner{Runtime: &fakeSemanticRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!play","arguments":"song"}`}}},
		&fakeLLMPolicyManager{policy: provider.GuildPolicy{GuildID: "guild-id", ToolRoutingMode: provider.ToolRoutingEnabled}},
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "user-id",
		Text:    "could you play song",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content == "!play song" || !strings.Contains(response.Content, "Dry-run only; no command sent.") {
		t.Fatalf("response = %q, want dry-run for restricted semantic plan", response.Content)
	}
}

type fakeExternalAppRegistry struct {
	guildID   string
	manifests []plugins.Manifest
	err       error
}

type fakeSemanticRuntime struct {
	req      llm.GenerateTextRequest
	response llm.TextResponse
	err      error
}

func (r *fakeSemanticRuntime) GenerateText(_ context.Context, req llm.GenerateTextRequest) (llm.TextResponse, error) {
	r.req = req
	return r.response, r.err
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
