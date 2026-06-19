package agent

import (
	"context"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/contextbroker"
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
	Planner         Planner
	Policy          RoutingPolicy
	Executor        Executor
	Trace           Trace
	Limits          Limits
	RunStore        RunStore
	NewRunID        func() string
	ContextProvider ContextSnippetProvider
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
	decision, err := r.Policy.CheckBeforePlan(ctx, request)
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, string(decision.Reason), capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan))
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationPermissionFailed, Response{Text: "Permission check failed."}, true, nil)
	}
	if !decision.Allowed {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusDenied, string(decision.Reason), capabilityMetadata(r.Policy.RequiredCapabilityBeforePlan))
		return r.completeRun(ctx, runID, runStarted, RunStatusDenied, TerminationPermissionDenied, Response{Text: "Permission denied for agent tools."}, true, nil)
	}
	request, err = r.withContextPack(ctx, request)
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "context_failed", nil)
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationExecutorFailed, Response{Text: "Agent context failed."}, true, nil)
	}
	plan, ok, err := r.Planner.Plan(ctx, request, executor.Tools.Specs())
	if err != nil {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "planner_failed", nil)
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationPlannerFailed, Response{Text: "Agent routing failed."}, true, nil)
	}
	if !ok {
		return r.completeRun(ctx, runID, runStarted, RunStatusSucceeded, TerminationIgnored, Response{}, false, nil)
	}
	if strings.TrimSpace(plan.ClarifyingQuestion) != "" {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "clarify", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return r.completeRun(ctx, runID, runStarted, RunStatusSucceeded, TerminationClarify, Response{Text: plan.ClarifyingQuestion}, true, nil)
	}
	if plan.RequiresConfirmation {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "confirmation_required", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return r.completeRun(ctx, runID, runStarted, RunStatusConfirmationRequired, TerminationConfirmationRequired, Response{Text: "I can plan that, but confirmation is required before running it."}, true, nil)
	}
	if mode == llmprovider.ToolRoutingDryRun {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusSucceeded, "dry_run", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return r.completeRun(ctx, runID, runStarted, RunStatusDryRun, TerminationDryRun, Response{Text: formatDryRunPlan(plan)}, true, nil)
	}
	if r.maxSteps() > 0 && plannedStepCount(plan) > r.maxSteps() {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "step_budget_exceeded", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationStepBudgetExceeded, Response{Text: "Agent step budget exceeded."}, true, nil)
	}
	if r.maxToolCalls() > 0 && len(plan.ToolCalls) > r.maxToolCalls() {
		_ = trace.Record(ctx, request, "agent.plan", audit.StatusFailed, "tool_budget_exceeded", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return r.completeRun(ctx, runID, runStarted, RunStatusFailed, TerminationToolBudgetExceeded, Response{Text: "Agent tool budget exceeded."}, true, nil)
	}
	response, err := executor.Execute(ctx, request, plan)
	status, reason := executionTermination(response, err)
	return r.completeRun(ctx, runID, runStarted, status, reason, response, true, err)
}

func (r Runner) withContextPack(ctx context.Context, request Request) (Request, error) {
	if request.ContextScope == "none" || len(request.ContextPack.Items) > 0 {
		return request, nil
	}
	if r.ContextProvider != nil && len(request.ContextSnippets) == 0 {
		snippets, err := r.ContextProvider.LoadContextSnippets(ctx, request)
		if err != nil {
			return request, err
		}
		request.ContextSnippets = snippets
	}
	if len(request.ContextSnippets) == 0 {
		return request, nil
	}
	var previous contextbroker.SessionState
	if request.PriorRun != nil {
		previous = request.PriorRun.ContextState
	}
	request.ContextPack = contextbroker.BuildPack(contextbroker.BuildRequest{
		Snippets: request.ContextSnippets,
		Previous: previous,
	})
	return request, nil
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
