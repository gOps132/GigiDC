package agent

import (
	"context"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

type Budget struct {
	MaxInputTokens  int
	MaxOutputTokens int
	MaxLLMCalls     int
}

type Limits struct {
	MaxSteps     int
	MaxToolCalls int
	Budget       Budget
}

type Runner struct {
	Planner  Planner
	Policy   RoutingPolicy
	Executor Executor
	Trace    Trace
	Limits   Limits
}

func (r Runner) Run(ctx context.Context, request Request) (Response, bool, error) {
	if r.Planner == nil || request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return Response{}, false, nil
	}
	if request.ContextScope == "none" {
		return Response{}, false, nil
	}
	mode, err := r.Policy.Mode(ctx, request.GuildID)
	if err != nil {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "routing_policy_failed", nil)
		return Response{Text: "Agent routing failed."}, true, nil
	}
	if mode == llmprovider.ToolRoutingOff {
		return Response{}, false, nil
	}
	decision, err := r.Policy.CheckBeforePlan(ctx, request)
	if err != nil {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusFailed, string(decision.Reason), capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan))
		return Response{Text: "Permission check failed."}, true, nil
	}
	if !decision.Allowed {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusDenied, string(decision.Reason), capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan))
		return Response{Text: "Permission denied for agent tools."}, true, nil
	}
	plan, ok, err := r.Planner.Plan(ctx, request, r.Executor.Tools.Specs())
	if err != nil {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "planner_failed", nil)
		return Response{Text: "Agent routing failed."}, true, nil
	}
	if !ok {
		return Response{}, false, nil
	}
	if strings.TrimSpace(plan.ClarifyingQuestion) != "" {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "clarify", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: plan.ClarifyingQuestion}, true, nil
	}
	if plan.RequiresConfirmation {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "confirmation_required", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "I can plan that, but confirmation is required before running it."}, true, nil
	}
	if mode == llmprovider.ToolRoutingDryRun {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "dry_run", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: formatDryRunPlan(plan)}, true, nil
	}
	if r.maxToolCalls() > 0 && len(plan.ToolCalls) > r.maxToolCalls() {
		_ = r.Trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "tool_budget_exceeded", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "Agent tool budget exceeded."}, true, nil
	}
	response, err := r.Executor.Execute(ctx, request, plan)
	return response, true, err
}

func (r Runner) maxToolCalls() int {
	if r.Limits.MaxToolCalls > 0 {
		return r.Limits.MaxToolCalls
	}
	return 5
}

func capabilityMetadata(required capability.Capability) map[string]string {
	if required == "" {
		return nil
	}
	return map[string]string{"capability": string(required)}
}
