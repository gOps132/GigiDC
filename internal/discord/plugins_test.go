package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/plugins"
)

func TestPluginCommandsExposeAdminSurface(t *testing.T) {
	commands := PluginCommands(&fakePluginManager{}, &fakePluginFetcher{}, nil)
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.RequiredCapability != capability.Capability("plugin.install") {
		t.Fatalf("required capability = %q, want plugin.install", command.RequiredCapability)
	}
	for _, name := range []string{"list", "import-manifest", "import-file", "enable", "disable", "enabled", "dry-run"} {
		if findOption(command.Options, name) == nil {
			t.Fatalf("plugins command missing %q", name)
		}
	}
	importManifest := findOption(command.Options, "import-manifest")
	if option := findOption(importManifest.Options, "url"); option == nil || option.Type != discordgo.ApplicationCommandOptionString {
		t.Fatalf("import url option = %+v, want string", option)
	}
	importFile := findOption(command.Options, "import-file")
	if option := findOption(importFile.Options, "attachment"); option == nil || option.Type != discordgo.ApplicationCommandOptionAttachment {
		t.Fatalf("import file option = %+v, want attachment", option)
	}
	dryRun := findOption(command.Options, "dry-run")
	if option := findOption(dryRun.Options, "text"); option == nil || option.Type != discordgo.ApplicationCommandOptionString {
		t.Fatalf("dry-run text option = %+v, want string", option)
	}
}

func TestPluginCommandListsApprovedAndEnabledManifests(t *testing.T) {
	manager := &fakePluginManager{manifests: []plugins.Manifest{pluginManifest("example-tool", "1.0.0")}}
	handler := PluginCommands(manager, &fakePluginFetcher{}, nil)[0].Handle

	listResponse, err := handler(context.Background(), pluginInteraction("list", nil))
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if manager.method != "ApprovedManifests" || !strings.Contains(listResponse.Content, "Approved plugins") || !listResponse.Ephemeral {
		t.Fatalf("list response = %+v manager = %+v, want approved plugin list", listResponse, manager)
	}

	enabledResponse, err := handler(context.Background(), pluginInteraction("enabled", nil))
	if err != nil {
		t.Fatalf("enabled returned error: %v", err)
	}
	if manager.method != "EnabledForGuild" || manager.guildID != "guild-id" || !strings.Contains(enabledResponse.Content, "Enabled plugins") || !enabledResponse.Ephemeral {
		t.Fatalf("enabled response = %+v manager = %+v, want guild enabled list", enabledResponse, manager)
	}
}

func TestPluginCommandImportsManifestAndAudits(t *testing.T) {
	manager := &fakePluginManager{}
	fetcher := &fakePluginFetcher{manifest: pluginManifest("example-tool", "1.0.0")}
	recorder := &fakeAuditRecorder{}
	handler := PluginCommands(manager, fetcher, recorder)[0].Handle

	response, err := handler(context.Background(), pluginInteraction("import-manifest", []InteractionOption{{Name: "url", Value: "https://example.test/gigi-plugin.json"}}))
	if err != nil {
		t.Fatalf("import returned error: %v", err)
	}
	if fetcher.url != "https://example.test/gigi-plugin.json" {
		t.Fatalf("fetch url = %q, want manifest URL", fetcher.url)
	}
	if manager.method != "UpsertApprovedManifest" || manager.manifest.ID != "example-tool" || manager.actorID != "actor-id" {
		t.Fatalf("manager = %+v, want approved manifest upsert", manager)
	}
	if !strings.Contains(response.Content, "Imported plugin") || !response.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral import success", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["plugin_id"] != "example-tool" {
		t.Fatalf("audit events = %+v, want successful import audit", recorder.events)
	}
}

func TestPluginCommandImportsAttachmentAndAudits(t *testing.T) {
	manager := &fakePluginManager{}
	fetcher := &fakePluginFetcher{manifest: pluginManifest("example-tool", "1.0.0")}
	recorder := &fakeAuditRecorder{}
	handler := PluginCommands(manager, fetcher, recorder)[0].Handle

	response, err := handler(context.Background(), Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "plugins",
		Attachments: map[string]InteractionAttachment{
			"attachment-id": {
				ID:          "attachment-id",
				URL:         "https://cdn.discordapp.com/attachments/gigi-plugin.json?ex=value",
				Filename:    "gigi-plugin.json",
				ContentType: "application/json",
				Size:        123,
			},
		},
		Options: []InteractionOption{{
			Name: "import-file",
			Options: []InteractionOption{
				{Name: "attachment", Type: discordgo.ApplicationCommandOptionAttachment, Value: "attachment-id"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("import-file returned error: %v", err)
	}
	if fetcher.attachment.ID != "attachment-id" || fetcher.attachment.Filename != "gigi-plugin.json" {
		t.Fatalf("attachment = %+v, want resolved attachment", fetcher.attachment)
	}
	if manager.method != "UpsertApprovedManifest" || manager.manifest.ID != "example-tool" || manager.actorID != "actor-id" {
		t.Fatalf("manager = %+v, want approved manifest upsert", manager)
	}
	if !strings.Contains(response.Content, "uploaded file") || !response.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral upload import success", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["action"] != "import-file" {
		t.Fatalf("audit events = %+v, want successful import-file audit", recorder.events)
	}
}

func TestPluginCommandEnableDisableAndAudit(t *testing.T) {
	manager := &fakePluginManager{}
	recorder := &fakeAuditRecorder{}
	handler := PluginCommands(manager, &fakePluginFetcher{}, recorder)[0].Handle

	enableResponse, err := handler(context.Background(), pluginInteraction("enable", []InteractionOption{
		{Name: "plugin", Value: "example-tool"},
		{Name: "version", Value: "1.0.0"},
	}))
	if err != nil {
		t.Fatalf("enable returned error: %v", err)
	}
	if manager.method != "EnableForGuild" || manager.guildID != "guild-id" || manager.pluginID != "example-tool" || manager.version != "1.0.0" {
		t.Fatalf("manager = %+v, want enable call", manager)
	}
	if !strings.Contains(enableResponse.Content, "Enabled plugin") || !enableResponse.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral enable success", enableResponse)
	}

	disableResponse, err := handler(context.Background(), pluginInteraction("disable", []InteractionOption{{Name: "plugin", Value: "example-tool"}}))
	if err != nil {
		t.Fatalf("disable returned error: %v", err)
	}
	if manager.method != "DisableForGuild" || manager.pluginID != "example-tool" {
		t.Fatalf("manager = %+v, want disable call", manager)
	}
	if !strings.Contains(disableResponse.Content, "Disabled plugin") || !disableResponse.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral disable success", disableResponse)
	}
	if len(recorder.events) != 2 || recorder.events[0].Metadata["action"] != "enable" || recorder.events[1].Metadata["action"] != "disable" {
		t.Fatalf("audit events = %+v, want enable and disable audit", recorder.events)
	}
}

func TestPluginCommandDryRunsEnabledManifest(t *testing.T) {
	manifest := pluginManifest("jockie-music", "0.1.0")
	manifest.Name = "Jockie Music"
	manifest.Triggers = []plugins.Trigger{{Kind: "prefix", Value: "!play"}}
	manager := &fakePluginManager{manifests: []plugins.Manifest{manifest}}
	handler := PluginCommands(manager, &fakePluginFetcher{}, nil)[0].Handle

	response, err := handler(context.Background(), pluginInteraction("dry-run", []InteractionOption{
		{Name: "text", Value: "play never gonna give you up"},
	}))
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if manager.method != "EnabledForGuild" || manager.guildID != "guild-id" {
		t.Fatalf("manager = %+v, want enabled manifests lookup", manager)
	}
	if !strings.Contains(response.Content, "Matched external app: `Jockie Music`.") ||
		!strings.Contains(response.Content, "Planned command: `!play never gonna give you up`.") ||
		!response.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral dry-run plan", response)
	}
}

func TestPluginCommandDryRunFallsClosedWithoutMatch(t *testing.T) {
	manager := &fakePluginManager{manifests: []plugins.Manifest{pluginManifest("example-tool", "1.0.0")}}
	handler := PluginCommands(manager, &fakePluginFetcher{}, nil)[0].Handle

	response, err := handler(context.Background(), pluginInteraction("dry-run", []InteractionOption{
		{Name: "text", Value: "play never gonna give you up"},
	}))
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	if response.Content != "No enabled external app manifest matched that text." || !response.Ephemeral {
		t.Fatalf("response = %+v, want clean no-match response", response)
	}
}

func TestPluginCommandImportFailureIsCleanAndAudited(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	handler := PluginCommands(&fakePluginManager{}, &fakePluginFetcher{err: errors.New("manifest URL must be an HTTPS URL")}, recorder)[0].Handle

	response, err := handler(context.Background(), pluginInteraction("import-manifest", []InteractionOption{{Name: "url", Value: "http://example.test/gigi-plugin.json"}}))
	if err != nil {
		t.Fatalf("import returned error: %v", err)
	}
	if response.Content != "Manifest URL must be HTTPS." || !response.Ephemeral {
		t.Fatalf("response = %+v, want clean HTTPS error", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusFailed {
		t.Fatalf("audit events = %+v, want failed import audit", recorder.events)
	}
}

func pluginInteraction(action string, options []InteractionOption) Interaction {
	return Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "plugins",
		Options: []InteractionOption{{
			Name:    action,
			Options: options,
		}},
	}
}

func pluginManifest(id string, version string) plugins.Manifest {
	manifest := plugins.Manifest{
		ID:                   id,
		Name:                 "Example Tool",
		Version:              version,
		Source:               "builtin",
		SourceKind:           plugins.SourceKindKnown,
		DiscordApplicationID: "1511678703963209813",
		Capabilities: []plugins.Capability{{
			Name:        "example.run",
			Description: "Run example.",
		}},
		Triggers: []plugins.Trigger{{
			Kind:  "prefix",
			Value: "!example",
		}},
		Surfaces:    []string{"guild_text"},
		Permissions: []string{"example.run"},
		AuditEvents: []string{"plugin.example.run"},
		Attribution: []plugins.Resource{{
			Name:   "Example Provider",
			Use:    "Example data.",
			Source: "https://example.com",
		}},
	}
	return manifest
}

type fakePluginManager struct {
	method    string
	guildID   string
	pluginID  string
	version   string
	actorID   string
	manifest  plugins.Manifest
	manifests []plugins.Manifest
	err       error
}

func (m *fakePluginManager) ApprovedManifests(context.Context) ([]plugins.Manifest, error) {
	m.method = "ApprovedManifests"
	return m.manifests, m.err
}

func (m *fakePluginManager) UpsertApprovedManifest(_ context.Context, manifest plugins.Manifest, actorID string) error {
	m.method, m.manifest, m.actorID = "UpsertApprovedManifest", manifest, actorID
	return m.err
}

func (m *fakePluginManager) EnableForGuild(_ context.Context, guildID string, pluginID string, version string, actorID string) error {
	m.method, m.guildID, m.pluginID, m.version, m.actorID = "EnableForGuild", guildID, pluginID, version, actorID
	return m.err
}

func (m *fakePluginManager) DisableForGuild(_ context.Context, guildID string, pluginID string, actorID string) error {
	m.method, m.guildID, m.pluginID, m.actorID = "DisableForGuild", guildID, pluginID, actorID
	return m.err
}

func (m *fakePluginManager) EnabledForGuild(_ context.Context, guildID string) ([]plugins.Manifest, error) {
	m.method, m.guildID = "EnabledForGuild", guildID
	return m.manifests, m.err
}

type fakePluginFetcher struct {
	url        string
	attachment plugins.AttachmentSource
	manifest   plugins.Manifest
	err        error
}

func (f *fakePluginFetcher) Fetch(_ context.Context, manifestURL string) (plugins.Manifest, error) {
	f.url = manifestURL
	return f.manifest, f.err
}

func (f *fakePluginFetcher) FetchAttachment(_ context.Context, attachment plugins.AttachmentSource) (plugins.Manifest, error) {
	f.attachment = attachment
	return f.manifest, f.err
}
