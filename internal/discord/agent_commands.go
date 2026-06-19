package discord

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/audit"
)

type AgentRunManager interface {
	LoadRun(context.Context, string) (agent.RunRecord, bool, error)
	RequestCancelRun(context.Context, string, string) error
	PendingConfirmation(context.Context, string) (agent.ConfirmationRecord, bool, error)
	ResolveConfirmation(context.Context, string, agent.ConfirmationStatus, string) error
}

func AgentCommands(manager AgentRunManager, recorder AuditRecorder) []Command {
	return []Command{{
		Name:        "agent",
		Description: "Manage durable Gigi agent runs.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "cancel",
				Description: "Request cancellation for one of your running agent runs.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("run", "Agent run id.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "pending",
				Description: "Show a pending confirmation for one of your agent runs.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("run", "Agent run id.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "confirm",
				Description: "Confirm a pending action for one of your agent runs.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("run", "Agent run id.", nil),
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "reject",
				Description: "Reject a pending action for one of your agent runs.",
				Options: []*discordgo.ApplicationCommandOption{
					stringOption("run", "Agent run id.", nil),
				},
			},
		},
		Handle: agentCommandHandler(manager, recorder),
	}}
}

type agentCommandRequest struct {
	Action       string
	RunID        string
	Confirmation agent.ConfirmationRecord
}

func agentCommandHandler(manager AgentRunManager, recorder AuditRecorder) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		request, err := parseAgentCommandRequest(interaction)
		if err != nil {
			_ = recordAgentCommand(ctx, recorder, interaction, request, audit.StatusFailed, "invalid_request")
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}
		if manager == nil {
			_ = recordAgentCommand(ctx, recorder, interaction, request, audit.StatusFailed, "manager_missing")
			return CommandResponse{Content: "Agent run manager is unavailable.", Ephemeral: true}, nil
		}
		response, err := executeAgentCommand(ctx, manager, interaction, &request)
		if err != nil {
			status := audit.StatusFailed
			reason := "agent_command_failed"
			if errors.Is(err, errAgentRunForbidden) {
				status = audit.StatusDenied
				reason = "agent_run_forbidden"
			}
			_ = recordAgentCommand(ctx, recorder, interaction, request, status, reason)
			return CommandResponse{Content: cleanAgentCommandError(err), Ephemeral: true}, nil
		}
		_ = recordAgentCommand(ctx, recorder, interaction, request, audit.StatusSucceeded, "")
		return CommandResponse{Content: response, Ephemeral: true}, nil
	}
}

func parseAgentCommandRequest(interaction Interaction) (agentCommandRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return agentCommandRequest{}, fmt.Errorf("Agent runs can only be managed inside a Discord server.")
	}
	if len(interaction.Options) != 1 {
		return agentCommandRequest{}, fmt.Errorf("Choose one agent action.")
	}
	action := interaction.Options[0]
	request := agentCommandRequest{
		Action: strings.TrimSpace(action.Name),
		RunID:  strings.TrimSpace(optionByName(action.Options, "run")),
	}
	switch request.Action {
	case "cancel", "pending", "confirm", "reject":
		if request.RunID == "" {
			return request, fmt.Errorf("Agent run id is required.")
		}
		return request, nil
	default:
		return request, fmt.Errorf("Unsupported agent action.")
	}
}

var errAgentRunForbidden = errors.New("agent run forbidden")

func executeAgentCommand(ctx context.Context, manager AgentRunManager, interaction Interaction, request *agentCommandRequest) (string, error) {
	run, ok, err := manager.LoadRun(ctx, request.RunID)
	if err != nil {
		return "", err
	}
	if !ok || strings.TrimSpace(run.GuildID) != strings.TrimSpace(interaction.GuildID) {
		return "", sql.ErrNoRows
	}
	if !canManageAgentRun(interaction, run.ActorUserID) {
		return "", errAgentRunForbidden
	}
	switch request.Action {
	case "cancel":
		if run.Status != agent.RunStatusRunning {
			return fmt.Sprintf("Agent run `%s` is already `%s`.", safeInline(request.RunID), safeInline(string(run.Status))), nil
		}
		if err := manager.RequestCancelRun(ctx, request.RunID, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Cancellation requested for agent run `%s`.", safeInline(request.RunID)), nil
	case "pending":
		confirmation, ok, err := manager.PendingConfirmation(ctx, request.RunID)
		if err != nil {
			return "", err
		}
		if !ok {
			return fmt.Sprintf("No pending confirmation for agent run `%s`.", safeInline(request.RunID)), nil
		}
		if !canManageAgentRun(interaction, confirmationOwner(confirmation, run)) {
			return "", errAgentRunForbidden
		}
		request.Confirmation = confirmation
		return formatAgentConfirmation(request.RunID, confirmation), nil
	case "confirm", "reject":
		confirmation, ok, err := manager.PendingConfirmation(ctx, request.RunID)
		if err != nil {
			return "", err
		}
		if !ok {
			return fmt.Sprintf("No pending confirmation for agent run `%s`.", safeInline(request.RunID)), nil
		}
		if !canManageAgentRun(interaction, confirmationOwner(confirmation, run)) {
			return "", errAgentRunForbidden
		}
		request.Confirmation = confirmation
		status := agent.ConfirmationStatusConfirmed
		verb := "Confirmed"
		if request.Action == "reject" {
			status = agent.ConfirmationStatusCanceled
			verb = "Rejected"
		}
		if err := manager.ResolveConfirmation(ctx, confirmation.ID, status, interaction.UserID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s pending agent action `%s` for run `%s`.", verb, safeInline(confirmation.ToolName), safeInline(request.RunID)), nil
	default:
		return "", fmt.Errorf("unsupported agent action")
	}
}

func canManageAgentRun(interaction Interaction, ownerID string) bool {
	ownerID = strings.TrimSpace(ownerID)
	return interaction.HasAdministrator || (ownerID != "" && strings.TrimSpace(interaction.UserID) == ownerID)
}

func confirmationOwner(confirmation agent.ConfirmationRecord, run agent.RunRecord) string {
	if strings.TrimSpace(confirmation.CreatedByUserID) != "" {
		return confirmation.CreatedByUserID
	}
	return run.ActorUserID
}

func formatAgentConfirmation(runID string, confirmation agent.ConfirmationRecord) string {
	lines := []string{
		fmt.Sprintf("Pending confirmation for agent run `%s`.", safeInline(runID)),
		fmt.Sprintf("Tool: `%s`.", safeInline(confirmation.ToolName)),
		fmt.Sprintf("Step: `%d`.", confirmation.StepIndex),
		fmt.Sprintf("Confirmation: `%s`.", safeInline(confirmation.ID)),
	}
	if payload := formatAgentConfirmationPayload(confirmation.Payload); payload != "" {
		lines = append(lines, "Payload: "+payload)
	}
	lines = append(lines, fmt.Sprintf("Use `/agent confirm run:%s` or `/agent reject run:%s`.", safeInline(runID), safeInline(runID)))
	return strings.Join(lines, "\n")
}

func formatAgentConfirmationPayload(payload map[string]string) string {
	payload = audit.SanitizeMetadata(payload)
	if len(payload) == 0 {
		return ""
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 5 {
		keys = keys[:5]
	}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("`%s=%s`", safeInlineLimit(key, 32), safeInlineLimit(payload[key], 80)))
	}
	return strings.Join(parts, ", ")
}

func cleanAgentCommandError(err error) string {
	switch {
	case err == nil:
		return "Agent command failed."
	case errors.Is(err, sql.ErrNoRows):
		return "Agent run is not active or was not found."
	case errors.Is(err, errAgentRunForbidden):
		return "Permission denied for that agent run."
	default:
		return "Agent command failed."
	}
}

func recordAgentCommand(ctx context.Context, recorder AuditRecorder, interaction Interaction, request agentCommandRequest, status audit.Status, reason string) error {
	if recorder == nil || strings.TrimSpace(interaction.UserID) == "" {
		return nil
	}
	metadata := map[string]string{
		"command": interaction.Name,
		"action":  request.Action,
	}
	if request.RunID != "" {
		metadata["run_id"] = request.RunID
	}
	if request.Confirmation.ID != "" {
		metadata["confirmation_id"] = request.Confirmation.ID
		metadata["tool"] = request.Confirmation.ToolName
		metadata["step_index"] = strconv.Itoa(request.Confirmation.StepIndex)
	}
	return recorder.Record(ctx, audit.Event{
		Kind:     "discord.agent." + safeAuditAction(request.Action),
		GuildID:  interaction.GuildID,
		ActorID:  interaction.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}

func safeAuditAction(action string) string {
	action = strings.TrimSpace(action)
	switch action {
	case "cancel", "pending", "confirm", "reject":
		return action
	default:
		return "unknown"
	}
}
