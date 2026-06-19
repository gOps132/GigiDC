package agent

import (
	"context"
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
	ContextFetcher               ContextFetcher
	Policy                       PolicyManager
	Checker                      CapabilityChecker
	Recorder                     AuditRecorder
	RequiredCapabilityBeforePlan capability.Capability
	Limits                       Limits
	NewRunID                     func() string
	FollowUps                    FollowUpStore
	TraceSink                    TraceSink
}

func (h PlanningHandler) HandleAgentRequest(ctx context.Context, request Request) (Response, bool, error) {
	if h.FollowUps != nil && request.PriorRun == nil {
		snapshot, ok, err := h.FollowUps.Load(ctx, request)
		if err != nil {
			return Response{Text: "Agent context failed."}, true, nil
		}
		if ok {
			request.PriorRun = &snapshot
		}
	}
	trace := Trace{Recorder: h.Recorder, Sink: h.TraceSink, Source: "agent"}
	policy := RoutingPolicy{
		Policy:                       h.Policy,
		Checker:                      h.Checker,
		RequiredCapabilityBeforePlan: h.RequiredCapabilityBeforePlan,
	}
	runner := Runner{
		Planner:        h.Planner,
		ContextFetcher: h.ContextFetcher,
		Policy:         policy,
		Executor: Executor{
			Tools:     h.Tools,
			Answerer:  h.Answerer,
			Policy:    policy,
			Trace:     trace,
			FollowUps: h.FollowUps,
		},
		Trace:    trace,
		Limits:   h.Limits,
		NewRunID: h.NewRunID,
	}
	return runner.Run(ctx, request)
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
