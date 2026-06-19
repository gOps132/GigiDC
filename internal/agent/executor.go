package agent

import (
	"context"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/storage"
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
			return Response{Text: "Agent tool failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationExecutorFailed}, nil
		}
		if canceled, err := traceRunCanceled(ctx, trace); err != nil {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "cancel_check_failed", map[string]string{"tool": safeAuditValue(spec.Name)})
			return Response{Text: "Agent run failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationExecutorFailed}, nil
		} else if canceled {
			return Response{Text: "Agent run was canceled.", RunStatus: RunStatusCanceled, TerminationReason: TerminationCanceled}, nil
		}
		if spec.Kind == ToolKindWrite {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusDenied, "confirmation_required", map[string]string{
				"tool":       safeAuditValue(spec.Name),
				"kind":       safeAuditValue(string(spec.Kind)),
				"capability": safeAuditValue(spec.Capability),
			})
			confirmationID, err := createPendingConfirmation(ctx, trace, request, index+1, spec, call)
			if err != nil {
				_ = trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "confirmation_create_failed", map[string]string{"tool": safeAuditValue(spec.Name)})
				return Response{Text: "Agent confirmation failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationExecutorFailed}, nil
			}
			return Response{Text: "I can plan that, but confirmation is required before running it.", ConfirmationID: confirmationID, RunStatus: RunStatusConfirmationRequired, TerminationReason: TerminationConfirmationRequired}, nil
		}
		decision, err := e.Policy.CheckBeforeTool(ctx, request, spec)
		if err != nil {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "permission_check_failed", map[string]string{
				"tool":       safeAuditValue(spec.Name),
				"kind":       safeAuditValue(string(spec.Kind)),
				"capability": safeAuditValue(spec.Capability),
			})
			return Response{Text: "Permission check failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationPermissionFailed}, nil
		}
		if !decision.Allowed {
			_ = trace.Record(ctx, request, "agent.tool", audit.StatusDenied, "permission_denied", map[string]string{
				"tool":            safeAuditValue(spec.Name),
				"kind":            safeAuditValue(string(spec.Kind)),
				"capability":      safeAuditValue(string(decision.Capability)),
				"decision_reason": safeAuditValue(string(decision.Reason)),
			})
			return Response{Text: "Permission denied for agent tool.", RunStatus: RunStatusDenied, TerminationReason: TerminationPermissionDenied}, nil
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
			return Response{Text: "Agent tool failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationExecutorFailed}, nil
		}
		results = append(results, result)
		_ = trace.Record(ctx, request, "agent.tool", audit.StatusSucceeded, "", metadata)
	}
	answerTrace := e.Trace.WithStep(len(plan.ToolCalls) + 1)
	if canceled, err := traceRunCanceled(ctx, answerTrace); err != nil {
		_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusFailed, "cancel_check_failed", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return Response{Text: "Agent run failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationExecutorFailed}, nil
	} else if canceled {
		return Response{Text: "Agent run was canceled.", RunStatus: RunStatusCanceled, TerminationReason: TerminationCanceled}, nil
	}
	if e.Answerer != nil {
		if e.SkipAnswerReason != "" {
			_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusFailed, e.SkipAnswerReason, map[string]string{"intent": safeAuditValue(plan.Intent)})
			response := Response{Text: formatToolResults(results), RunStatus: RunStatusFailed, TerminationReason: TerminationLLMBudgetExceeded}
			_ = e.saveFollowUp(ctx, request, plan, results, response)
			return response, nil
		}
		response, err := e.Answerer.Answer(ctx, request, plan, results)
		if err != nil {
			_ = answerTrace.Record(ctx, request, "agent.answer", audit.StatusFailed, "answer_failed", map[string]string{"intent": safeAuditValue(plan.Intent)})
			return Response{Text: "Agent answer failed.", RunStatus: RunStatusFailed, TerminationReason: TerminationExecutorFailed}, nil
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
	results, responseText := sanitizeFollowUpSnapshot(results, response.Text)
	snapshot := RunSnapshot{
		Intent:       plan.Intent,
		Results:      results,
		ResponseText: responseText,
		ContextState: request.ContextPack.NextState,
	}
	return e.FollowUps.Save(ctx, request, snapshot)
}

func sanitizeFollowUpSnapshot(results []ToolResult, responseText string) ([]ToolResult, string) {
	sanitized := make([]ToolResult, 0, len(results))
	redactedMemoryData := false
	for _, result := range results {
		copied := copyToolResult(result)
		if shouldRedactFollowUpToolData(copied.Name) && (copied.Data != nil || copied.Summary != "") {
			copied.Data = redactedFollowUpData(copied.Data)
			copied.Summary = redactedFollowUpSummary(copied)
			redactedMemoryData = true
		}
		sanitized = append(sanitized, copied)
	}
	if redactedMemoryData {
		return sanitized, ""
	}
	return sanitized, responseText
}

func redactedFollowUpData(data map[string]string) map[string]string {
	if len(data) == 0 {
		return nil
	}
	redacted := map[string]string{}
	for _, key := range []string{"count", "matches", "messages", "scope"} {
		if value := data[key]; value != "" {
			redacted[key] = value
		}
	}
	if len(redacted) == 0 {
		return nil
	}
	return redacted
}

func redactedFollowUpSummary(result ToolResult) string {
	switch result.Name {
	case ToolMemorySearch:
		return "Memory search results redacted from follow-up snapshot."
	case ToolMemoryRecent:
		return "Recent memory results redacted from follow-up snapshot."
	default:
		return result.Summary
	}
}

func shouldRedactFollowUpToolData(toolName string) bool {
	switch toolName {
	case ToolMemorySearch, ToolMemoryRecent:
		return true
	default:
		return false
	}
}

func traceRunCanceled(ctx context.Context, trace Trace) (bool, error) {
	if trace.Store == nil || trace.RunID == "" {
		return false, nil
	}
	return trace.Store.IsRunCanceled(ctx, trace.RunID)
}

func createPendingConfirmation(ctx context.Context, trace Trace, request Request, stepIndex int, spec ToolSpec, call ToolCall) (string, error) {
	if trace.Store == nil || trace.RunID == "" {
		return "", nil
	}
	confirmationID := storage.NewID("agentconfirm")
	payload := map[string]string{
		"tool": spec.Name,
	}
	for key, value := range call.Args {
		payload["arg."+key] = value
	}
	payload = audit.SanitizeMetadata(payload)
	if payload == nil {
		payload = map[string]string{}
	}
	return confirmationID, trace.Store.CreateConfirmation(ctx, ConfirmationRecord{
		ID:              confirmationID,
		RunID:           trace.RunID,
		StepIndex:       stepIndex,
		Status:          ConfirmationStatusPending,
		ToolName:        spec.Name,
		Payload:         payload,
		CreatedByUserID: request.ActorUserID,
	})
}
