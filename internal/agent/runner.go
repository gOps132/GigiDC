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
	RunStore       RunStore
	NewRunID       func() string
}

func (r Runner) Run(ctx context.Context, request Request) (Response, bool, error) {
	runID := r.newRunID()
	trace := r.Trace.WithRunID(runID)
	trace.Store = r.RunStore
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
	runStarted := false
	if r.RunStore != nil {
		if err := r.RunStore.StartRun(ctx, RunRecord{
			ID:              runID,
			GuildID:         request.GuildID,
			ChannelID:       request.ChannelID,
			ActorUserID:     request.ActorUserID,
			Surface:         request.Surface,
			ContextScope:    request.ContextScope,
			Status:          RunStatusRunning,
			MaxSteps:        r.maxSteps(),
			MaxToolCalls:    r.maxToolCalls(),
			MaxLLMCalls:     r.maxLLMCalls(),
			MaxInputTokens:  r.Limits.Budget.MaxInputTokens,
			MaxOutputTokens: r.Limits.Budget.MaxOutputTokens,
		}); err != nil {
			return Response{Text: "Agent run failed."}, true, nil
		}
		runStarted = true
		if canceled, err := r.RunStore.IsRunCanceled(ctx, runID); err != nil {
			return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationCanceled, Response{Text: "Agent run failed."}, true, nil)
		} else if canceled {
			return r.completeRun(ctx, runID, runStarted, RunStatusCanceled, TerminationCanceled, Response{Text: "Agent run was canceled."}, true, nil)
		}
	}
	mode, err := r.Policy.Mode(ctx, request.GuildID)
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "routing_policy_failed", nil)
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationRoutingPolicyFailed, Response{Text: "Agent routing failed."}, true, nil)
	}
	if mode == llmprovider.ToolRoutingOff {
		return r.completeRun(ctx, runID, runStarted, RunStatusSucceeded, TerminationRoutingOff, Response{}, false, nil)
	}
	planMetadata := map[string]string{"routing_mode": safeAuditValue(string(mode))}
	decision, err := r.Policy.CheckBeforePlan(ctx, request)
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, string(decision.Reason), mergeMetadata(planMetadata, capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan)))
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationPermissionFailed, Response{Text: "Permission check failed."}, true, nil)
	}
	if !decision.Allowed {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusDenied, string(decision.Reason), mergeMetadata(planMetadata, capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan)))
		return r.completeRun(ctx, runID, runStarted, RunStatusDenied, TerminationPermissionDenied, Response{Text: "Permission denied for agent tools."}, true, nil)
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
				return r.completeRun(ctx, runID, runStarted, RunStatusDenied, TerminationPermissionDenied, Response{Text: "Permission denied for agent context."}, true, nil)
			} else {
				_ = trace.WithStep(1).Record(ctx, request, "agent.context", audit.StatusFailed, "context_fetch_failed", metadata)
				return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationExecutorFailed, Response{Text: "Agent context fetch failed."}, true, nil)
			}
		} else {
			request.ContextPack = &pack
			executor.TraceStepOffset = 1
			_ = trace.WithStep(1).Record(ctx, request, "agent.context", audit.StatusSucceeded, "", metadata)
		}
	}
	plan, ok, err := r.Planner.Plan(ctx, request, executor.Tools.Specs())
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "planner_failed", planMetadata)
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationPlannerFailed, Response{Text: "Agent routing failed."}, true, nil)
	}
	if !ok {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "no_plan", mergeMetadata(planMetadata, map[string]string{"planner": "agent"}))
		return r.completeRun(ctx, runID, runStarted, RunStatusSucceeded, TerminationIgnored, Response{}, false, nil)
	}
	if strings.TrimSpace(plan.ClarifyingQuestion) != "" {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "clarify", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent)}))
		return r.completeRun(ctx, runID, runStarted, RunStatusSucceeded, TerminationClarify, Response{Text: plan.ClarifyingQuestion}, true, nil)
	}
	if plan.RequiresConfirmation {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "confirmation_required", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent)}))
		return r.completeRun(ctx, runID, runStarted, RunStatusConfirmationRequired, TerminationConfirmationRequired, Response{Text: "I can plan that, but confirmation is required before running it."}, true, nil)
	}
	if len(plan.ToolCalls) == 0 && request.PriorRun == nil && shouldFallThroughNoToolPlan(request, plan) {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "no_tools", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent), "planner": "agent"}))
		return r.completeRun(ctx, runID, runStarted, RunStatusSucceeded, TerminationIgnored, Response{}, false, nil)
	}
	if mode == llmprovider.ToolRoutingDryRun {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "dry_run", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent)}))
		return r.completeRun(ctx, runID, runStarted, RunStatusDryRun, TerminationDryRun, Response{Text: formatDryRunPlan(plan)}, true, nil)
	}
	if r.maxSteps() > 0 && plannedStepCount(plan)+executor.TraceStepOffset > r.maxSteps() {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "step_budget_exceeded", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent)}))
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationStepBudgetExceeded, Response{Text: "Agent step budget exceeded."}, true, nil)
	}
	if r.maxToolCalls() > 0 && len(plan.ToolCalls) > r.maxToolCalls() {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "tool_budget_exceeded", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent)}))
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationToolBudgetExceeded, Response{Text: "Agent tool budget exceeded."}, true, nil)
	}
	_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "", mergeMetadata(planMetadata, map[string]string{"intent": safeAuditValue(plan.Intent), "planner": "agent"}))
	response, err := executor.Execute(ctx, request, plan)
	status, reason := executionTermination(response, err)
	return r.completeRun(ctx, runID, runStarted, status, reason, response, true, err)
}

func (r Runner) maxToolCalls() int {
	if r.Limits.MaxToolCalls > 0 {
		return r.Limits.MaxToolCalls
	}
	return 5
}

func (r Runner) completeRun(ctx context.Context, runID string, started bool, status RunStatus, reason TerminationReason, response Response, handled bool, err error) (Response, bool, error) {
	if started && r.RunStore != nil {
		_ = r.RunStore.CompleteRun(ctx, runID, status, reason)
	}
	response.RunID = runID
	response.RunStatus = status
	response.TerminationReason = reason
	return response, handled, err
}

func executionTermination(response Response, err error) (RunStatus, TerminationReason) {
	if err != nil {
		return RunStatusFailed, TerminationExecutorFailed
	}
	if response.RunStatus != "" {
		reason := response.TerminationReason
		if reason == "" {
			reason = TerminationCompleted
		}
		return response.RunStatus, reason
	}
	return RunStatusSucceeded, TerminationCompleted
}

func shouldFallThroughNoToolPlan(request Request, plan Plan) bool {
	if request.ContextPack == nil {
		return true
	}
	return isOptionalContextScope(request.ContextScope) && strings.EqualFold(strings.TrimSpace(plan.Intent), "chat")
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

func mergeMetadata(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, values := range maps {
		for key, value := range values {
			if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
				merged[key] = value
			}
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
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
