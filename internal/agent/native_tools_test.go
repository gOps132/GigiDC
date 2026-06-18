package agent

import (
	"context"
	"strings"
	"testing"

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
