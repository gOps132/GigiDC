package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/plugins"
)

const (
	ToolPluginsEnabled = "plugins.enabled"
	ToolPluginsPlan    = "plugins.plan"
)

type PluginRegistry interface {
	EnabledForGuild(ctx context.Context, guildID string) ([]plugins.Manifest, error)
}

type PluginPlanTool struct {
	Registry PluginRegistry
	Checker  CapabilityChecker
}

type PluginsEnabledTool struct {
	Registry PluginRegistry
}

func (t PluginsEnabledTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolPluginsEnabled,
		Description: "List enabled external Discord app manifests for this guild.",
		Kind:        ToolKindRead,
		Capability:  "plugin.install",
	}
}

func (t PluginsEnabledTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return ToolResult{}, fmt.Errorf("guild plugin context is required")
	}
	if t.Registry == nil {
		return ToolResult{}, fmt.Errorf("plugin registry is required")
	}
	manifests, err := t.Registry.EnabledForGuild(ctx, request.GuildID)
	if err != nil {
		return ToolResult{}, err
	}
	if len(manifests) == 0 {
		return ToolResult{
			Name:    ToolPluginsEnabled,
			Summary: "No external app plugins are enabled for this server.",
			Data: map[string]string{
				"count": "0",
			},
		}, nil
	}
	names := make([]string, 0, len(manifests))
	ids := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		if strings.TrimSpace(manifest.Name) != "" {
			names = append(names, safeInline(manifest.Name))
		}
		if strings.TrimSpace(manifest.ID) != "" {
			ids = append(ids, manifest.ID)
		}
	}
	return ToolResult{
		Name:    ToolPluginsEnabled,
		Summary: fmt.Sprintf("Enabled external app plugins: %s.", strings.Join(names, ", ")),
		Data: map[string]string{
			"count":      fmt.Sprintf("%d", len(manifests)),
			"plugin_ids": strings.Join(ids, ","),
		},
	}, nil
}

func (t PluginPlanTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolPluginsPlan,
		Description: "Plan which enabled external Discord app command would match text without dispatching it.",
		Kind:        ToolKindRead,
	}
}

func (t PluginPlanTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return ToolResult{}, fmt.Errorf("guild plugin context is required")
	}
	if t.Registry == nil {
		return ToolResult{}, fmt.Errorf("plugin registry is required")
	}
	text := strings.TrimSpace(call.Args["text"])
	if text == "" {
		text = request.Text
	}
	if text == "" {
		return ToolResult{}, fmt.Errorf("plugin plan text is required")
	}
	manifests, err := t.Registry.EnabledForGuild(ctx, request.GuildID)
	if err != nil {
		return ToolResult{}, err
	}
	plan, ok := plugins.PlanCommand(manifests, "guild_text", text)
	if !ok {
		return ToolResult{
			Name:    ToolPluginsPlan,
			Summary: "No enabled external app manifest matched that text.",
			Data: map[string]string{
				"matched": "false",
			},
		}, nil
	}
	decision := pluginPlanDecision{Allowed: true}
	if len(plan.RequiredCapabilities) > 0 {
		decision, err = t.authorize(ctx, request, plan.RequiredCapabilities)
		if err != nil {
			return ToolResult{}, err
		}
	}
	if !decision.Allowed {
		return ToolResult{
			Name:    ToolPluginsPlan,
			Summary: "Permission denied for external app action.",
			Data: map[string]string{
				"matched":               "true",
				"allowed":               "false",
				"required_capabilities": joinCapabilities(plan.RequiredCapabilities),
				"denied_capability":     string(decision.Capability),
				"decision_reason":       string(decision.Reason),
			},
		}, nil
	}
	return ToolResult{
		Name:    ToolPluginsPlan,
		Summary: fmt.Sprintf("Matched external app `%s`; planned command `%s`.", safeInline(plan.Manifest.Name), safeInline(plan.Command)),
		Data: map[string]string{
			"matched":               "true",
			"allowed":               "true",
			"plugin_id":             plan.Manifest.ID,
			"plugin_name":           plan.Manifest.Name,
			"plugin_version":        plan.Manifest.Version,
			"trigger":               plan.Trigger.Value,
			"command":               plan.Command,
			"arguments":             plan.Arguments,
			"dispatch":              string(plan.Manifest.Dispatch),
			"required_capabilities": joinCapabilities(plan.RequiredCapabilities),
		},
	}, nil
}

type pluginPlanDecision struct {
	Allowed    bool
	Capability capability.Capability
	Reason     capability.Reason
}

func (t PluginPlanTool) authorize(ctx context.Context, request Request, required []capability.Capability) (pluginPlanDecision, error) {
	if t.Checker == nil {
		return pluginPlanDecision{Allowed: false, Capability: required[0], Reason: capability.ReasonStoreError}, fmt.Errorf("capability checker is required")
	}
	for _, requiredCapability := range required {
		decision, err := t.Checker.Check(ctx, capability.Subject{
			GuildID:          request.GuildID,
			UserID:           request.ActorUserID,
			RoleIDs:          request.RoleIDs,
			HasAdministrator: request.HasAdministrator,
		}, requiredCapability)
		if err != nil {
			return pluginPlanDecision{Allowed: false, Capability: decision.Capability, Reason: decision.Reason}, err
		}
		if !decision.Allowed {
			return pluginPlanDecision{Allowed: false, Capability: decision.Capability, Reason: decision.Reason}, nil
		}
	}
	return pluginPlanDecision{Allowed: true}, nil
}

func joinCapabilities(values []capability.Capability) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = capability.Capability(strings.TrimSpace(string(value)))
		if value != "" {
			parts = append(parts, string(value))
		}
	}
	return strings.Join(parts, ",")
}
