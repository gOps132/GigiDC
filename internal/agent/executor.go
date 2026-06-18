package agent

import (
	"context"

	"github.com/gOps132/GigiDC/internal/audit"
)

type Executor struct {
	Tools    Registry
	Answerer Answerer
	Trace    Trace
}

func (e Executor) Execute(ctx context.Context, request Request, plan Plan) (Response, error) {
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		result, err := e.Tools.Execute(ctx, request, call)
		if err != nil {
			_ = e.Trace.Record(ctx, request, "agent.tool", audit.StatusFailed, "tool_failed", map[string]string{"tool": safeAuditValue(call.Name)})
			return Response{Text: "Agent tool failed."}, nil
		}
		results = append(results, result)
		_ = e.Trace.Record(ctx, request, "agent.tool", audit.StatusSucceeded, "", map[string]string{"tool": safeAuditValue(call.Name)})
	}
	if e.Answerer != nil {
		response, err := e.Answerer.Answer(ctx, request, plan, results)
		if err != nil {
			_ = e.Trace.Record(ctx, request, "agent.answer", audit.StatusFailed, "answer_failed", map[string]string{"intent": safeAuditValue(plan.Intent)})
			return Response{Text: "Agent answer failed."}, nil
		}
		_ = e.Trace.Record(ctx, request, "agent.answer", audit.StatusSucceeded, "", map[string]string{"intent": safeAuditValue(plan.Intent)})
		return response, nil
	}
	_ = e.Trace.Record(ctx, request, "agent.answer", audit.StatusSucceeded, "", map[string]string{"intent": safeAuditValue(plan.Intent)})
	return Response{Text: formatToolResults(results)}, nil
}
