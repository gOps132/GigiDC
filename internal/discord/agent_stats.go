package discord

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
)

const agentAnalyticsCapability capability.Capability = "agent.analytics"
const agentReplyLatencyManageCapability capability.Capability = "agent.reply_latency.manage"

type AgentCommandConfig struct {
	StatsReader       AgentStatsReader
	StatsAuthorizer   CommandAuthorizer
	Clock             func() time.Time
	ReplyLatencyStore GuildReplyLatencyStore
}

type AgentStatsReader interface {
	AgentAnalytics(context.Context, agent.AnalyticsQuery) (agent.AnalyticsSummary, error)
}

type agentStatsRequest struct {
	Period              string
	Since               time.Time
	Until               time.Time
	ReplyLatencySet     bool
	ReplyLatencyEnabled bool
}

func agentStatsOptions() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "stats",
		Description: "Inspect aggregate agent stats.",
		Options: []*discordgo.ApplicationCommandOption{{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "guild",
			Description: "Show private aggregate agent stats for this server.",
			Options: []*discordgo.ApplicationCommandOption{
				optionalStringOption("period", "Stats window.", []*discordgo.ApplicationCommandOptionChoice{
					{Name: "24h", Value: "24h"},
					{Name: "7d", Value: "7d"},
					{Name: "30d", Value: "30d"},
					{Name: "all", Value: "all"},
				}),
				optionalStringOption("reply-latency", "Append response time to guild replies.", []*discordgo.ApplicationCommandOptionChoice{
					{Name: "off", Value: "off"},
					{Name: "on", Value: "on"},
				}),
			},
		}},
	}
}

func agentStatsHandler(reader AgentStatsReader, authorizer CommandAuthorizer, recorder AuditRecorder, clock func() time.Time, replyLatencyStores ...GuildReplyLatencyStore) CommandHandler {
	if clock == nil {
		clock = time.Now
	}
	replyLatencyStore := defaultGuildReplyLatencyStore
	if len(replyLatencyStores) > 0 && replyLatencyStores[0] != nil {
		replyLatencyStore = replyLatencyStores[0]
	}
	return func(ctx context.Context, interaction Interaction) (CommandResponse, error) {
		request, err := parseAgentStatsRequest(interaction, clock)
		if err != nil {
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "invalid_request")
			return CommandResponse{Content: err.Error(), Ephemeral: true}, nil
		}
		if request.ReplyLatencySet {
			if authorizer == nil {
				_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusDenied, "authorizer_missing")
				return CommandResponse{Content: "Permission denied.", Ephemeral: true}, nil
			}
			decision, err := authorizer.Check(ctx, interaction, agentReplyLatencyManageCapability)
			if err != nil {
				_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "permission_check_failed")
				return CommandResponse{Content: "Permission check failed.", Ephemeral: true}, nil
			}
			if !decision.Allowed {
				_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusDenied, string(decision.Reason))
				return CommandResponse{Content: "Permission denied.", Ephemeral: true}, nil
			}
			if replyLatencyStore == nil {
				_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "reply_latency_store_missing")
				return CommandResponse{Content: "Agent reply latency setting is not configured.", Ephemeral: true}, nil
			}
			if err := replyLatencyStore.SetGuildReplyLatencyEnabled(ctx, interaction.GuildID, request.ReplyLatencyEnabled); err != nil {
				_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "reply_latency_set_failed")
				return CommandResponse{Content: "Agent reply latency setting failed.", Ephemeral: true}, nil
			}
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusSucceeded, "")
			return CommandResponse{Content: "Set guild reply latency display to `" + formatReplyLatencyMode(request.ReplyLatencyEnabled) + "`.", Ephemeral: true}, nil
		}
		if reader == nil {
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "reader_missing")
			return CommandResponse{Content: "Agent stats are not configured yet.", Ephemeral: true}, nil
		}
		if authorizer == nil {
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusDenied, "authorizer_missing")
			return CommandResponse{Content: "Permission denied.", Ephemeral: true}, nil
		}
		decision, err := authorizer.Check(ctx, interaction, agentAnalyticsCapability)
		if err != nil {
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "permission_check_failed")
			return CommandResponse{Content: "Permission check failed.", Ephemeral: true}, nil
		}
		if !decision.Allowed {
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusDenied, string(decision.Reason))
			return CommandResponse{Content: "Permission denied.", Ephemeral: true}, nil
		}
		summary, err := reader.AgentAnalytics(ctx, agent.AnalyticsQuery{
			GuildID: interaction.GuildID,
			Since:   request.Since,
			Until:   request.Until,
			Limit:   5,
		})
		if err != nil {
			_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusFailed, "lookup_failed")
			return CommandResponse{Content: "Agent stats lookup failed.", Ephemeral: true}, nil
		}
		_ = recordAgentStats(ctx, recorder, interaction, request, audit.StatusSucceeded, "")
		return CommandResponse{Content: formatAgentStatsSummary(summary, request.Period), Ephemeral: true}, nil
	}
}

func parseAgentStatsRequest(interaction Interaction, clock func() time.Time) (agentStatsRequest, error) {
	if strings.TrimSpace(interaction.GuildID) == "" {
		return agentStatsRequest{}, fmt.Errorf("Agent stats can only be shown inside a Discord server.")
	}
	if len(interaction.Options) != 1 || interaction.Options[0].Name != "stats" || len(interaction.Options[0].Options) != 1 || interaction.Options[0].Options[0].Name != "guild" {
		return agentStatsRequest{}, fmt.Errorf("Choose an agent stats action.")
	}
	options := interaction.Options[0].Options[0].Options
	period, err := parseAgentStatsPeriod(optionByName(options, "period"))
	if err != nil {
		return agentStatsRequest{}, err
	}
	replyLatencyEnabled, replyLatencySet, err := parseReplyLatencyPreference(optionByName(options, "reply-latency"))
	if err != nil {
		return agentStatsRequest{}, err
	}
	request := agentStatsRequest{
		Period:              period,
		Until:               clock().UTC(),
		ReplyLatencySet:     replyLatencySet,
		ReplyLatencyEnabled: replyLatencyEnabled,
	}
	switch period {
	case "24h":
		request.Since = request.Until.Add(-24 * time.Hour)
	case "7d":
		request.Since = request.Until.Add(-7 * 24 * time.Hour)
	case "30d":
		request.Since = request.Until.Add(-30 * 24 * time.Hour)
	case "all":
		request.Until = time.Time{}
	default:
		return agentStatsRequest{}, fmt.Errorf("Unsupported stats period.")
	}
	return request, nil
}

func parseAgentStatsPeriod(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "":
		return "7d", nil
	case "24h", "7d", "30d", "all":
		return strings.TrimSpace(value), nil
	default:
		return "", fmt.Errorf("Choose period: `24h`, `7d`, `30d`, or `all`.")
	}
}

func parseReplyLatencyPreference(value string) (bool, bool, error) {
	switch strings.TrimSpace(value) {
	case "":
		return false, false, nil
	case "off":
		return false, true, nil
	case "on":
		return true, true, nil
	default:
		return false, false, fmt.Errorf("Choose reply latency: `off` or `on`.")
	}
}

func formatReplyLatencyMode(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}

func formatAgentStatsSummary(summary agent.AnalyticsSummary, period string) string {
	header := "Agent stats for this server (" + formatAgentStatsPeriod(period) + "):"
	if summary.TotalRuns == 0 {
		return header + " no recorded agent runs."
	}
	lines := []string{
		header,
		fmt.Sprintf("runs: `%d` total%s", summary.TotalRuns, formatAgentStatusCounts(summary.StatusCounts)),
	}
	if summary.Duration.AverageMS > 0 || summary.Duration.P50MS > 0 || summary.Duration.P95MS > 0 || summary.Duration.MaxMS > 0 {
		lines = append(lines, "latency: avg `"+formatMillis(summary.Duration.AverageMS)+"`, p50 `"+formatMillis(summary.Duration.P50MS)+"`, p95 `"+formatMillis(summary.Duration.P95MS)+"`, max `"+formatMillis(summary.Duration.MaxMS)+"`")
	}
	lines = append(lines, fmt.Sprintf("steps: `%d` total, `%d` tool, `%d` llm", summary.StepsUsed, summary.ToolCallsUsed, summary.LLMCallsUsed))
	if topReasons := formatTopAgentTerminations(summary.TerminationCounts); topReasons != "" {
		lines = append(lines, "top reasons: "+topReasons)
	}
	if topTools := formatTopAgentTools(summary.TopTools); topTools != "" {
		lines = append(lines, "top tools: "+topTools)
	}
	return strings.Join(lines, "\n")
}

func formatAgentStatsPeriod(period string) string {
	switch period {
	case "24h":
		return "last 24 hours"
	case "7d":
		return "last 7 days"
	case "30d":
		return "last 30 days"
	case "all":
		return "all time"
	default:
		return "last 7 days"
	}
}

func formatAgentStatusCounts(counts map[agent.RunStatus]int) string {
	order := []agent.RunStatus{
		agent.RunStatusSucceeded,
		agent.RunStatusFailed,
		agent.RunStatusDenied,
		agent.RunStatusDryRun,
		agent.RunStatusConfirmationRequired,
		agent.RunStatusCanceled,
		agent.RunStatusRunning,
	}
	parts := []string{}
	for _, status := range order {
		if count := counts[status]; count > 0 {
			parts = append(parts, fmt.Sprintf("`%d` %s", count, safeInline(string(status))))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func formatTopAgentTerminations(counts map[agent.TerminationReason]int) string {
	items := make([]agentCountItem, 0, len(counts))
	for reason, count := range counts {
		if count > 0 && reason != "" && reason != agent.TerminationCompleted {
			items = append(items, agentCountItem{Name: string(reason), Count: count})
		}
	}
	sortAgentCountItems(items)
	if len(items) > 3 {
		items = items[:3]
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, "`"+safeInline(item.Name)+"` `"+strconv.Itoa(item.Count)+"`")
	}
	return strings.Join(parts, "; ")
}

func formatTopAgentTools(tools []agent.AnalyticsToolCount) string {
	parts := make([]string, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" || tool.Total <= 0 {
			continue
		}
		parts = append(parts, "`"+safeInline(tool.Name)+"` `"+strconv.FormatInt(tool.Succeeded, 10)+"` ok / `"+strconv.FormatInt(tool.Failed, 10)+"` failed")
	}
	return strings.Join(parts, "; ")
}

func formatMillis(value int64) string {
	if value <= 0 {
		return "0ms"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fs", float64(value)/1000)
	}
	return strconv.FormatInt(value, 10) + "ms"
}

type agentCountItem struct {
	Name  string
	Count int
}

func sortAgentCountItems(items []agentCountItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
}

func recordAgentStats(ctx context.Context, recorder AuditRecorder, interaction Interaction, request agentStatsRequest, status audit.Status, reason string) error {
	if recorder == nil {
		return nil
	}
	metadata := map[string]string{
		"scope":      "guild",
		"period":     safeInline(request.Period),
		"capability": string(agentStatsCapabilityFor(request)),
	}
	if request.ReplyLatencySet {
		metadata["reply_latency"] = formatReplyLatencyMode(request.ReplyLatencyEnabled)
	}
	return recorder.Record(ctx, audit.Event{
		Kind:     "discord.agent.stats",
		GuildID:  interaction.GuildID,
		ActorID:  interaction.UserID,
		Status:   status,
		Reason:   safeInline(reason),
		Metadata: metadata,
	})
}

func agentStatsCapabilityFor(request agentStatsRequest) capability.Capability {
	if request.ReplyLatencySet {
		return agentReplyLatencyManageCapability
	}
	return agentAnalyticsCapability
}
