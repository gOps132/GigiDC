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
	SourceKindKnown        SourceKind = "known"
	SourceKindManifestURL  SourceKind = "manifest_url"
	SourceKindUploadedFile SourceKind = "uploaded_file"
)

type DispatchMode string

const (
	DispatchModeDryRun      DispatchMode = "dry_run"
	DispatchModeSendMessage DispatchMode = "send_message"
)

type SafetyClass string

const (
	SafetyClassPublic     SafetyClass = "public"
	SafetyClassRestricted SafetyClass = "restricted"
)

type DispatchAdapter string

const (
	DispatchAdapterPrefixCommand DispatchAdapter = "prefix_command"
)

type Manifest struct {
	ID                    string       `json:"id"`
	Name                  string       `json:"name"`
	Version               string       `json:"version"`
	Source                string       `json:"source"`
	SourceKind            SourceKind   `json:"source_kind"`
	DiscordApplicationID  string       `json:"discord_application_id,omitempty"`
	DiscordBotUserID      string       `json:"discord_bot_user_id,omitempty"`
	ManifestURL           string       `json:"manifest_url,omitempty"`
	Capabilities          []Capability `json:"capabilities"`
	Triggers              []Trigger    `json:"triggers"`
	Actions               []Action     `json:"actions,omitempty"`
	Surfaces              []string     `json:"surfaces"`
	Permissions           []string     `json:"permissions"`
	Dispatch              DispatchMode `json:"dispatch,omitempty"`
	PublicDispatchAllowed bool         `json:"-"`
	ConfigSchema          string       `json:"config_schema,omitempty"`
	AuditEvents           []string     `json:"audit_events"`
	Attribution           []Resource   `json:"attribution"`
}

type Action struct {
	ID          string          `json:"id"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Trigger     Trigger         `json:"trigger"`
	Surfaces    []string        `json:"surfaces,omitempty"`
	Permissions []string        `json:"permissions,omitempty"`
	Safety      SafetyClass     `json:"safety"`
	Dispatch    DispatchMode    `json:"dispatch,omitempty"`
	Adapter     DispatchAdapter `json:"adapter,omitempty"`
	ArgSchema   string          `json:"arg_schema,omitempty"`
}

type Capability struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Trigger struct {
	Kind    string   `json:"kind"`
	Value   string   `json:"value"`
	Aliases []string `json:"aliases,omitempty"`
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

func DecodeManifestFromAttachment(body []byte) (Manifest, error) {
	manifest, err := decodeManifest(bytes.NewReader(body))
	if err != nil {
		return Manifest{}, err
	}
	manifest.SourceKind = SourceKindUploadedFile
	manifest.ManifestURL = ""
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
	if m.SourceKind != SourceKindKnown && m.SourceKind != SourceKindManifestURL && m.SourceKind != SourceKindUploadedFile {
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
	if len(m.Triggers) == 0 && len(m.Actions) == 0 {
		return fmt.Errorf("triggers are required")
	}
	for _, trigger := range m.Triggers {
		if err := validateTrigger(trigger); err != nil {
			return err
		}
	}
	if len(m.Surfaces) == 0 && len(m.Actions) == 0 {
		return fmt.Errorf("surfaces are required")
	}
	if err := validateSurfaces(m.Surfaces, "surface"); err != nil {
		return err
	}
	if err := validatePermissions(m.Permissions, "permission"); err != nil {
		return err
	}
	if err := validateDispatchMode(m.Dispatch); err != nil {
		return err
	}
	for _, action := range m.Actions {
		if err := m.validateAction(action); err != nil {
			return err
		}
	}
	if err := validateJSONObject(m.ConfigSchema, "config schema"); err != nil {
		return err
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

func (m Manifest) NormalizedActions() []Action {
	if len(m.Actions) > 0 {
		actions := make([]Action, 0, len(m.Actions))
		for _, action := range m.Actions {
			actions = append(actions, m.normalizeAction(action, ""))
		}
		return actions
	}
	actions := make([]Action, 0, len(m.Triggers))
	for index, trigger := range m.Triggers {
		actionID := "legacy"
		if len(m.Triggers) > 1 {
			actionID = fmt.Sprintf("legacy-%d", index+1)
		}
		actions = append(actions, Action{
			ID:          actionID,
			Trigger:     trigger,
			Surfaces:    append([]string(nil), m.Surfaces...),
			Permissions: append([]string(nil), m.Permissions...),
			Safety:      safetyForPermissions(m.Permissions),
			Dispatch:    normalizedDispatchMode(m.Dispatch),
			Adapter:     DispatchAdapterPrefixCommand,
		})
	}
	return actions
}

func (m Manifest) HasPublicSendMessageAction() bool {
	for _, action := range m.NormalizedActions() {
		if action.Dispatch == DispatchModeSendMessage && action.Safety == SafetyClassPublic && len(action.Permissions) == 0 {
			return true
		}
	}
	return false
}

func (m Manifest) PublicDispatchConsentRequired() bool {
	return m.HasPublicSendMessageAction()
}

func (m Manifest) normalizeAction(action Action, defaultID string) Action {
	action.ID = strings.TrimSpace(action.ID)
	if action.ID == "" {
		action.ID = defaultID
	}
	if len(action.Surfaces) == 0 {
		action.Surfaces = append([]string(nil), m.Surfaces...)
	} else {
		action.Surfaces = append([]string(nil), action.Surfaces...)
	}
	action.Permissions = append([]string(nil), action.Permissions...)
	if action.Safety == "" {
		action.Safety = safetyForPermissions(action.Permissions)
	}
	if action.Dispatch == "" {
		action.Dispatch = normalizedDispatchMode(m.Dispatch)
	} else {
		action.Dispatch = normalizedDispatchMode(action.Dispatch)
	}
	if action.Adapter == "" {
		action.Adapter = DispatchAdapterPrefixCommand
	}
	return action
}

func (m Manifest) validateAction(action Action) error {
	actionID := strings.TrimSpace(action.ID)
	if actionID == "" {
		return fmt.Errorf("action id is required")
	}
	if !validIdentifier(actionID) {
		return fmt.Errorf("action %q id contains unsupported characters", actionID)
	}
	if err := validateTrigger(action.Trigger); err != nil {
		return fmt.Errorf("action %q: %w", actionID, err)
	}
	surfaces := action.Surfaces
	if len(surfaces) == 0 {
		surfaces = m.Surfaces
	}
	if len(surfaces) == 0 {
		return fmt.Errorf("action %q surfaces are required", actionID)
	}
	if err := validateSurfaces(surfaces, fmt.Sprintf("action %q surface", actionID)); err != nil {
		return err
	}
	if err := validatePermissions(action.Permissions, fmt.Sprintf("action %q permission", actionID)); err != nil {
		return err
	}
	if action.Safety != SafetyClassPublic && action.Safety != SafetyClassRestricted {
		return fmt.Errorf("action %q has unsupported safety class %q", actionID, action.Safety)
	}
	if action.Safety == SafetyClassPublic && len(action.Permissions) > 0 {
		return fmt.Errorf("action %q public safety cannot require permissions", actionID)
	}
	if action.Safety == SafetyClassRestricted && len(action.Permissions) == 0 {
		return fmt.Errorf("action %q restricted safety requires permissions", actionID)
	}
	if err := validateDispatchMode(action.Dispatch); err != nil {
		return fmt.Errorf("action %q: %w", actionID, err)
	}
	if action.Adapter != "" && action.Adapter != DispatchAdapterPrefixCommand {
		return fmt.Errorf("action %q has unsupported dispatch adapter %q", actionID, action.Adapter)
	}
	if err := validateJSONObject(action.ArgSchema, fmt.Sprintf("action %q arg schema", actionID)); err != nil {
		return err
	}
	return nil
}

func validateTrigger(trigger Trigger) error {
	if strings.TrimSpace(trigger.Kind) == "" || strings.TrimSpace(trigger.Value) == "" {
		return fmt.Errorf("trigger kind and value are required")
	}
	for _, alias := range trigger.Aliases {
		if strings.TrimSpace(alias) == "" {
			return fmt.Errorf("trigger alias is required")
		}
	}
	return nil
}

func validateSurfaces(surfaces []string, label string) error {
	for _, surface := range surfaces {
		if strings.TrimSpace(surface) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	return nil
}

func validatePermissions(permissions []string, label string) error {
	for _, permission := range permissions {
		if _, err := capability.Normalize(permission); err != nil {
			return fmt.Errorf("%s %q: %w", label, permission, err)
		}
	}
	return nil
}

func validateDispatchMode(dispatch DispatchMode) error {
	if dispatch != "" && dispatch != DispatchModeDryRun && dispatch != DispatchModeSendMessage {
		return fmt.Errorf("unsupported dispatch mode %q", dispatch)
	}
	return nil
}

func validateJSONObject(value string, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(value), &schema); err != nil {
		return fmt.Errorf("%s must be a JSON object: %w", label, err)
	}
	if schema == nil {
		return fmt.Errorf("%s must be a JSON object", label)
	}
	return nil
}

func safetyForPermissions(permissions []string) SafetyClass {
	if len(permissions) == 0 {
		return SafetyClassPublic
	}
	return SafetyClassRestricted
}

func normalizedDispatchMode(dispatch DispatchMode) DispatchMode {
	if dispatch == "" {
		return DispatchModeDryRun
	}
	return dispatch
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
