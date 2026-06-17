package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/llm/provider"
)

type LLMProviderManager interface {
	ListCredentials(ctx context.Context, owner provider.Scope) ([]provider.CredentialRecord, error)
	RevokeCredential(ctx context.Context, owner provider.Scope, label string, actorID string) error
	SelectModelProfile(ctx context.Context, req provider.SelectModelRequest) error
	ActiveModelProfile(ctx context.Context, owner provider.Scope, purpose provider.Purpose) (provider.ModelProfile, error)
}

func LLMCommands(manager LLMProviderManager, recorder AuditRecorder) []Command {
	return []Command{{
		Name:                  "llm",
		Description:           "Manage Gigi LLM provider settings.",
		RequiredCapabilityFor: llmRequiredCapability,
		Options: []*discordgo.ApplicationCommandOption{
			llmProviderGroup(),
			llmModelGroup(),
		},
		Handle: llmHandler(manager, recorder),
	}}
}

func llmProviderGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "provider",
		Description: "Manage guild-owned LLM provider credentials.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "list",
				Description: "List configured provider credentials.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "Start credential entry for a provider.",
				Options: []*discordgo.ApplicationCommandOption{
					providerOption(),
					stringOption("label", "Credential label.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "test",
				Description: "Test a configured provider credential.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("label", "Credential label.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "rotate",
				Description: "Start credential rotation for a provider.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("label", "Credential label.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "delete",
				Description: "Revoke a configured provider credential.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("label", "Credential label.", nil),
					boolOption("confirm", "Confirm credential revocation."),
				},
			},
		},
	}
}

func llmModelGroup() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "model",
		Description: "Manage guild LLM model profiles.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "show",
				Description: "Show the active model for a purpose.",
				Options: []*discordgo.ApplicationCommandOption{
					purposeOption(),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "set",
				Description: "Select a model for a purpose.",
				Options: []*discordgo.ApplicationCommandOption{
					purposeOption(),
					stringOption("label", "Credential label.", nil),
					stringOption("model", "Provider model id.", nil),
				},
			},
		},
	}
}

func boolOption(name string, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        name,
		Description: description,
		Required:    true,
	}
}

func providerOption() *discordgo.ApplicationCommandOption {
	return stringOption("provider", "LLM provider.", []*discordgo.ApplicationCommandOptionChoice{
		{Name: "OpenAI", Value: string(provider.ProviderOpenAI)},
		{Name: "Anthropic", Value: string(provider.ProviderAnthropic)},
		{Name: "Gemini", Value: string(provider.ProviderGemini)},
		{Name: "Custom", Value: string(provider.ProviderCustom)},
	})
}

func purposeOption() *discordgo.ApplicationCommandOption {
	return stringOption("purpose", "LLM purpose.", []*discordgo.ApplicationCommandOptionChoice{
		{Name: "chat", Value: string(provider.PurposeChat)},
		{Name: "reasoning", Value: string(provider.PurposeReasoning)},
		{Name: "embedding", Value: string(provider.PurposeEmbedding)},
		{Name: "routing", Value: string(provider.PurposeRouting)},
	})
}

func llmRequiredCapability(interaction Interaction) capability.Capability {
	group, action, ok := llmPath(interaction)
	if !ok {
		return ""
	}
	switch {
	case group == "provider" && (action == "add" || action == "rotate" || action == "delete" || action == "list"):
		return capability.Capability("llm.provider.write")
	case group == "provider" && action == "test":
		return capability.Capability("llm.provider.test")
	case group == "model" && (action == "show" || action == "set"):
		return capability.Capability("llm.provider.select")
	default:
		return ""
	}
}

func llmPath(interaction Interaction) (string, string, bool) {
	if len(interaction.Options) != 1 {
		return "", "", false
	}
	group := interaction.Options[0]
	if len(group.Options) != 1 {
		return group.Name, "", false
	}
	return group.Name, group.Options[0].Name, true
}

func llmHandler(manager LLMProviderManager, recorder AuditRecorder) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if manager == nil {
			return CommandResponse{}, fmt.Errorf("llm provider manager is required")
		}
		request, err := parseLLMRequest(interaction)
		if err != nil {
			_ = recordLLMAction(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}

		response, err := executeLLMRequest(ctx, manager, interaction, &request)
		if err != nil {
			_ = recordLLMAction(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: cleanLLMError(err), Ephemeral: true}, nil
		}
		if shouldAuditLLMAction(request) {
			if err := recordLLMAction(ctx, recorder, interaction, request, audit.StatusSucceeded, nil); err != nil {
				return CommandResponse{}, err
			}
		}
		return CommandResponse{Content: response, Ephemeral: true}, nil
	}
}

type llmRequest struct {
	Group      string
	Action     string
	ProviderID provider.ProviderID
	Label      string
	Purpose    provider.Purpose
	ModelID    string
	Confirm    bool
}

func parseLLMRequest(interaction Interaction) (llmRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return llmRequest{}, fmt.Errorf("LLM providers can only be managed inside a Discord server.")
	}
	if len(interaction.Options) != 1 {
		return llmRequest{}, fmt.Errorf("Choose one llm group.")
	}
	group := interaction.Options[0]
	if len(group.Options) != 1 {
		return llmRequest{Group: group.Name}, fmt.Errorf("Choose one llm action.")
	}
	action := group.Options[0]
	request := llmRequest{Group: group.Name, Action: action.Name}

	switch request.Group {
	case "provider":
		return parseLLMProviderRequest(request, action.Options)
	case "model":
		return parseLLMModelRequest(request, action.Options)
	default:
		return request, fmt.Errorf("Unsupported llm group.")
	}
}

func parseLLMProviderRequest(request llmRequest, options []InteractionOption) (llmRequest, error) {
	switch request.Action {
	case "list":
		return request, nil
	case "add":
		request.ProviderID = provider.ProviderID(optionByName(options, "provider"))
		if err := provider.ValidateProvider(request.ProviderID); err != nil {
			return request, err
		}
		return parseLLMLabel(request, options)
	case "test", "rotate":
		return parseLLMLabel(request, options)
	case "delete":
		request, err := parseLLMLabel(request, options)
		if err != nil {
			return request, err
		}
		request.Confirm = boolByName(options, "confirm")
		if !request.Confirm {
			return request, fmt.Errorf("Confirm must be true to revoke a credential.")
		}
		return request, nil
	default:
		return request, fmt.Errorf("Unsupported llm provider action.")
	}
}

func parseLLMModelRequest(request llmRequest, options []InteractionOption) (llmRequest, error) {
	purpose := provider.Purpose(optionByName(options, "purpose"))
	if err := provider.ValidatePurpose(purpose); err != nil {
		return request, err
	}
	request.Purpose = purpose

	switch request.Action {
	case "show":
		return request, nil
	case "set":
		request, err := parseLLMLabel(request, options)
		if err != nil {
			return request, err
		}
		modelID, err := provider.ValidateModelID(optionByName(options, "model"))
		if err != nil {
			return request, err
		}
		request.ModelID = modelID
		return request, nil
	default:
		return request, fmt.Errorf("Unsupported llm model action.")
	}
}

func parseLLMLabel(request llmRequest, options []InteractionOption) (llmRequest, error) {
	request.Label = strings.TrimSpace(optionByName(options, "label"))
	if request.Label == "" {
		return request, fmt.Errorf("Credential label is required.")
	}
	if len(request.Label) > 80 {
		return request, fmt.Errorf("Credential label is too long.")
	}
	return request, nil
}

func executeLLMRequest(ctx context.Context, manager LLMProviderManager, interaction Interaction, request *llmRequest) (string, error) {
	owner := provider.Scope{OwnerType: provider.OwnerGuild, GuildID: interaction.GuildID}

	switch {
	case request.Group == "provider" && request.Action == "list":
		records, err := manager.ListCredentials(ctx, owner)
		if err != nil {
			return "", err
		}
		return formatLLMCredentialList(records), nil
	case request.Group == "provider" && (request.Action == "add" || request.Action == "rotate"):
		return "Credential entry requires a Discord modal or private flow; not enabled yet.", nil
	case request.Group == "provider" && request.Action == "test":
		return "Provider test requires provider client adapters; not enabled yet.", nil
	case request.Group == "provider" && request.Action == "delete":
		if err := manager.RevokeCredential(ctx, owner, request.Label, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Revoked LLM credential `%s`.", safeInline(request.Label)), nil
	case request.Group == "model" && request.Action == "show":
		profile, err := manager.ActiveModelProfile(ctx, owner, request.Purpose)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Active `%s` model: `%s` via `%s` (`%s`).",
			request.Purpose,
			safeInline(profile.ModelID),
			safeInline(string(profile.ProviderID)),
			safeInline(profile.CredentialID),
		), nil
	case request.Group == "model" && request.Action == "set":
		credential, err := credentialByLabel(ctx, manager, owner, request.Label)
		if err != nil {
			return "", err
		}
		if err := manager.SelectModelProfile(ctx, provider.SelectModelRequest{
			Owner:        owner,
			Purpose:      request.Purpose,
			CredentialID: credential.ID,
			ProviderID:   credential.ProviderID,
			ModelID:      request.ModelID,
			ParamsJSON:   "{}",
			ActorID:      interaction.UserID,
		}); err != nil {
			return "", err
		}
		return fmt.Sprintf("Selected `%s` for `%s` using credential `%s`.", safeInline(request.ModelID), request.Purpose, safeInline(request.Label)), nil
	default:
		return "", fmt.Errorf("unsupported llm action")
	}
}

func credentialByLabel(ctx context.Context, manager LLMProviderManager, owner provider.Scope, label string) (provider.CredentialRecord, error) {
	records, err := manager.ListCredentials(ctx, owner)
	if err != nil {
		return provider.CredentialRecord{}, err
	}
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Label), strings.TrimSpace(label)) && record.Status == provider.CredentialStatusActive {
			return record, nil
		}
	}
	return provider.CredentialRecord{}, fmt.Errorf("credential label was not found")
}

func formatLLMCredentialList(records []provider.CredentialRecord) string {
	if len(records) == 0 {
		return "LLM provider credentials: none."
	}
	limit := len(records)
	if limit > 10 {
		limit = 10
	}
	lines := []string{"LLM provider credentials:"}
	for _, record := range records[:limit] {
		testStatus := record.LastTestStatus
		if testStatus == "" {
			testStatus = provider.TestStatusUntested
		}
		lines = append(lines, fmt.Sprintf("- `%s` - `%s` (%s, test: %s)", safeInline(record.Label), safeInline(string(record.ProviderID)), record.Status, testStatus))
	}
	if len(records) > limit {
		lines = append(lines, fmt.Sprintf("...and %d more.", len(records)-limit))
	}
	return strings.Join(lines, "\n")
}

func boolByName(options []InteractionOption, name string) bool {
	value := strings.ToLower(strings.TrimSpace(optionByName(options, name)))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func cleanLLMError(err error) string {
	if err == nil {
		return "LLM command failed."
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "not found"):
		return "LLM provider credential or model profile was not found."
	case strings.Contains(message, "does not support purpose"):
		return "Provider does not support that LLM purpose."
	case strings.Contains(message, "model"):
		return "Model selection is invalid."
	default:
		return "LLM command failed."
	}
}

func shouldAuditLLMAction(request llmRequest) bool {
	switch {
	case request.Group == "provider" && request.Action == "delete":
		return true
	case request.Group == "model" && request.Action == "set":
		return true
	default:
		return false
	}
}

func recordLLMAction(ctx context.Context, recorder AuditRecorder, interaction Interaction, request llmRequest, status audit.Status, err error) error {
	if recorder == nil || strings.TrimSpace(interaction.UserID) == "" || !shouldAuditLLMAction(request) {
		return nil
	}
	reason := ""
	if err != nil {
		reason = "llm_action_failed"
	}
	metadata := map[string]string{
		"command": interaction.Name,
		"group":   request.Group,
		"action":  request.Action,
	}
	if request.ProviderID != "" {
		metadata["provider_id"] = string(request.ProviderID)
	}
	if request.Purpose != "" {
		metadata["purpose"] = string(request.Purpose)
	}
	if request.ModelID != "" {
		metadata["model_id"] = request.ModelID
	}
	return recorder.Record(ctx, audit.Event{
		Kind:     "discord.llm.change",
		GuildID:  interaction.GuildID,
		ActorID:  interaction.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}
