package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/memory"
)

type MemoryManager interface {
	GuildStatus(ctx context.Context, guildID string) (memory.Status, error)
	UpsertChannelPolicy(ctx context.Context, req memory.UpsertChannelPolicyRequest) (memory.ChannelPolicy, error)
	CountMentions(ctx context.Context, req memory.CountRequest) (memory.CountResult, error)
	SearchMessages(ctx context.Context, req memory.SearchRequest) ([]memory.SearchResult, error)
}

func MemoryCommands(manager MemoryManager, recorder AuditRecorder) []Command {
	return []Command{{
		Name:                  "memory",
		Description:           "Manage Gigi guild memory settings.",
		RequiredCapabilityFor: memoryRequiredCapability,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "status",
				Description: "Show guild memory status.",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "count",
				Description: "Count exact text mentions in readable memory.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("text", "Text to count.", nil),
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "user",
						Description: "Discord user.",
						Required:    false,
					},
					memoryScopeOption(),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "search",
				Description: "Search retained memory in this channel.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("query", "Search query.", nil),
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "user",
						Description: "Discord user.",
						Required:    false,
					},
					integerOption("limit", "Result limit, 1-25.", false),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
				Name:        "settings",
				Description: "Manage guild memory settings.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "show",
						Description: "Show guild memory settings.",
					},
					{
						Type:        discordgo.ApplicationCommandOptionSubCommand,
						Name:        "set",
						Description: "Set channel memory mode.",
						Options: []*discordgo.ApplicationCommandOption{
							channelOption("channel", "Discord channel."),
							memoryModeOption(),
							integerOption("retention-days", "Retention days, 1-365. Omit to use default.", false),
						},
					},
				},
			},
		},
		Handle: memoryHandler(manager, recorder),
	}}
}

func memoryRequiredCapability(interaction Interaction) capability.Capability {
	group, action, ok := memoryPath(interaction)
	if !ok {
		return ""
	}
	switch {
	case group == "status":
		return capability.Capability("memory.read.guild")
	case group == "count":
		return capability.Capability("memory.read.guild")
	case group == "search":
		return capability.Capability("memory.read.guild")
	case group == "settings" && (action == "show" || action == "set"):
		return capability.Capability("memory.manage.guild")
	default:
		return ""
	}
}

func memoryPath(interaction Interaction) (string, string, bool) {
	if len(interaction.Options) != 1 {
		return "", "", false
	}
	first := interaction.Options[0]
	if first.Name == "status" {
		return "status", "", true
	}
	if first.Name == "count" {
		return "count", "", true
	}
	if first.Name == "search" {
		return "search", "", true
	}
	if len(first.Options) != 1 {
		return first.Name, "", false
	}
	return first.Name, first.Options[0].Name, true
}

type memoryRequest struct {
	Group         string
	Action        string
	ChannelID     string
	Mode          memory.Mode
	RetentionDays int
	UserID        string
	Text          string
	Scope         string
	Query         string
	Limit         int
}

func memoryHandler(manager MemoryManager, recorder AuditRecorder) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		if manager == nil {
			return CommandResponse{}, fmt.Errorf("memory manager is required")
		}
		request, err := parseMemoryRequest(interaction)
		if err != nil {
			_ = recordMemoryAction(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}

		response, err := executeMemoryRequest(ctx, manager, interaction, &request)
		if err != nil {
			_ = recordMemoryAction(ctx, recorder, interaction, request, audit.StatusFailed, err)
			return CommandResponse{Content: "Memory command failed.", Ephemeral: true}, nil
		}
		if shouldAuditMemoryAction(request) {
			if err := recordMemoryAction(ctx, recorder, interaction, request, audit.StatusSucceeded, nil); err != nil {
				return CommandResponse{}, err
			}
		}
		return CommandResponse{Content: response, Ephemeral: true}, nil
	}
}

func parseMemoryRequest(interaction Interaction) (memoryRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return memoryRequest{}, fmt.Errorf("Memory can only be managed inside a Discord server.")
	}
	if len(interaction.Options) != 1 {
		return memoryRequest{}, fmt.Errorf("Choose one memory action.")
	}
	first := interaction.Options[0]
	if first.Name == "status" {
		return memoryRequest{Group: "status"}, nil
	}
	if first.Name == "count" {
		return parseMemoryCountRequest(memoryRequest{Group: "count"}, first.Options)
	}
	if first.Name == "search" {
		return parseMemorySearchRequest(memoryRequest{Group: "search"}, first.Options)
	}
	if len(first.Options) != 1 {
		return memoryRequest{Group: first.Name}, fmt.Errorf("Choose one memory settings action.")
	}
	action := first.Options[0]
	request := memoryRequest{Group: first.Name, Action: action.Name}
	if request.Group != "settings" {
		return request, fmt.Errorf("Unsupported memory action.")
	}
	switch request.Action {
	case "show":
		return request, nil
	case "set":
		return parseMemorySettingsSet(request, action.Options)
	default:
		return request, fmt.Errorf("Unsupported memory settings action.")
	}
}

func parseMemorySearchRequest(request memoryRequest, options []InteractionOption) (memoryRequest, error) {
	request.Query = strings.TrimSpace(optionByName(options, "query"))
	if request.Query == "" {
		return request, fmt.Errorf("Query is required.")
	}
	request.UserID = optionByName(options, "user")
	limitValue := optionByName(options, "limit")
	if limitValue == "" {
		request.Limit = 5
		return request, nil
	}
	limit, err := strconv.Atoi(limitValue)
	if err != nil {
		return request, fmt.Errorf("Limit must be a whole number.")
	}
	if limit < 1 || limit > 25 {
		return request, fmt.Errorf("Limit must be between 1 and 25.")
	}
	request.Limit = limit
	return request, nil
}

func parseMemoryCountRequest(request memoryRequest, options []InteractionOption) (memoryRequest, error) {
	request.Text = strings.TrimSpace(optionByName(options, "text"))
	if request.Text == "" {
		return request, fmt.Errorf("Text is required.")
	}
	request.UserID = optionByName(options, "user")
	request.Scope = optionByName(options, "scope")
	if request.Scope == "" {
		request.Scope = "this-channel"
	}
	switch request.Scope {
	case "this-channel", "server":
		return request, nil
	default:
		return request, fmt.Errorf("Unsupported memory scope.")
	}
}

func parseMemorySettingsSet(request memoryRequest, options []InteractionOption) (memoryRequest, error) {
	request.ChannelID = optionByName(options, "channel")
	if request.ChannelID == "" {
		return request, fmt.Errorf("Channel is required.")
	}
	request.Mode = memory.Mode(optionByName(options, "mode"))
	if err := memory.ValidateMode(request.Mode); err != nil {
		return request, err
	}
	retentionValue := optionByName(options, "retention-days")
	if retentionValue == "" {
		return request, nil
	}
	retentionDays, err := strconv.Atoi(retentionValue)
	if err != nil {
		return request, fmt.Errorf("Retention days must be a whole number.")
	}
	if retentionDays < 1 || retentionDays > 365 {
		return request, fmt.Errorf("Retention days must be between 1 and 365.")
	}
	request.RetentionDays = retentionDays
	return request, nil
}

func executeMemoryRequest(ctx context.Context, manager MemoryManager, interaction Interaction, request *memoryRequest) (string, error) {
	switch {
	case request.Group == "status":
		status, err := manager.GuildStatus(ctx, interaction.GuildID)
		if err != nil {
			return "", err
		}
		return formatMemoryStatus(status), nil
	case request.Group == "count":
		channelID := interaction.ChannelID
		if request.Scope == "server" {
			return "Server-wide memory counts are not available yet. Use `this-channel` scope.", nil
		}
		result, err := manager.CountMentions(ctx, memory.CountRequest{
			GuildID:      interaction.GuildID,
			ChannelID:    channelID,
			AuthorUserID: request.UserID,
			Text:         request.Text,
		})
		if err != nil {
			return "", err
		}
		return formatMemoryCount(result, *request), nil
	case request.Group == "search":
		results, err := manager.SearchMessages(ctx, memory.SearchRequest{
			GuildID:      interaction.GuildID,
			ChannelID:    interaction.ChannelID,
			AuthorUserID: request.UserID,
			Query:        request.Query,
			Limit:        request.Limit,
		})
		if err != nil {
			return "", err
		}
		return formatMemorySearch(results, *request), nil
	case request.Group == "settings" && request.Action == "show":
		status, err := manager.GuildStatus(ctx, interaction.GuildID)
		if err != nil {
			return "", err
		}
		return formatMemoryStatus(status), nil
	case request.Group == "settings" && request.Action == "set":
		channel, err := manager.UpsertChannelPolicy(ctx, memory.UpsertChannelPolicyRequest{
			GuildID:       interaction.GuildID,
			ChannelID:     request.ChannelID,
			Mode:          request.Mode,
			RetentionDays: request.RetentionDays,
			ActorID:       interaction.UserID,
		})
		if err != nil {
			return "", err
		}
		return formatMemoryChannelSet(channel), nil
	default:
		return "", fmt.Errorf("unsupported memory action")
	}
}

func formatMemorySearch(results []memory.SearchResult, request memoryRequest) string {
	if len(results) == 0 {
		return "Memory search found no retained full-mode matches in this channel."
	}
	lines := []string{fmt.Sprintf("Memory search for `%s`:", safeInline(memory.NormalizeText(request.Query)))}
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("- <@%s>: %s", safeInline(result.AuthorUserID), safeInlineLimit(result.Text, 140)))
	}
	return strings.Join(lines, "\n")
}

func formatMemoryCount(result memory.CountResult, request memoryRequest) string {
	who := "Messages"
	if request.UserID != "" {
		who = fmt.Sprintf("<@%s>", safeInline(request.UserID))
	}
	return fmt.Sprintf("%s mentioned `%s` %d times in this channel.", who, safeInline(memory.NormalizeText(request.Text)), result.Count)
}

func formatMemoryStatus(status memory.Status) string {
	lines := []string{
		fmt.Sprintf("Guild memory: default `%s`, retention %d days, embeddings %s.",
			safeInline(string(status.Policy.RawStorageMode)),
			status.Policy.DefaultRetentionDays,
			formatEnabled(status.Policy.EmbeddingsEnabled),
		),
	}
	if len(status.Channels) == 0 {
		lines = append(lines, "Configured channels: none.")
		return strings.Join(lines, "\n")
	}
	limit := len(status.Channels)
	if limit > 10 {
		limit = 10
	}
	lines = append(lines, "Configured channels:")
	for _, channel := range status.Channels[:limit] {
		retention := "default"
		if channel.RetentionDays > 0 {
			retention = fmt.Sprintf("%d days", channel.RetentionDays)
		}
		lines = append(lines, fmt.Sprintf("- <#%s> - `%s` (retention: %s)", safeInline(channel.ChannelID), safeInline(string(channel.Mode)), retention))
	}
	if len(status.Channels) > limit {
		lines = append(lines, fmt.Sprintf("...and %d more.", len(status.Channels)-limit))
	}
	return strings.Join(lines, "\n")
}

func formatMemoryChannelSet(channel memory.ChannelPolicy) string {
	retention := "default"
	if channel.RetentionDays > 0 {
		retention = fmt.Sprintf("%d days", channel.RetentionDays)
	}
	return fmt.Sprintf("Set memory for <#%s> to `%s` (retention: %s).", safeInline(channel.ChannelID), safeInline(string(channel.Mode)), retention)
}

func formatEnabled(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func shouldAuditMemoryAction(request memoryRequest) bool {
	return request.Group == "settings" && request.Action == "set"
}

func recordMemoryAction(ctx context.Context, recorder AuditRecorder, interaction Interaction, request memoryRequest, status audit.Status, err error) error {
	if recorder == nil || strings.TrimSpace(interaction.UserID) == "" || !shouldAuditMemoryAction(request) {
		return nil
	}
	reason := ""
	if err != nil {
		reason = "memory_action_failed"
	}
	metadata := map[string]string{
		"command": interaction.Name,
		"group":   request.Group,
		"action":  request.Action,
	}
	if request.ChannelID != "" {
		metadata["channel_id"] = request.ChannelID
	}
	if request.Mode != "" {
		metadata["mode"] = string(request.Mode)
	}
	if request.RetentionDays > 0 {
		metadata["retention_days"] = strconv.Itoa(request.RetentionDays)
	}
	return recorder.Record(ctx, audit.Event{
		Kind:     "discord.memory.settings.change",
		GuildID:  interaction.GuildID,
		ActorID:  interaction.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}

func channelOption(name string, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionChannel,
		Name:        name,
		Description: description,
		Required:    true,
	}
}

func integerOption(name string, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionInteger,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func memoryModeOption() *discordgo.ApplicationCommandOption {
	return stringOption("mode", "Memory mode.", []*discordgo.ApplicationCommandOptionChoice{
		{Name: "off", Value: string(memory.ModeOff)},
		{Name: "metadata", Value: string(memory.ModeMetadata)},
		{Name: "full", Value: string(memory.ModeFull)},
	})
}

func memoryScopeOption() *discordgo.ApplicationCommandOption {
	return optionalStringOption("scope", "Memory scope.", []*discordgo.ApplicationCommandOptionChoice{
		{Name: "this-channel", Value: "this-channel"},
		{Name: "server", Value: "server"},
	})
}

func optionalStringOption(name string, description string, choices []*discordgo.ApplicationCommandOptionChoice) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        name,
		Description: description,
		Required:    false,
		Choices:     choices,
	}
}
