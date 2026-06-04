package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode"

	"github.com/gOps132/GigiDC/internal/capability"
)

const maxManifestBytes = 1 << 20

type SourceKind string

const (
	SourceKindKnown       SourceKind = "known"
	SourceKindManifestURL SourceKind = "manifest_url"
)

type Manifest struct {
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	Version              string       `json:"version"`
	Source               string       `json:"source"`
	SourceKind           SourceKind   `json:"source_kind"`
	DiscordApplicationID string       `json:"discord_application_id,omitempty"`
	DiscordBotUserID     string       `json:"discord_bot_user_id,omitempty"`
	ManifestURL          string       `json:"manifest_url,omitempty"`
	Capabilities         []Capability `json:"capabilities"`
	Triggers             []Trigger    `json:"triggers"`
	Surfaces             []string     `json:"surfaces"`
	Permissions          []string     `json:"permissions"`
	ConfigSchema         string       `json:"config_schema,omitempty"`
	AuditEvents          []string     `json:"audit_events"`
	Attribution          []Resource   `json:"attribution"`
}

type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Trigger struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Resource struct {
	Name   string `json:"name"`
	Use    string `json:"use"`
	Source string `json:"source"`
}

type Registry interface {
	EnabledForGuild(ctx context.Context, guildID string) ([]Manifest, error)
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	manifest, err := decodeManifest(reader)
	if err != nil {
		return Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func DecodeManifestFromURL(body []byte, manifestURL string) (Manifest, error) {
	manifest, err := decodeManifest(bytes.NewReader(body))
	if err != nil {
		return Manifest{}, err
	}
	manifest.SourceKind = SourceKindManifestURL
	manifest.ManifestURL = strings.TrimSpace(manifestURL)
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func decodeManifest(reader io.Reader) (Manifest, error) {
	if reader == nil {
		return Manifest{}, fmt.Errorf("manifest reader is required")
	}
	var manifest Manifest
	decoder := json.NewDecoder(io.LimitReader(reader, maxManifestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("plugin id is required")
	}
	if !validIdentifier(m.ID) {
		return fmt.Errorf("plugin id contains unsupported characters")
	}
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(m.Source) == "" {
		return fmt.Errorf("source is required")
	}
	if m.SourceKind == "" {
		return fmt.Errorf("source kind is required")
	}
	if m.SourceKind != SourceKindKnown && m.SourceKind != SourceKindManifestURL {
		return fmt.Errorf("unsupported source kind %q", m.SourceKind)
	}
	if strings.TrimSpace(m.DiscordApplicationID) == "" && strings.TrimSpace(m.DiscordBotUserID) == "" {
		return fmt.Errorf("Discord application ID or bot user ID is required")
	}
	if strings.TrimSpace(m.DiscordApplicationID) != "" && !validDiscordID(m.DiscordApplicationID) {
		return fmt.Errorf("Discord application ID must be a snowflake ID")
	}
	if strings.TrimSpace(m.DiscordBotUserID) != "" && !validDiscordID(m.DiscordBotUserID) {
		return fmt.Errorf("Discord bot user ID must be a snowflake ID")
	}
	if m.SourceKind == SourceKindManifestURL {
		if err := validateManifestURL(m.ManifestURL); err != nil {
			return err
		}
	}
	if len(m.Capabilities) == 0 {
		return fmt.Errorf("capabilities are required")
	}
	for _, cap := range m.Capabilities {
		if _, err := capability.Normalize(cap.Name); err != nil {
			return fmt.Errorf("capability %q: %w", cap.Name, err)
		}
		if strings.TrimSpace(cap.Description) == "" {
			return fmt.Errorf("capability %q description is required", cap.Name)
		}
	}
	if len(m.Triggers) == 0 {
		return fmt.Errorf("triggers are required")
	}
	for _, trigger := range m.Triggers {
		if strings.TrimSpace(trigger.Kind) == "" || strings.TrimSpace(trigger.Value) == "" {
			return fmt.Errorf("trigger kind and value are required")
		}
	}
	if len(m.Surfaces) == 0 {
		return fmt.Errorf("surfaces are required")
	}
	for _, surface := range m.Surfaces {
		if strings.TrimSpace(surface) == "" {
			return fmt.Errorf("surface is required")
		}
	}
	for _, permission := range m.Permissions {
		if _, err := capability.Normalize(permission); err != nil {
			return fmt.Errorf("permission %q: %w", permission, err)
		}
	}
	if len(m.AuditEvents) == 0 {
		return fmt.Errorf("audit events are required")
	}
	for _, event := range m.AuditEvents {
		if strings.TrimSpace(event) == "" {
			return fmt.Errorf("audit event is required")
		}
	}
	if len(m.Attribution) == 0 {
		return fmt.Errorf("attribution is required")
	}
	for _, resource := range m.Attribution {
		if strings.TrimSpace(resource.Name) == "" || strings.TrimSpace(resource.Use) == "" || strings.TrimSpace(resource.Source) == "" {
			return fmt.Errorf("attribution name, use, and source are required")
		}
	}
	return nil
}

func validateManifestURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("manifest URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("manifest URL must be an HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("manifest URL must not include user info, query, or fragment")
	}
	return nil
}

func validDiscordID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 5 || len(value) > 30 {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if unicode.IsLower(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '.', '_', '-':
			continue
		default:
			return false
		}
	}
	return true
}
