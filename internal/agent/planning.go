package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

type PolicyManager interface {
	GuildPolicy(context.Context, string) (llmprovider.GuildPolicy, error)
}

type CapabilityChecker interface {
	Check(context.Context, capability.Subject, capability.Capability) (capability.Decision, error)
}

type AuditRecorder interface {
	Record(context.Context, audit.Event) error
}

type Answerer interface {
	Answer(context.Context, Request, Plan, []ToolResult) (Response, error)
}

type PlanningHandler struct {
	Planner                      Planner
	Tools                        Registry
	Answerer                     Answerer
	Policy                       PolicyManager
	Checker                      CapabilityChecker
	Recorder                     AuditRecorder
	RequiredCapabilityBeforePlan capability.Capability
}

func (h PlanningHandler) HandleAgentRequest(ctx context.Context, request Request) (Response, bool, error) {
	if h.Planner == nil || request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return Response{}, false, nil
	}
	if request.ContextScope == "none" {
		return Response{}, false, nil
	}
	mode, err := h.toolRoutingMode(ctx, request.GuildID)
	if err != nil {
		_ = h.record(ctx, request, "agent.plan", audit.StatusFailed, "routing_policy_failed", nil)
		return Response{Text: "Agent routing failed."}, true, nil
	}
	if mode == llmprovider.ToolRoutingOff {
		return Response{}, false, nil
	}
	if h.RequiredCapabilityBeforePlan != "" {
		decision, err := h.checkCapability(ctx, request, h.RequiredCapabilityBeforePlan)
		if err != nil {
			_ = h.record(ctx, request, "agent.plan", audit.StatusFailed, string(decision.Reason), map[string]string{"capability": string(h.RequiredCapabilityBeforePlan)})
			return Response{Text: "Permission check failed."}, true, nil
		}
		if !decision.Allowed {
			_ = h.record(ctx, request, "agent.plan", audit.StatusDenied, string(decision.Reason), map[string]string{"capability": string(h.RequiredCapabilityBeforePlan)})
			return Response{Text: "Permission denied for agent tools."}, true, nil
		}
	}
	plan, ok, err := h.Planner.Plan(ctx, request, h.Tools.Specs())
	if err != nil {
		_ = h.record(ctx, request, "agent.plan", audit.StatusFailed, "planner_failed", nil)
		return Response{Text: "Agent routing failed."}, true, nil
	}
	if !ok {
		return Response{}, false, nil
	}
	if strings.TrimSpace(plan.ClarifyingQuestion) != "" {
		_ = h.record(ctx, request, "agent.plan", audit.StatusSucceeded, "clarify", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: plan.ClarifyingQuestion}, true, nil
	}
	if plan.RequiresConfirmation {
		_ = h.record(ctx, request, "agent.plan", audit.StatusSucceeded, "confirmation_required", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "I can plan that, but confirmation is required before running it."}, true, nil
	}
	if mode == llmprovider.ToolRoutingDryRun {
		_ = h.record(ctx, request, "agent.plan", audit.StatusSucceeded, "dry_run", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: formatDryRunPlan(plan)}, true, nil
	}
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		result, err := h.Tools.Execute(ctx, request, call)
		if err != nil {
			_ = h.record(ctx, request, "agent.tool", audit.StatusFailed, "tool_failed", map[string]string{"tool": safeAuditValue(call.Name)})
			return Response{Text: "Agent tool failed."}, true, nil
		}
		results = append(results, result)
		_ = h.record(ctx, request, "agent.tool", audit.StatusSucceeded, "", map[string]string{"tool": safeAuditValue(call.Name)})
	}
	if h.Answerer != nil {
		response, err := h.Answerer.Answer(ctx, request, plan, results)
		if err != nil {
			_ = h.record(ctx, request, "agent.answer", audit.StatusFailed, "answer_failed", map[string]string{"intent": safeAuditValue(plan.Intent)})
			return Response{Text: "Agent answer failed."}, true, nil
		}
		_ = h.record(ctx, request, "agent.answer", audit.StatusSucceeded, "", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return response, true, nil
	}
	_ = h.record(ctx, request, "agent.answer", audit.StatusSucceeded, "", map[string]string{"intent": safeAuditValue(plan.Intent)})
	return Response{Text: formatToolResults(results)}, true, nil
}

func (h PlanningHandler) toolRoutingMode(ctx context.Context, guildID string) (llmprovider.ToolRoutingMode, error) {
	if h.Policy == nil {
		return llmprovider.ToolRoutingOff, nil
	}
	policy, err := h.Policy.GuildPolicy(ctx, guildID)
	if err != nil {
		return "", err
	}
	return policy.ToolRoutingMode, nil
}

func (h PlanningHandler) checkCapability(ctx context.Context, request Request, required capability.Capability) (capability.Decision, error) {
	if h.Checker == nil {
		return capability.Decision{Allowed: false, Capability: required, Reason: capability.ReasonStoreError}, fmt.Errorf("capability checker is required")
	}
	return h.Checker.Check(ctx, capability.Subject{
		GuildID:          request.GuildID,
		UserID:           request.ActorUserID,
		RoleIDs:          request.RoleIDs,
		HasAdministrator: request.HasAdministrator,
	}, required)
}

func (h PlanningHandler) record(ctx context.Context, request Request, kind string, status audit.Status, reason string, metadata map[string]string) error {
	if h.Recorder == nil || strings.TrimSpace(request.ActorUserID) == "" {
		return nil
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["source"] = "agent"
	return h.Recorder.Record(ctx, audit.Event{
		Kind:     kind,
		GuildID:  request.GuildID,
		ActorID:  request.ActorUserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}

func formatDryRunPlan(plan Plan) string {
	if len(plan.ToolCalls) == 0 {
		return "Planned agent response. LLM tool routing is in `dry-run` mode."
	}
	names := make([]string, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		names = append(names, "`"+strings.TrimSpace(call.Name)+"`")
	}
	return "Planned agent tools: " + strings.Join(names, ", ") + ". LLM tool routing is in `dry-run` mode."
}

func formatToolResults(results []ToolResult) string {
	if len(results) == 0 {
		return "No agent tool results."
	}
	lines := make([]string, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.Summary) != "" {
			lines = append(lines, strings.TrimSpace(result.Summary))
		}
	}
	if len(lines) == 0 {
		return "Agent tools completed."
	}
	return strings.Join(lines, "\n")
}

func safeAuditValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("\n", " ", "\r", " ", "`", "'").Replace(value)
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}
