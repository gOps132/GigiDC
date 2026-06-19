package discord

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
)

func TestAgentCommandsExposeStatsGuild(t *testing.T) {
	commands := AgentCommands(nil, nil, nil, AgentCommandConfig{StatsReader: &fakeAgentStatsReader{}})
	if len(commands) != 1 || commands[0].Name != "agent" {
		t.Fatalf("commands = %+v, want agent command", commands)
	}
	stats := findOption(commands[0].Options, "stats")
	if stats == nil {
		t.Fatalf("options = %+v, want stats group", commands[0].Options)
	}
	guild := findOption(stats.Options, "guild")
	if guild == nil {
		t.Fatalf("stats options = %+v, want guild command", stats.Options)
	}
	period := findOption(guild.Options, "period")
	if period == nil || !hasChoice(period, "24h") || !hasChoice(period, "7d") || !hasChoice(period, "30d") || !hasChoice(period, "all") {
		t.Fatalf("period option = %+v, want fixed period choices", period)
	}
}

func TestAgentStatsGuildChecksCapabilityQueriesPeriodAndFormatsAggregateSummary(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	reader := &fakeAgentStatsReader{summary: agent.AnalyticsSummary{
		TotalRuns: 9,
		StatusCounts: map[agent.RunStatus]int{
			agent.RunStatusSucceeded:            6,
			agent.RunStatusFailed:               2,
			agent.RunStatusCanceled:             1,
			agent.RunStatusConfirmationRequired: 3,
		},
		TerminationCounts: map[agent.TerminationReason]int{
			agent.TerminationReason("planner_failed"): 2,
		},
		Duration: agent.AnalyticsDurationSummary{
			AverageMS: 1200,
			P50MS:     900,
			P95MS:     4200,
			MaxMS:     9000,
		},
		StepsUsed:     19,
		ToolCallsUsed: 14,
		LLMCallsUsed:  5,
		TopTools: []agent.AnalyticsToolCount{{
			Name:      "memory.search",
			Total:     3,
			Succeeded: 2,
			Failed:    1,
		}},
	}}
	authorizer := &fakeAgentStatsAuthorizer{decision: capability.Decision{Allowed: true, Capability: agentAnalyticsCapability}}
	recorder := &fakeAuditRecorder{}
	handler := AgentCommands(nil, nil, recorder, AgentCommandConfig{
		StatsReader:     reader,
		StatsAuthorizer: authorizer,
		Clock:           func() time.Time { return now },
	})[0].Handle

	response, err := handler(context.Background(), agentStatsInteraction("7d"))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if !response.Ephemeral {
		t.Fatalf("response = %+v, want private stats", response)
	}
	if authorizer.required != agentAnalyticsCapability || authorizer.interaction.GuildID != "guild-id" || authorizer.interaction.UserID != "user-id" {
		t.Fatalf("authorizer = %+v, want stats capability check", authorizer)
	}
	if reader.query.GuildID != "guild-id" || !reader.query.Since.Equal(now.Add(-7*24*time.Hour)) || !reader.query.Until.Equal(now) || reader.query.Limit != 5 {
		t.Fatalf("query = %+v, want 7d guild analytics query", reader.query)
	}
	for _, want := range []string{
		"Agent stats for this server (last 7 days):",
		"runs: `9` total",
		"`6` succeeded",
		"`2` failed",
		"`1` canceled",
		"`3` confirmation_required",
		"steps: `19` total, `14` tool, `5` llm",
		"latency: avg `1.2s`, p50 `900ms`, p95 `4.2s`, max `9.0s`",
		"top reasons: `planner_failed` `2`",
		"top tools: `memory.search` `2` ok / `1` failed",
	} {
		if !strings.Contains(response.Content, want) {
			t.Fatalf("response = %q, missing %q", response.Content, want)
		}
	}
	for _, raw := range []string{"run-", "user-", "api_key"} {
		if strings.Contains(response.Content, raw) {
			t.Fatalf("response = %q, want no raw value %q", response.Content, raw)
		}
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.agent.stats" || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["period"] != "7d" {
		t.Fatalf("events = %+v, want succeeded stats audit", recorder.events)
	}
}

func TestAgentStatsGuildAllTimeDoesNotSetTimeBounds(t *testing.T) {
	reader := &fakeAgentStatsReader{summary: agent.AnalyticsSummary{TotalRuns: 1}}
	response, err := agentStatsHandler(reader, &fakeAgentStatsAuthorizer{decision: capability.Decision{Allowed: true, Capability: agentAnalyticsCapability}}, nil, nil)(context.Background(), agentStatsInteraction("all"))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if !reader.query.Since.IsZero() || !reader.query.Until.IsZero() {
		t.Fatalf("query = %+v, want no time bounds for all-time", reader.query)
	}
	if !strings.Contains(response.Content, "all time") || !response.Ephemeral {
		t.Fatalf("response = %+v, want all-time private summary", response)
	}
}

func TestAgentStatsGuildDefaultsTo7dAndHandlesNoData(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	reader := &fakeAgentStatsReader{}
	response, err := agentStatsHandler(reader, &fakeAgentStatsAuthorizer{decision: capability.Decision{Allowed: true, Capability: agentAnalyticsCapability}}, nil, func() time.Time { return now })(context.Background(), agentStatsInteraction(""))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if !reader.query.Since.Equal(now.Add(-7*24*time.Hour)) || !reader.query.Until.Equal(now) {
		t.Fatalf("query = %+v, want 7d default", reader.query)
	}
	if response.Content != "Agent stats for this server (last 7 days): no recorded agent runs." || !response.Ephemeral {
		t.Fatalf("response = %+v, want empty private summary", response)
	}
}

func TestAgentStatsGuildRejectsInvalidPeriod(t *testing.T) {
	response, err := agentStatsHandler(&fakeAgentStatsReader{}, &fakeAgentStatsAuthorizer{decision: capability.Decision{Allowed: true, Capability: agentAnalyticsCapability}}, nil, nil)(context.Background(), agentStatsInteraction("yesterday"))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if response.Content != "Choose period: `24h`, `7d`, `30d`, or `all`." || !response.Ephemeral {
		t.Fatalf("response = %+v, want period validation", response)
	}
}

func TestAgentStatsGuildReaderDisabled(t *testing.T) {
	response, err := agentStatsHandler(nil, nil, nil, nil)(context.Background(), agentStatsInteraction("24h"))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if response.Content != "Agent stats are not configured yet." || !response.Ephemeral {
		t.Fatalf("response = %+v, want disabled stats message", response)
	}
}

func TestAgentStatsGuildAuthorizerDenied(t *testing.T) {
	reader := &fakeAgentStatsReader{summary: agent.AnalyticsSummary{TotalRuns: 1}}
	response, err := agentStatsHandler(reader, &fakeAgentStatsAuthorizer{decision: capability.Decision{Allowed: false, Capability: agentAnalyticsCapability}}, nil, nil)(context.Background(), agentStatsInteraction("24h"))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if response.Content != "Permission denied." || !response.Ephemeral {
		t.Fatalf("response = %+v, want private permission denial", response)
	}
	if reader.query.GuildID != "" {
		t.Fatalf("query = %+v, want no stats read after denial", reader.query)
	}
}

func TestAgentStatsGuildAuthorizerMissing(t *testing.T) {
	reader := &fakeAgentStatsReader{summary: agent.AnalyticsSummary{TotalRuns: 1}}
	response, err := agentStatsHandler(reader, nil, nil, nil)(context.Background(), agentStatsInteraction("24h"))
	if err != nil {
		t.Fatalf("stats returned error: %v", err)
	}
	if response.Content != "Permission denied." || !response.Ephemeral {
		t.Fatalf("response = %+v, want private permission failure", response)
	}
	if reader.query.GuildID != "" {
		t.Fatalf("query = %+v, want no stats read without authorizer", reader.query)
	}
}

func agentStatsInteraction(period string) Interaction {
	options := []InteractionOption(nil)
	if period != "" {
		options = append(options, InteractionOption{Name: "period", Value: period})
	}
	return Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
		Name:      "agent",
		Options: []InteractionOption{{
			Name: "stats",
			Options: []InteractionOption{{
				Name:    "guild",
				Options: options,
			}},
		}},
	}
}

type fakeAgentStatsReader struct {
	query   agent.AnalyticsQuery
	summary agent.AnalyticsSummary
	err     error
}

func (r *fakeAgentStatsReader) AgentAnalytics(_ context.Context, query agent.AnalyticsQuery) (agent.AnalyticsSummary, error) {
	r.query = query
	if r.err != nil {
		return agent.AnalyticsSummary{}, r.err
	}
	if r.summary.StatusCounts == nil {
		r.summary.StatusCounts = map[agent.RunStatus]int{}
	}
	if r.summary.TerminationCounts == nil {
		r.summary.TerminationCounts = map[agent.TerminationReason]int{}
	}
	return r.summary, nil
}

type fakeAgentStatsAuthorizer struct {
	interaction Interaction
	required    capability.Capability
	decision    capability.Decision
	err         error
}

func (a *fakeAgentStatsAuthorizer) Check(_ context.Context, interaction Interaction, required capability.Capability) (capability.Decision, error) {
	a.interaction = interaction
	a.required = required
	if a.err != nil {
		return capability.Decision{}, a.err
	}
	if a.decision.Capability == "" {
		a.decision.Capability = required
	}
	return a.decision, nil
}
