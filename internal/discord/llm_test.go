package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestLLMCommandsExposeGuildProviderSurface(t *testing.T) {
	commands := LLMCommands(&fakeLLMProviderManager{}, nil)
	if len(commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(commands))
	}
	command := commands[0]
	if command.Name != "llm" {
		t.Fatalf("command name = %q, want llm", command.Name)
	}
	providerGroup := findOption(command.Options, "provider")
	if providerGroup == nil || providerGroup.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatalf("provider group = %+v, want subcommand group", providerGroup)
	}
	for _, name := range []string{"list", "add", "test", "rotate", "delete"} {
		if findOption(providerGroup.Options, name) == nil {
			t.Fatalf("provider group missing %q", name)
		}
	}
	add := findOption(providerGroup.Options, "add")
	if option := findOption(add.Options, "provider"); option == nil || !hasChoice(option, string(provider.ProviderOpenAI)) || !hasChoice(option, string(provider.ProviderAnthropic)) || !hasChoice(option, string(provider.ProviderGemini)) {
		t.Fatalf("provider option = %+v, want provider choices", option)
	}

	modelGroup := findOption(command.Options, "model")
	if modelGroup == nil || modelGroup.Type != discordgo.ApplicationCommandOptionSubCommandGroup {
		t.Fatalf("model group = %+v, want subcommand group", modelGroup)
	}
	for _, name := range []string{"show", "set"} {
		if findOption(modelGroup.Options, name) == nil {
			t.Fatalf("model group missing %q", name)
		}
	}
	show := findOption(modelGroup.Options, "show")
	if option := findOption(show.Options, "purpose"); option == nil || !hasChoice(option, string(provider.PurposeChat)) || !hasChoice(option, string(provider.PurposeEmbedding)) {
		t.Fatalf("purpose option = %+v, want purpose choices", option)
	}
}

func TestLLMCommandDynamicCapabilities(t *testing.T) {
	tests := []struct {
		name string
		i    Interaction
		want capability.Capability
	}{
		{name: "provider list", i: llmInteraction("provider", "list", nil), want: "llm.provider.write"},
		{name: "provider add", i: llmInteraction("provider", "add", []InteractionOption{{Name: "provider", Value: "openai"}, {Name: "label", Value: "main"}}), want: "llm.provider.write"},
		{name: "provider add bad value still authorizes path", i: llmInteraction("provider", "add", []InteractionOption{{Name: "provider", Value: "wat"}, {Name: "label", Value: "main"}}), want: "llm.provider.write"},
		{name: "provider rotate", i: llmInteraction("provider", "rotate", []InteractionOption{{Name: "label", Value: "main"}}), want: "llm.provider.write"},
		{name: "provider delete", i: llmInteraction("provider", "delete", []InteractionOption{{Name: "label", Value: "main"}, {Name: "confirm", Value: "false"}}), want: "llm.provider.write"},
		{name: "provider test", i: llmInteraction("provider", "test", []InteractionOption{{Name: "label", Value: "main"}}), want: "llm.provider.test"},
		{name: "model show", i: llmInteraction("model", "show", []InteractionOption{{Name: "purpose", Value: "chat"}}), want: "llm.provider.select"},
		{name: "model set", i: llmInteraction("model", "set", []InteractionOption{{Name: "purpose", Value: "chat"}, {Name: "label", Value: "main"}, {Name: "model", Value: "gpt-4o-mini"}}), want: "llm.provider.select"},
		{name: "bad path", i: llmInteraction("provider", "wat", nil), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LLMCommands(&fakeLLMProviderManager{}, nil)[0].RequiredCapabilityFor(tt.i)
			if got != tt.want {
				t.Fatalf("capability = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLLMCommandRejectsDMs(t *testing.T) {
	handler := LLMCommands(&fakeLLMProviderManager{}, nil)[0].Handle

	response, err := handler(context.Background(), Interaction{
		UserID: "actor-id",
		Name:   "llm",
		Options: []InteractionOption{{
			Name: "provider",
			Options: []InteractionOption{{
				Name: "list",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Content != "LLM providers can only be managed inside a Discord server." || !response.Ephemeral {
		t.Fatalf("response = %+v, want guild-only error", response)
	}
}

func TestLLMCommandListsCredentialMetadata(t *testing.T) {
	manager := &fakeLLMProviderManager{records: []provider.CredentialRecord{{
		ID:             "credential-id",
		ProviderID:     provider.ProviderOpenAI,
		Label:          "main",
		Status:         provider.CredentialStatusActive,
		LastTestStatus: provider.TestStatusSucceeded,
		Ciphertext:     []byte("ciphertext"),
		Nonce:          []byte("nonce"),
	}}}
	handler := LLMCommands(manager, nil)[0].Handle

	response, err := handler(context.Background(), llmInteraction("provider", "list", nil))
	if err != nil {
		t.Fatalf("list returned error: %v", err)
	}
	if manager.method != "ListCredentials" || manager.owner.GuildID != "guild-id" || manager.owner.OwnerType != provider.OwnerGuild {
		t.Fatalf("manager = %+v, want guild list", manager)
	}
	if !strings.Contains(response.Content, "`main` - `openai`") || strings.Contains(response.Content, "ciphertext") || strings.Contains(response.Content, "nonce") || !response.Ephemeral {
		t.Fatalf("response = %+v, want metadata-only credential list", response)
	}
}

func TestLLMCommandProviderAddRotateDisabledWithoutCredentialEntry(t *testing.T) {
	handler := LLMCommands(&fakeLLMProviderManager{}, nil)[0].Handle

	addResponse, err := handler(context.Background(), llmInteraction("provider", "add", []InteractionOption{
		{Name: "provider", Value: "openai"},
		{Name: "label", Value: "main"},
	}))
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}
	if addResponse.Content != "LLM credential entry is not configured." || !addResponse.Ephemeral {
		t.Fatalf("response = %+v, want disabled credential entry message", addResponse)
	}

	testResponse, err := handler(context.Background(), llmInteraction("provider", "test", []InteractionOption{{Name: "label", Value: "main"}}))
	if err != nil {
		t.Fatalf("test returned error: %v", err)
	}
	if !strings.Contains(testResponse.Content, "Provider test requires") || !testResponse.Ephemeral {
		t.Fatalf("response = %+v, want tester-required message", testResponse)
	}
}

func TestLLMCommandProviderAddReturnsOpaqueModal(t *testing.T) {
	handler := LLMCommands(&fakeLLMProviderManager{}, nil, LLMCommandConfig{CredentialEntryEnabled: true})[0].Handle

	response, err := handler(context.Background(), llmInteraction("provider", "add", []InteractionOption{
		{Name: "provider", Value: "openai"},
		{Name: "label", Value: "main"},
	}))
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}
	if response.Modal == nil || response.Modal.Title != "Add LLM Credential" {
		t.Fatalf("response = %+v, want add modal", response)
	}
	if !strings.HasPrefix(response.Modal.CustomID, llmCredentialModalPrefix) {
		t.Fatalf("modal custom id = %q, want llm credential prefix", response.Modal.CustomID)
	}
	for _, forbidden := range []string{"openai", "main", "guild-id", "actor-id", "sk-"} {
		if strings.Contains(response.Modal.CustomID, forbidden) {
			t.Fatalf("modal custom id = %q, leaked %q", response.Modal.CustomID, forbidden)
		}
	}
}

func TestLLMCommandProviderAddRejectsSensitiveLabel(t *testing.T) {
	handler := LLMCommands(&fakeLLMProviderManager{}, nil, LLMCommandConfig{CredentialEntryEnabled: true})[0].Handle

	response, err := handler(context.Background(), llmInteraction("provider", "add", []InteractionOption{
		{Name: "provider", Value: "openai"},
		{Name: "label", Value: "sk-secret"},
	}))
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}
	if response.Content != "Credential label looks sensitive." || !response.Ephemeral {
		t.Fatalf("response = %+v, want sensitive label rejection", response)
	}
}

func TestLLMCommandProviderAddModalSubmitStoresCredentialAndAudits(t *testing.T) {
	manager := &fakeLLMProviderManager{}
	recorder := &fakeAuditRecorder{}
	command := LLMCommands(manager, recorder, LLMCommandConfig{CredentialEntryEnabled: true})[0]
	addResponse, err := command.Handle(context.Background(), llmInteraction("provider", "add", []InteractionOption{
		{Name: "provider", Value: "openai"},
		{Name: "label", Value: "main"},
	}))
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	submitResponse, err := command.HandleModal(context.Background(), ModalInteraction{
		GuildID:  "guild-id",
		UserID:   "actor-id",
		Name:     "llm",
		CustomID: addResponse.Modal.CustomID,
		Values:   map[string]string{llmCredentialInputID: "sk-test"},
	})
	if err != nil {
		t.Fatalf("modal submit returned error: %v", err)
	}
	if manager.method != "AddCredential" || manager.addReq.ProviderID != provider.ProviderOpenAI || manager.addReq.Label != "main" || manager.addReq.RawSecret != "sk-test" || manager.addReq.Owner.GuildID != "guild-id" || manager.addReq.ActorID != "actor-id" {
		t.Fatalf("manager = %+v, want add credential request", manager)
	}
	if strings.Contains(submitResponse.Content, "sk-test") || !strings.Contains(submitResponse.Content, "Saved `openai` credential `main`.") || !submitResponse.Ephemeral {
		t.Fatalf("response = %+v, want safe saved response", submitResponse)
	}
	if len(recorder.events) != 1 || recorder.events[0].Metadata["action"] != "add" || recorder.events[0].Metadata["provider_id"] != "openai" {
		t.Fatalf("audit events = %+v, want add audit", recorder.events)
	}
	for _, event := range recorder.events {
		for key, value := range event.Metadata {
			if strings.Contains(key, "secret") || strings.Contains(value, "sk-test") || strings.Contains(value, "main") {
				t.Fatalf("audit leaked sensitive value: %+v", event.Metadata)
			}
		}
	}
}

func TestLLMCommandProviderRotateModalSubmitStoresCredential(t *testing.T) {
	manager := &fakeLLMProviderManager{records: []provider.CredentialRecord{{
		ID:         "credential-id",
		ProviderID: provider.ProviderOpenAI,
		Label:      "main",
		Status:     provider.CredentialStatusActive,
	}}}
	command := LLMCommands(manager, nil, LLMCommandConfig{CredentialEntryEnabled: true})[0]
	rotateResponse, err := command.Handle(context.Background(), llmInteraction("provider", "rotate", []InteractionOption{{Name: "label", Value: "main"}}))
	if err != nil {
		t.Fatalf("rotate returned error: %v", err)
	}

	_, err = command.HandleModal(context.Background(), ModalInteraction{
		GuildID:  "guild-id",
		UserID:   "actor-id",
		Name:     "llm",
		CustomID: rotateResponse.Modal.CustomID,
		Values:   map[string]string{llmCredentialInputID: "sk-rotated"},
	})
	if err != nil {
		t.Fatalf("modal submit returned error: %v", err)
	}
	if manager.method != "RotateCredential" || manager.rotateReq.ProviderID != provider.ProviderOpenAI || manager.rotateReq.Label != "main" || manager.rotateReq.RawSecret != "sk-rotated" {
		t.Fatalf("manager = %+v, want rotate credential request", manager)
	}
}

func TestLLMCommandProviderModalSubmitRejectsReplayAndActorMismatch(t *testing.T) {
	manager := &fakeLLMProviderManager{}
	command := LLMCommands(manager, nil, LLMCommandConfig{CredentialEntryEnabled: true})[0]
	addResponse, err := command.Handle(context.Background(), llmInteraction("provider", "add", []InteractionOption{
		{Name: "provider", Value: "openai"},
		{Name: "label", Value: "main"},
	}))
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	mismatchResponse, err := command.HandleModal(context.Background(), ModalInteraction{
		GuildID:  "guild-id",
		UserID:   "other-user",
		Name:     "llm",
		CustomID: addResponse.Modal.CustomID,
		Values:   map[string]string{llmCredentialInputID: "sk-test"},
	})
	if err != nil {
		t.Fatalf("mismatch submit returned error: %v", err)
	}
	if mismatchResponse.Content != "LLM provider credential or model profile was not found." || !mismatchResponse.Ephemeral {
		t.Fatalf("response = %+v, want generic missing modal response", mismatchResponse)
	}

	replayResponse, err := command.HandleModal(context.Background(), ModalInteraction{
		GuildID:  "guild-id",
		UserID:   "actor-id",
		Name:     "llm",
		CustomID: addResponse.Modal.CustomID,
		Values:   map[string]string{llmCredentialInputID: "sk-test"},
	})
	if err != nil {
		t.Fatalf("replay submit returned error: %v", err)
	}
	if replayResponse.Content != "LLM provider credential or model profile was not found." || manager.method != "" {
		t.Fatalf("response = %+v manager = %+v, want no provider call", replayResponse, manager)
	}
}

func TestLLMCommandRevokesCredentialAndAudits(t *testing.T) {
	manager := &fakeLLMProviderManager{}
	recorder := &fakeAuditRecorder{}
	handler := LLMCommands(manager, recorder)[0].Handle

	response, err := handler(context.Background(), llmInteraction("provider", "delete", []InteractionOption{
		{Name: "label", Value: "main"},
		{Name: "confirm", Value: "true"},
	}))
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if manager.method != "RevokeCredential" || manager.owner.GuildID != "guild-id" || manager.label != "main" || manager.actorID != "actor-id" {
		t.Fatalf("manager = %+v, want revoke call", manager)
	}
	if !strings.Contains(response.Content, "Revoked LLM credential") || !response.Ephemeral {
		t.Fatalf("response = %+v, want ephemeral revoke success", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.llm.change" || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["action"] != "delete" {
		t.Fatalf("audit events = %+v, want successful delete audit", recorder.events)
	}
}

func TestLLMCommandRequiresDeleteConfirmation(t *testing.T) {
	manager := &fakeLLMProviderManager{}
	handler := LLMCommands(manager, nil)[0].Handle

	response, err := handler(context.Background(), llmInteraction("provider", "delete", []InteractionOption{
		{Name: "label", Value: "main"},
		{Name: "confirm", Value: "false"},
	}))
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if response.Content != "Confirm must be true to revoke a credential." || !response.Ephemeral {
		t.Fatalf("response = %+v, want confirmation error", response)
	}
	if manager.method != "" {
		t.Fatalf("manager = %+v, want no call", manager)
	}
}

func TestLLMCommandShowsAndSetsModelProfile(t *testing.T) {
	manager := &fakeLLMProviderManager{
		profile: provider.ModelProfile{
			ID:           "profile-id",
			CredentialID: "credential-id",
			ProviderID:   provider.ProviderOpenAI,
			ModelID:      "gpt-4o-mini",
		},
		records: []provider.CredentialRecord{{
			ID:         "credential-id",
			ProviderID: provider.ProviderOpenAI,
			Label:      "main",
			Status:     provider.CredentialStatusActive,
		}},
	}
	recorder := &fakeAuditRecorder{}
	handler := LLMCommands(manager, recorder)[0].Handle

	showResponse, err := handler(context.Background(), llmInteraction("model", "show", []InteractionOption{{Name: "purpose", Value: "chat"}}))
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if manager.method != "ActiveModelProfile" || manager.purpose != provider.PurposeChat || !strings.Contains(showResponse.Content, "Active `chat` model") {
		t.Fatalf("response = %+v manager = %+v, want active profile", showResponse, manager)
	}

	setResponse, err := handler(context.Background(), llmInteraction("model", "set", []InteractionOption{
		{Name: "purpose", Value: "chat"},
		{Name: "label", Value: "main"},
		{Name: "model", Value: "gpt-4o-mini"},
	}))
	if err != nil {
		t.Fatalf("set returned error: %v", err)
	}
	if manager.method != "SelectModelProfile" || manager.selectReq.CredentialID != "credential-id" || manager.selectReq.ProviderID != provider.ProviderOpenAI || manager.selectReq.ActorID != "actor-id" {
		t.Fatalf("manager = %+v, want selected model profile", manager)
	}
	if !strings.Contains(setResponse.Content, "Selected `gpt-4o-mini`") || !setResponse.Ephemeral {
		t.Fatalf("response = %+v, want selected model response", setResponse)
	}
	if len(recorder.events) != 1 || recorder.events[0].Metadata["action"] != "set" || recorder.events[0].Metadata["model_id"] != "gpt-4o-mini" {
		t.Fatalf("audit events = %+v, want model set audit", recorder.events)
	}
}

func TestLLMCommandModelSetRejectsMissingLabel(t *testing.T) {
	manager := &fakeLLMProviderManager{}
	handler := LLMCommands(manager, nil)[0].Handle

	response, err := handler(context.Background(), llmInteraction("model", "set", []InteractionOption{
		{Name: "purpose", Value: "chat"},
		{Name: "label", Value: "main"},
		{Name: "model", Value: "gpt-4o-mini"},
	}))
	if err != nil {
		t.Fatalf("set returned error: %v", err)
	}
	if response.Content != "LLM provider credential or model profile was not found." || !response.Ephemeral {
		t.Fatalf("response = %+v, want clean missing credential error", response)
	}
}

func TestLLMCommandFailureIsCleanAndAudited(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	handler := LLMCommands(&fakeLLMProviderManager{err: errors.New("db down")}, recorder)[0].Handle

	response, err := handler(context.Background(), llmInteraction("provider", "delete", []InteractionOption{
		{Name: "label", Value: "main"},
		{Name: "confirm", Value: "true"},
	}))
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if response.Content != "LLM command failed." || !response.Ephemeral {
		t.Fatalf("response = %+v, want clean failure", response)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusFailed {
		t.Fatalf("audit events = %+v, want failed delete audit", recorder.events)
	}
}

func llmInteraction(group string, action string, options []InteractionOption) Interaction {
	return Interaction{
		GuildID: "guild-id",
		UserID:  "actor-id",
		Name:    "llm",
		Options: []InteractionOption{{
			Name: group,
			Options: []InteractionOption{{
				Name:    action,
				Options: options,
			}},
		}},
	}
}

type fakeLLMProviderManager struct {
	method    string
	owner     provider.Scope
	label     string
	actorID   string
	purpose   provider.Purpose
	selectReq provider.SelectModelRequest
	addReq    provider.AddCredentialRequest
	rotateReq provider.AddCredentialRequest
	records   []provider.CredentialRecord
	profile   provider.ModelProfile
	err       error
}

func (m *fakeLLMProviderManager) AddCredential(_ context.Context, req provider.AddCredentialRequest) (provider.CredentialRecord, error) {
	m.method, m.addReq = "AddCredential", req
	return provider.CredentialRecord{ID: "credential-id", ProviderID: req.ProviderID, Label: req.Label, Status: provider.CredentialStatusActive}, m.err
}

func (m *fakeLLMProviderManager) RotateCredential(_ context.Context, req provider.AddCredentialRequest) (provider.CredentialRecord, error) {
	m.method, m.rotateReq = "RotateCredential", req
	return provider.CredentialRecord{ID: "credential-id", ProviderID: req.ProviderID, Label: req.Label, Status: provider.CredentialStatusActive}, m.err
}

func (m *fakeLLMProviderManager) ListCredentials(_ context.Context, owner provider.Scope) ([]provider.CredentialRecord, error) {
	m.method, m.owner = "ListCredentials", owner
	return m.records, m.err
}

func (m *fakeLLMProviderManager) RevokeCredential(_ context.Context, owner provider.Scope, label string, actorID string) error {
	m.method, m.owner, m.label, m.actorID = "RevokeCredential", owner, label, actorID
	return m.err
}

func (m *fakeLLMProviderManager) SelectModelProfile(_ context.Context, req provider.SelectModelRequest) error {
	m.method, m.selectReq = "SelectModelProfile", req
	return m.err
}

func (m *fakeLLMProviderManager) ActiveModelProfile(_ context.Context, owner provider.Scope, purpose provider.Purpose) (provider.ModelProfile, error) {
	m.method, m.owner, m.purpose = "ActiveModelProfile", owner, purpose
	return m.profile, m.err
}
