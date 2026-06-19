package discord

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/agent"
)

type AgentTraceReader interface {
	LastTrace(context.Context, agent.TraceQuery) (agent.TraceRun, bool, error)
}

func AgentCommands(reader AgentTraceReader) []Command {
	return []Command{{
		Name:        "agent",
		Description: "Inspect Gigi agent runtime diagnostics.",
		Options: []*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
			Name:        "trace",
			Description: "Inspect safe agent traces.",
			Options: []*discordgo.ApplicationCommandOption{{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "last",
				Description: "Show your last agent run in this channel.",
				Options: []*discordgo.ApplicationCommandOption{
					optionalStringOption("visibility", "Response visibility.", []*discordgo.ApplicationCommandOptionChoice{
						{Name: "private", Value: "private"},
						{Name: "public", Value: "public"},
					}),
				},
			}},
		}},
		Handle: agentTraceHandler(reader),
	}}
}

func agentTraceHandler(reader AgentTraceReader) CommandHandler {
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		visibility := normalizeAgentTraceVisibility(agentTraceVisibility(interaction.Options))
		if reader == nil {
			return CommandResponse{Content: "Agent trace is not configured yet.", Ephemeral: visibility != "public"}, nil
		}
		group, action, ok := agentTracePath(interaction)
		if !ok || group != "trace" || action != "last" {
			return CommandResponse{Content: "Choose an agent trace action.", Ephemeral: true}, nil
		}
		run, ok, err := reader.LastTrace(ctx, agent.TraceQuery{
			GuildID:     interaction.GuildID,
			ChannelID:   interaction.ChannelID,
			ActorUserID: interaction.UserID,
		})
		if err != nil {
			return CommandResponse{Content: "Agent trace lookup failed.", Ephemeral: true}, nil
		}
		if !ok {
			return CommandResponse{Content: "No agent trace found for you in this channel.", Ephemeral: true}, nil
		}
		return CommandResponse{Content: formatAgentTrace(run), Ephemeral: visibility != "public"}, nil
	}
}

func agentTracePath(interaction Interaction) (string, string, bool) {
	if len(interaction.Options) != 1 {
		return "", "", false
	}
	group := interaction.Options[0]
	if len(group.Options) != 1 {
		return group.Name, "", false
	}
	return group.Name, group.Options[0].Name, true
}

func agentTraceVisibility(options []InteractionOption) string {
	if len(options) != 1 || len(options[0].Options) != 1 {
		return "private"
	}
	return optionByName(options[0].Options[0].Options, "visibility")
}

func normalizeAgentTraceVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "public":
		return "public"
	default:
		return "private"
	}
}

func formatAgentTrace(run agent.TraceRun) string {
	lines := []string{fmt.Sprintf("Last agent run `%s`:", safeInline(run.RunID))}
	if run.Status != "" {
		lines = append(lines, fmt.Sprintf("status: `%s`", safeInline(run.Status)))
	}
	for _, event := range run.Events {
		lines = append(lines, formatAgentTraceEvent(event))
		if len(lines) >= 12 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

func formatAgentTraceEvent(event agent.TraceEvent) string {
	parts := []string{"-"}
	if event.StepIndex > 0 {
		parts = append(parts, "`"+strconv.Itoa(event.StepIndex)+"`")
	}
	if event.Phase != "" {
		parts = append(parts, "`"+safeInline(event.Phase)+"`")
	}
	if event.Status != "" {
		parts = append(parts, safeInline(event.Status))
	}
	if event.Reason != "" {
		parts = append(parts, "reason=`"+safeInline(event.Reason)+"`")
	}
	if event.ToolName != "" {
		parts = append(parts, "tool=`"+safeInline(event.ToolName)+"`")
	}
	if event.ToolKind != "" {
		parts = append(parts, "kind=`"+safeInline(event.ToolKind)+"`")
	}
	if event.Capability != "" {
		parts = append(parts, "capability=`"+safeInline(event.Capability)+"`")
	}
	if event.RoutingMode != "" {
		parts = append(parts, "routing=`"+safeInline(event.RoutingMode)+"`")
	}
	return strings.Join(parts, " ")
}
