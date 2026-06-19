package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/storage"
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
	Planner        Planner
	ContextFetcher ContextFetcher
	Policy         RoutingPolicy
	Executor       Executor
	Trace          Trace
	Limits         Limits
	NewRunID       func() string
}

func (r Runner) Run(ctx context.Context, request Request) (Response, bool, error) {
	trace := r.Trace.WithRunID(r.newRunID())
	executor := r.Executor
	executor.Trace = executor.Trace.inherit(trace)
	if executor.Answerer != nil && r.maxLLMCalls() > 0 && r.maxLLMCalls() < 2 {
		executor.SkipAnswerReason = "llm_budget_exceeded"
	}
	if r.Planner == nil || request.Surface != SurfaceGuildMention || request.GuildID == "" {
		return Response{}, false, nil
	}
	if request.ContextScope == "none" {
		return Response{}, false, nil
	}
	mode, err := r.Policy.Mode(ctx, request.GuildID)
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "routing_policy_failed", nil)
		return Response{Text: "Agent routing failed."}, true, nil
	}
	if mode == llmprovider.ToolRoutingOff {
		return Response{}, false, nil
	}
	decision, err := r.Policy.CheckBeforePlan(ctx, request)
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, string(decision.Reason), capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan))
		return Response{Text: "Permission check failed."}, true, nil
	}
	if !decision.Allowed {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusDenied, string(decision.Reason), capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan))
		return Response{Text: "Permission denied for agent tools."}, true, nil
	}
	if isChannelContextScope(request.ContextScope) && request.ContextPack == nil && r.ContextFetcher != nil {
		pack, err := r.ContextFetcher.FetchContext(ctx, request)
		metadata := contextMetadata(pack)
		if err != nil {
			if isOptionalContextScope(request.ContextScope) {
				_ = trace.WithStep(1).Record(ctx, request, "agent.context", contextFailureStatus(err), contextFailureReason(err), metadata)
				executor.TraceStepOffset = 1
			} else if errors.Is(err, ErrContextPermissionDenied) {
				_ = trace.WithStep(1).Record(ctx, request, "agent.context", audit.StatusDenied, "permission_denied", metadata)
				return Response{Text: "Permission denied for agent context."}, true, nil
			} else {
				_ = trace.WithStep(1).Record(ctx, request, "agent.context", audit.StatusFailed, "context_fetch_failed", metadata)
				return Response{Text: "Agent context fetch failed."}, true, nil
			}
		} else {
			request.ContextPack = &pack
			executor.TraceStepOffset = 1
			_ = trace.WithStep(1).Record(ctx, request, "agent.context", audit.StatusSucceeded, "", metadata)
		}
	}
	plan, ok, err := r.Planner.Plan(ctx, request, executor.Tools.Specs())
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "planner_failed", nil)
		return Response{Text: "Agent routing failed."}, true, nil
	}
	if !ok {
		return Response{}, false, nil
	}
	if strings.TrimSpace(plan.ClarifyingQuestion) != "" {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "clarify", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: plan.ClarifyingQuestion}, true, nil
	}
	if plan.RequiresConfirmation {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "confirmation_required", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "I can plan that, but confirmation is required before running it."}, true, nil
	}
	if mode == llmprovider.ToolRoutingDryRun {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "dry_run", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: formatDryRunPlan(plan)}, true, nil
	}
	if r.maxSteps() > 0 && plannedStepCount(plan)+executor.TraceStepOffset > r.maxSteps() {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "step_budget_exceeded", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "Agent step budget exceeded."}, true, nil
	}
	if r.maxToolCalls() > 0 && len(plan.ToolCalls) > r.maxToolCalls() {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "tool_budget_exceeded", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "Agent tool budget exceeded."}, true, nil
	}
	response, err := executor.Execute(ctx, request, plan)
	return response, true, err
}

func (r Runner) maxToolCalls() int {
	if r.Limits.MaxToolCalls > 0 {
		return r.Limits.MaxToolCalls
	}
	return 5
}

func (r Runner) maxSteps() int {
	return r.Limits.MaxSteps
}

func (r Runner) maxLLMCalls() int {
	return r.Limits.Budget.MaxLLMCalls
}

func (r Runner) newRunID() string {
	if r.NewRunID != nil {
		return strings.TrimSpace(r.NewRunID())
	}
	return storage.NewID("agentrun")
}

func capabilityMetadata(required capability.Capability) map[string]string {
	if required == "" {
		return nil
	}
	return map[string]string{"capability": string(required)}
}

func plannedStepCount(plan Plan) int {
	return 2 + len(plan.ToolCalls)
}

func isChannelContextScope(scope string) bool {
	scope = strings.TrimSpace(scope)
	return scope == "channel" || scope == "channel-auto"
}

func isOptionalContextScope(scope string) bool {
	return strings.TrimSpace(scope) == "channel-auto"
}

func contextFailureStatus(err error) audit.Status {
	if errors.Is(err, ErrContextPermissionDenied) {
		return audit.StatusDenied
	}
	return audit.StatusFailed
}

func contextFailureReason(err error) string {
	if errors.Is(err, ErrContextPermissionDenied) {
		return "permission_denied"
	}
	return "context_fetch_failed"
}
