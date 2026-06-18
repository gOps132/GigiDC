package agent

import (
	"context"
	"regexp"
	"strings"

	"github.com/gOps132/GigiDC/internal/capability"
)

type MultiPlanner []Planner

func (p MultiPlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	for _, planner := range p {
		if planner == nil {
			continue
		}
		plan, ok, err := planner.Plan(ctx, request, specs)
		if err != nil || ok {
			return plan, ok, err
		}
	}
	return Plan{}, false, nil
}

type HeuristicToolPlanner struct{}

func (p HeuristicToolPlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	available := availableTools(specs)
	text := strings.ToLower(strings.TrimSpace(request.Text))
	if text == "" {
		return Plan{}, false, nil
	}
	if available[ToolLLMUsageGuild] && mentionsAny(text, "llm usage", "token usage", "tokens used", "usage summary") {
		return Plan{Intent: ToolLLMUsageGuild, ToolCalls: []ToolCall{{Name: ToolLLMUsageGuild}}}, true, nil
	}
	if available[ToolPluginsEnabled] && mentionsAny(text, "enabled plugins", "plugins enabled", "what plugins", "which plugins") {
		return Plan{Intent: ToolPluginsEnabled, ToolCalls: []ToolCall{{Name: ToolPluginsEnabled}}}, true, nil
	}
	if available[ToolPluginsPlan] && mentionsAny(text, "plugin dry run", "plugin plan", "external app plan") {
		return Plan{Intent: ToolPluginsPlan, ToolCalls: []ToolCall{{Name: ToolPluginsPlan, Args: map[string]string{"text": pluginPlanText(request.Text)}}}}, true, nil
	}
	if available[ToolPermissionsCheck] {
		if required := mentionedCapability(text); required != "" {
			return Plan{Intent: ToolPermissionsCheck, ToolCalls: []ToolCall{{Name: ToolPermissionsCheck, Args: map[string]string{"capability": string(required)}}}}, true, nil
		}
	}
	if available[ToolMemoryRecent] && mentionsAny(text, "recent messages", "recent chat", "last messages", "summarize chat", "summarise chat") {
		return Plan{Intent: ToolMemoryRecent, ToolCalls: []ToolCall{{Name: ToolMemoryRecent, Args: map[string]string{"limit": "10"}}}}, true, nil
	}
	return Plan{}, false, nil
}

func availableTools(specs []ToolSpec) map[string]bool {
	available := make(map[string]bool, len(specs))
	for _, spec := range specs {
		spec = NormalizeToolSpec(spec)
		if spec.Name != "" {
			available[spec.Name] = true
		}
	}
	return available
}

func mentionsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func pluginPlanText(text string) string {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"plugin dry run", "plugin plan", "external app plan"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

var inlineCapabilityPattern = regexp.MustCompile("`([^`]+)`")

func mentionedCapability(text string) capability.Capability {
	for _, match := range inlineCapabilityPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		if normalized, err := capability.Normalize(match[1]); err == nil {
			return normalized
		}
	}
	for _, known := range capability.KnownCapabilities() {
		if strings.Contains(text, strings.ToLower(string(known))) {
			return known
		}
	}
	return ""
}
