package plugins

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManifestValidateAcceptsKnownDiscordIdentity(t *testing.T) {
	manifest := validManifest()

	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestManifestValidateRequiresExactDiscordIdentity(t *testing.T) {
	manifest := validManifest()
	manifest.DiscordApplicationID = ""
	manifest.DiscordBotUserID = ""

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "Discord application ID or bot user ID") {
		t.Fatalf("error = %v, want Discord identity requirement", err)
	}
}

func TestManifestValidateRejectsManifestURLSourceWithoutHTTPSURL(t *testing.T) {
	manifest := validManifest()
	manifest.SourceKind = SourceKindManifestURL
	manifest.ManifestURL = "http://example.test/gigi-plugin.json"

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "manifest URL") {
		t.Fatalf("error = %v, want HTTPS manifest URL requirement", err)
	}
}

func TestManifestValidateRejectsManifestURLSecrets(t *testing.T) {
	manifest := validManifest()
	manifest.SourceKind = SourceKindManifestURL
	manifest.ManifestURL = "https://example.test/gigi-plugin.json?token=value"

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "query") {
		t.Fatalf("error = %v, want query rejection", err)
	}
}

func TestManifestValidateAcceptsSendMessageDispatch(t *testing.T) {
	manifest := validManifest()
	manifest.Dispatch = DispatchModeSendMessage

	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestManifestValidateAcceptsActionContracts(t *testing.T) {
	manifest := validManifest()
	manifest.Triggers = nil
	manifest.Permissions = nil
	manifest.Actions = []Action{{
		ID:          "play",
		Name:        "Play",
		Description: "Play a track.",
		Trigger:     Trigger{Kind: "prefix", Value: "!play", Aliases: []string{"play"}},
		Surfaces:    []string{"guild_text"},
		Permissions: []string{"plugin.install"},
		Safety:      SafetyClassRestricted,
		Dispatch:    DispatchModeSendMessage,
		Adapter:     DispatchAdapterPrefixCommand,
		ArgSchema:   `{"type":"object","properties":{"query":{"type":"string"}}}`,
	}}
	manifest.ConfigSchema = `{"type":"object","properties":{"dj_role":{"type":"string"}}}`

	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestManifestValidateRejectsInvalidActionArgSchema(t *testing.T) {
	manifest := validManifest()
	manifest.Actions = []Action{{
		ID:        "play",
		Trigger:   Trigger{Kind: "prefix", Value: "!play"},
		Safety:    SafetyClassPublic,
		ArgSchema: `["not","object"]`,
	}}

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "arg schema") {
		t.Fatalf("error = %v, want arg schema validation", err)
	}
}

func TestManifestValidateRejectsUnsupportedDispatchMode(t *testing.T) {
	manifest := validManifest()
	manifest.Dispatch = "launch_missiles"

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported dispatch mode") {
		t.Fatalf("error = %v, want unsupported dispatch mode", err)
	}
}

func TestManifestValidateRejectsEmptyTriggerAlias(t *testing.T) {
	manifest := validManifest()
	manifest.Triggers[0].Aliases = []string{" "}

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "trigger alias") {
		t.Fatalf("error = %v, want trigger alias validation", err)
	}
}

func TestManifestValidateRejectsInvalidConfigSchema(t *testing.T) {
	manifest := validManifest()
	manifest.ConfigSchema = "[]"

	err := manifest.Validate()
	if err == nil || !strings.Contains(err.Error(), "config schema") {
		t.Fatalf("error = %v, want config schema validation", err)
	}
}

func TestDecodeManifestValidatesJSON(t *testing.T) {
	_, err := DecodeManifest(strings.NewReader(`{"id":"example-tool"}`))
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("error = %v, want validation failure", err)
	}
}

func TestDecodeManifestFromURLAppliesURLSource(t *testing.T) {
	manifest := validManifest()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	got, err := DecodeManifestFromURL(raw, "https://example.test/gigi-plugin.json")
	if err != nil {
		t.Fatalf("DecodeManifestFromURL returned error: %v", err)
	}
	if got.SourceKind != SourceKindManifestURL || got.ManifestURL != "https://example.test/gigi-plugin.json" {
		t.Fatalf("manifest source = %q/%q, want manifest_url source", got.SourceKind, got.ManifestURL)
	}
}

func TestDecodeManifestFromAttachmentAppliesUploadedFileSource(t *testing.T) {
	manifest := validManifest()
	manifest.SourceKind = SourceKindManifestURL
	manifest.ManifestURL = "https://example.test/gigi-plugin.json"
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	got, err := DecodeManifestFromAttachment(raw)
	if err != nil {
		t.Fatalf("DecodeManifestFromAttachment returned error: %v", err)
	}
	if got.SourceKind != SourceKindUploadedFile || got.ManifestURL != "" {
		t.Fatalf("manifest source = %q/%q, want uploaded_file source without URL", got.SourceKind, got.ManifestURL)
	}
}

func validManifest() Manifest {
	return Manifest{
		ID:                   "example-tool",
		Name:                 "Example Tool",
		Version:              "1.0.0",
		Source:               "builtin",
		SourceKind:           SourceKindKnown,
		DiscordApplicationID: "1511678703963209813",
		DiscordBotUserID:     "1511678703963209814",
		Capabilities: []Capability{{
			Name:        "example.run",
			Description: "Run the plugin's declared action.",
		}},
		Triggers: []Trigger{{
			Kind:  "prefix",
			Value: "!example",
		}},
		Surfaces:    []string{"guild_text"},
		Permissions: []string{"example.run"},
		AuditEvents: []string{"plugin.example.run"},
		Attribution: []Resource{{
			Name:   "Example Provider",
			Use:    "Provide data for the plugin action.",
			Source: "https://example.com",
		}},
	}
}
