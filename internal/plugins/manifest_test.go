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
