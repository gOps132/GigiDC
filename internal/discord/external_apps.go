package discord

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gOps132/GigiDC/internal/assistant"
	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/plugins"
)

type externalAppDryRunHandler struct {
	registry plugins.Registry
	checker  CapabilityChecker
	recorder AuditRecorder
	fallback MessageHandler
	semantic assistant.SemanticPluginPlanner
}

func ExternalAppDryRunHandler(registry plugins.Registry, checker CapabilityChecker, recorder AuditRecorder, fallback MessageHandler) MessageHandler {
	return ExternalAppDryRunHandlerWithSemantic(registry, checker, recorder, fallback, assistant.SemanticPluginPlanner{})
}

func ExternalAppDryRunHandlerWithSemantic(registry plugins.Registry, checker CapabilityChecker, recorder AuditRecorder, fallback MessageHandler, semantic assistant.SemanticPluginPlanner) MessageHandler {
	if fallback == nil {
		fallback = CoreMessageHandler()
	}
	return externalAppDryRunHandler{
		registry: registry,
		checker:  checker,
		recorder: recorder,
		fallback: fallback,
		semantic: semantic,
	}
}

func (h externalAppDryRunHandler) HandleMessage(ctx context.Context, message Message) (MessageResponse, error) {
	if message.Surface != MessageSurfaceGuildMention || strings.TrimSpace(message.GuildID) == "" || h.registry == nil {
		return h.fallback.HandleMessage(ctx, message)
	}

	manifests, err := h.registry.EnabledForGuild(ctx, message.GuildID)
	if err != nil {
		_ = h.record(ctx, message, plugins.CommandPlan{}, audit.StatusFailed, "registry_error", "")
		return MessageResponse{Content: "External app lookup failed."}, nil
	}
	plan, ok := plugins.PlanCommand(manifests, "guild_text", message.Text)
	if !ok {
		if response, routed, err := h.handleSemantic(ctx, message, manifests); routed || err != nil {
			return response, err
		}
		return h.fallback.HandleMessage(ctx, message)
	}

	decision, err := h.authorize(ctx, message, plan)
	if err != nil {
		_ = h.record(ctx, message, plan, audit.StatusFailed, string(decision.Reason), string(decision.Capability))
		return MessageResponse{Content: "Permission check failed."}, nil
	}
	if !decision.Allowed {
		_ = h.record(ctx, message, plan, audit.StatusDenied, string(decision.Reason), string(decision.Capability))
		return MessageResponse{Content: "Permission denied for external app action."}, nil
	}
	if shouldDispatch(plan) {
		if err := h.recordKind(ctx, "discord.external_app.dispatch", message, plan, audit.StatusSucceeded, "", ""); err != nil {
			return MessageResponse{}, err
		}
		return MessageResponse{Content: plan.Command}, nil
	}
	if err := h.record(ctx, message, plan, audit.StatusSucceeded, "", capabilityList(plan.RequiredCapabilities)); err != nil {
		return MessageResponse{}, err
	}
	return MessageResponse{Content: formatDryRunPlan(plan)}, nil
}

func (h externalAppDryRunHandler) handleSemantic(ctx context.Context, message Message, manifests []plugins.Manifest) (MessageResponse, bool, error) {
	if h.semantic.Runtime == nil {
		return MessageResponse{}, false, nil
	}
	plan, ok, err := h.semantic.Plan(ctx, assistant.SemanticPluginInput{
		GuildID:     message.GuildID,
		ChannelID:   message.ChannelID,
		ActorUserID: message.UserID,
		Text:        message.Text,
		Manifests:   manifests,
	})
	if err != nil {
		_ = h.recordKind(ctx, "discord.external_app.semantic_dry_run", message, plugins.CommandPlan{}, audit.StatusFailed, "semantic_routing_failed", "")
		return MessageResponse{Content: "External app semantic routing failed."}, true, nil
	}
	if !ok {
		return MessageResponse{}, false, nil
	}
	decision, err := h.authorize(ctx, message, plan)
	if err != nil {
		_ = h.recordKind(ctx, "discord.external_app.semantic_dry_run", message, plan, audit.StatusFailed, string(decision.Reason), string(decision.Capability))
		return MessageResponse{Content: "Permission check failed."}, true, nil
	}
	if !decision.Allowed {
		_ = h.recordKind(ctx, "discord.external_app.semantic_dry_run", message, plan, audit.StatusDenied, string(decision.Reason), string(decision.Capability))
		return MessageResponse{Content: "Permission denied for external app action."}, true, nil
	}
	if err := h.recordKind(ctx, "discord.external_app.semantic_dry_run", message, plan, audit.StatusSucceeded, "", capabilityList(plan.RequiredCapabilities)); err != nil {
		return MessageResponse{}, true, err
	}
	return MessageResponse{Content: formatDryRunPlan(plan)}, true, nil
}

func (h externalAppDryRunHandler) authorize(ctx context.Context, message Message, plan plugins.CommandPlan) (capability.Decision, error) {
	if len(plan.RequiredCapabilities) == 0 {
		return capability.Decision{Allowed: true, Reason: capability.ReasonPublicAction}, nil
	}
	if h.checker == nil {
		return capability.Decision{Allowed: false, Reason: capability.ReasonStoreError}, fmt.Errorf("capability checker is required")
	}
	subject := capability.Subject{
		GuildID:          message.GuildID,
		UserID:           message.UserID,
		RoleIDs:          message.RoleIDs,
		HasAdministrator: message.HasAdministrator,
	}
	var last capability.Decision
	for _, required := range plan.RequiredCapabilities {
		decision, err := h.checker.Check(ctx, subject, required)
		last = decision
		if err != nil || !decision.Allowed {
			return decision, err
		}
	}
	return last, nil
}

func (h externalAppDryRunHandler) record(ctx context.Context, message Message, plan plugins.CommandPlan, status audit.Status, reason string, capabilityValue string) error {
	return h.recordKind(ctx, "discord.external_app.dry_run", message, plan, status, reason, capabilityValue)
}

func (h externalAppDryRunHandler) recordKind(ctx context.Context, kind string, message Message, plan plugins.CommandPlan, status audit.Status, reason string, capabilityValue string) error {
	if h.recorder == nil || strings.TrimSpace(message.UserID) == "" {
		return nil
	}
	metadata := map[string]string{}
	if plan.Manifest.ID != "" {
		metadata["plugin_id"] = plan.Manifest.ID
		metadata["version"] = plan.Manifest.Version
		metadata["trigger_kind"] = plan.Trigger.Kind
		metadata["trigger"] = plan.Trigger.Value
	}
	if capabilityValue != "" {
		metadata["capability"] = capabilityValue
	}
	return h.recorder.Record(ctx, audit.Event{
		Kind:     kind,
		GuildID:  message.GuildID,
		ActorID:  message.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}

func shouldDispatch(plan plugins.CommandPlan) bool {
	return plan.Manifest.Dispatch == plugins.DispatchModeSendMessage && len(plan.RequiredCapabilities) == 0
}

func formatDryRunPlan(plan plugins.CommandPlan) string {
	return fmt.Sprintf("Matched external app: `%s`.\nPlanned command: `%s`.\nDry-run only; no command sent.",
		safeInlineLimit(plan.Manifest.Name, 80),
		safeInlineLimit(plan.Command, 180),
	)
}

func safeInlineLimit(value string, limit int) string {
	value = safeInline(value)
	if limit <= 0 {
		return value
	}
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit]) + "..."
}

func capabilityList(capabilities []capability.Capability) string {
	values := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		values = append(values, string(capability))
	}
	return strings.Join(values, ",")
}
