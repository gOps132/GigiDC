package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gOps132/GigiDC/internal/capability"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/plugins"
)

func TestPluginsEnabledToolListsGuildPlugins(t *testing.T) {
	tool := PluginsEnabledTool{Registry: fakePluginRegistry{manifests: []plugins.Manifest{testPluginManifest()}}}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{Name: ToolPluginsEnabled})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Name != ToolPluginsEnabled || result.Data["count"] != "1" || !strings.Contains(result.Summary, "Jockie Music") {
		t.Fatalf("result=%+v, want enabled plugin summary", result)
	}
}

func TestPluginPlanToolPlansEnabledManifest(t *testing.T) {
	tool := PluginPlanTool{
		Registry: fakePluginRegistry{manifests: []plugins.Manifest{testPluginManifest()}},
		Checker:  fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
	}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{
		Name: ToolPluginsPlan,
		Args: map[string]string{"text": "play never gonna give you up"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Data["matched"] != "true" || result.Data["command"] != "m!play never gonna give you up" || !strings.Contains(result.Summary, "planned command") {
		t.Fatalf("result=%+v, want command plan", result)
	}
	if result.Data["allowed"] != "true" {
		t.Fatalf("allowed = %q, want true", result.Data["allowed"])
	}
	if result.Data["required_capabilities"] != "music.play" {
		t.Fatalf("required capabilities = %q, want music.play", result.Data["required_capabilities"])
	}
}

func TestPluginPlanToolDeniesRestrictedManifest(t *testing.T) {
	tool := PluginPlanTool{
		Registry: fakePluginRegistry{manifests: []plugins.Manifest{testPluginManifest()}},
		Checker:  fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: false, Reason: capability.ReasonMissingCapability}},
	}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{
		Name: ToolPluginsPlan,
		Args: map[string]string{"text": "play never gonna give you up"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Data["allowed"] != "false" || result.Data["denied_capability"] != "music.play" || !strings.Contains(result.Summary, "Permission denied") {
		t.Fatalf("result=%+v, want denied plan", result)
	}
}

func TestPluginPlanToolReturnsNoMatch(t *testing.T) {
	tool := PluginPlanTool{Registry: fakePluginRegistry{manifests: []plugins.Manifest{testPluginManifest()}}}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{
		Name: ToolPluginsPlan,
		Args: map[string]string{"text": "weather tomorrow"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Data["matched"] != "false" || !strings.Contains(result.Summary, "No enabled external app") {
		t.Fatalf("result=%+v, want no match", result)
	}
}

func TestLLMUsageGuildToolSummarizesUsage(t *testing.T) {
	tool := LLMUsageGuildTool{Reporter: fakeUsageReporter{summary: llmprovider.UsageSummary{
		InputTokens:  10,
		OutputTokens: 5,
		TotalEvents:  2,
		FailedEvents: 1,
	}}}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{Name: ToolLLMUsageGuild})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Data["total_tokens"] != "15" || !strings.Contains(result.Summary, "1 failed") {
		t.Fatalf("result=%+v, want usage summary", result)
	}
}

func TestMemoryDateParsers(t *testing.T) {
	start, err := parseStartDate("2026-06-22")
	if err != nil {
		t.Fatalf("parseStartDate returned error: %v", err)
	}
	if want := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Fatalf("start = %v, want %v", start, want)
	}

	end, err := parseEndDate("2026-06-22")
	if err != nil {
		t.Fatalf("parseEndDate returned error: %v", err)
	}
	if want := time.Date(2026, 6, 22, 23, 59, 59, 0, time.UTC); !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}

	exact, err := parseEndDate("2026-06-22T10:00:00Z")
	if err != nil {
		t.Fatalf("parseEndDate RFC3339 returned error: %v", err)
	}
	if want := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC); !exact.Equal(want) {
		t.Fatalf("exact end = %v, want %v", exact, want)
	}

	if _, err := parseStartDate("06/22/2026"); err == nil {
		t.Fatalf("parseStartDate invalid format returned nil error")
	}
}

func TestPermissionsCheckToolReportsDecision(t *testing.T) {
	tool := PermissionsCheckTool{Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}}}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{
		Name: ToolPermissionsCheck,
		Args: map[string]string{"capability": "memory.read.guild"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Data["allowed"] != "true" || result.Data["capability"] != "memory.read.guild" {
		t.Fatalf("result=%+v, want allowed capability", result)
	}
}

func TestPlanningHandlerExecutesHeuristicUsageTool(t *testing.T) {
	request := agentTestRequest()
	request.Text = "how many LLM tokens used here?"
	handler := PlanningHandler{
		Planner: HeuristicToolPlanner{},
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingEnabled},
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
		Tools: NewRegistry(LLMUsageGuildTool{Reporter: fakeUsageReporter{summary: llmprovider.UsageSummary{
			InputTokens: 7,
			TotalEvents: 1,
		}}}),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || !strings.Contains(response.Text, "7 tokens") {
		t.Fatalf("response=%+v handled=%v, want usage response", response, handled)
	}
}

func TestPlanningHandlerExecutesHeuristicPluginPlan(t *testing.T) {
	request := agentTestRequest()
	request.Text = "plugin plan play never gonna give you up"
	handler := PlanningHandler{
		Planner: HeuristicToolPlanner{},
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingEnabled},
		Tools: NewRegistry(PluginPlanTool{
			Registry: fakePluginRegistry{manifests: []plugins.Manifest{testPluginManifest()}},
			Checker:  fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
		}),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || !strings.Contains(response.Text, "m!play never gonna give you up") {
		t.Fatalf("response=%+v handled=%v, want plugin plan response", response, handled)
	}
}

func testPluginManifest() plugins.Manifest {
	return plugins.Manifest{
		ID:       "jockie-music",
		Name:     "Jockie Music",
		Version:  "0.1.0",
		Surfaces: []string{"guild_text"},
		Triggers: []plugins.Trigger{{
			Kind:    "prefix",
			Value:   "m!play",
			Aliases: []string{"play"},
		}},
		Permissions: []string{"music.play"},
		Dispatch:    plugins.DispatchModeSendMessage,
	}
}

type fakePluginRegistry struct {
	manifests []plugins.Manifest
	err       error
}

func (r fakePluginRegistry) EnabledForGuild(ctx context.Context, guildID string) ([]plugins.Manifest, error) {
	return r.manifests, r.err
}

type fakeUsageReporter struct {
	summary llmprovider.UsageSummary
	err     error
}

func (r fakeUsageReporter) GuildUsageSummary(ctx context.Context, guildID string) (llmprovider.UsageSummary, error) {
	return r.summary, r.err
}
