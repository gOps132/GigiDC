package agent

import (
	"context"

	"github.com/gOps132/GigiDC/internal/audit"
)

type Executor struct {
	Tools            Registry
	Answerer         Answerer
	SkipAnswerReason string
	Policy           RoutingPolicy
	Trace            Trace
	FollowUps        FollowUpStore
}

func (e Executor) Execute(ctx context.Context, request Request, plan Plan) (Response, error) {
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for index, call := range plan.ToolCalls {
		trace := e.Trace.WithStep(index + 1)
		tool, spec, err := e.Tools.Lookup(call.Name)
		if err != nil {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "tool_failed", map[string]string{"tool": safeAuditValue(call.Name)})
			return Response{Text: "Agent tool failed."}, nil
		}
		if spec.Kind == ToolKindWrite {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusDenied, "confirmation_required", map[string]string{
				"tool":       safeAuditValue(spec.Name),
				"kind":       safeAuditValue(string(spec.Kind)),
				"capability": safeAuditValue(spec.Capability),
			})
			return Response{Text: "I can plan that, but confirmation is required before running it."}, nil
		}
		decision, err := e.Policy.CheckBeforeTool(ctx, request, spec)
		if err != nil {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "permission_check_failed", map[string]string{
				"tool":       safeAuditValue(spec.Name),
				"kind":       safeAuditValue(string(spec.Kind)),
				"capability": safeAuditValue(spec.Capability),
			})
			return Response{Text: "Permission check failed."}, nil
		}
		if !decision.Allowed {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusDenied, "permission_denied", map[string]string{
				"tool":            safeAuditValue(spec.Name),
				"kind":            safeAuditValue(string(spec.Kind)),
				"capability":      safeAuditValue(string(decision.Capability)),
				"decision_reason": safeAuditValue(string(decision.Reason)),
			})
			return Response{Text: "Permission denied for agent tool."}, nil
		}
		call.Name = spec.Name
		result, err := tool.Execute(ctx, request, call)
		metadata := map[string]string{
			"tool":       safeAuditValue(spec.Name),
			"kind":       safeAuditValue(string(spec.Kind)),
			"capability": safeAuditValue(spec.Capability),
		}
		if err != nil {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "tool_failed", metadata)
			return Response{Text: "Agent tool failed."}, nil
		}
		results = append(results, result)
		_ = trace.Record(ctx, request, "agent.tool", audit.StatusSucceeded, "", metadata)
	}
	answerTrace := e.Trace.WithStep(len(plan.ToolCalls) + 1)
	if e.Answerer != nil {
		if e.SkipAnswerReason != "" {
			_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusFailed, e.SkipAnswerReason, map[string]string{"intent": safeAuditValue(plan.Intent)})
			response := Response{Text: formatToolResults(results)}
			_ = e.saveFollowUp(ctx, request, plan, results, response)
			return response, nil
		}
		response, err := e.Answerer.Answer(ctx, request, plan, results)
		if err != nil {
			_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusFailed, "answer_failed", map[string]string{"intent": safeAuditValue(plan.Intent)})
			return Response{Text: "Agent answer failed."}, nil
		}
		_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusSucceeded, "", map[string]string{"intent": safeAuditValue(plan.Intent)})
		_ = e.saveFollowUp(ctx, request, plan, results, response)
		return response, nil
	}
	_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusSucceeded, "", map[string]string{"intent": safeAuditValue(plan.Intent)})
	response := Response{Text: formatToolResults(results)}
	_ = e.saveFollowUp(ctx, request, plan, results, response)
	return response, nil
}

func (e Executor) saveFollowUp(ctx context.Context, request Request, plan Plan, results []ToolResult, response Response) error {
	if e.FollowUps == nil {
		return nil
	}
	snapshot := RunSnapshot{
		Intent:       plan.Intent,
		Results:      append([]ToolResult(nil), results...),
		ResponseText: response.Text,
	}
	return e.FollowUps.Save(ctx, request, snapshot)
}
