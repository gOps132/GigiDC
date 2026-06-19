package evalharness

import (
	"context"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/agent"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestRunAgentCaseCapturesGoldenReadPath(t *testing.T) {
	result := RunAgentCase(context.Background(), AgentCase{
		Name: "read path",
		Runner: testRunner(agent.Plan{
			Intent:    "memory.search",
			ToolCalls: []agent.ToolCall{{Name: "fake.read", Args: map[string]string{"query": "postgres"}}},
		}, agent.ToolKindRead),
		Request:               testRequest(),
		WantHandled:           true,
		WantTextContains:      []string{"answer from fake.read"},
		WantRunStatus:         agent.RunStatusSucceeded,
		WantTerminationReason: agent.TerminationCompleted,
	})

	if len(result.Failures) > 0 {
		t.Fatalf("failures=%+v", result.Failures)
	}
	if len(result.Events) != 3 || result.Events[0].Kind != "agent.plan" || result.Events[1].Kind != "agent.tool" || result.Events[2].Kind != "agent.answer" {
		t.Fatalf("events=%+v, want plan, tool, and answer events", result.Events)
	}
	if len(result.Steps) != 3 || result.Steps[0].Kind != "agent.plan" || result.Steps[1].Kind != "agent.tool" || result.Steps[2].Kind != "agent.answer" {
		t.Fatalf("steps=%+v, want durable plan, tool, and answer steps", result.Steps)
	}
	if result.Response.RunID == "" || len(result.StartedRuns) != 1 || len(result.CompletedRuns) != 1 {
		t.Fatalf("result=%+v, want durable run lifecycle", result)
	}
}

func TestRunAgentCaseFlagsObservabilityLeaks(t *testing.T) {
	result := RunAgentCase(context.Background(), AgentCase{
		Name: "write confirmation redaction",
		Runner: testRunner(agent.Plan{
			Intent: "write",
			ToolCalls: []agent.ToolCall{{
				Name: "fake.write",
				Args: map[string]string{
					"target":  "channel-id",
					"api_key": "sk-live-secret",
				},
			}},
		}, agent.ToolKindWrite),
		Request:                      testRequest(),
		WantHandled:                  true,
		WantTextContains:             []string{"confirmation is required"},
		WantRunStatus:                agent.RunStatusConfirmationRequired,
		WantTerminationReason:        agent.TerminationConfirmationRequired,
		ForbiddenObservabilityValues: []string{"sk-live-secret"},
	})

	if len(result.Failures) > 0 {
		t.Fatalf("failures=%+v", result.Failures)
	}
	if result.Response.RunID == "" || result.Response.ConfirmationID == "" {
		t.Fatalf("response=%+v, want run and confirmation IDs", result.Response)
	}
	if len(result.Confirmations) != 1 || result.Confirmations[0].ToolName != "fake.write" {
		t.Fatalf("confirmations=%+v, want pending write confirmation", result.Confirmations)
	}
	if payload := result.Confirmations[0].Payload; payload["arg.target"] != "channel-id" || payload["arg.api_key"] != "" {
		t.Fatalf("payload=%+v, want sanitized target and no api key", payload)
	}
}

func TestRunAgentCaseReportsExpectationFailures(t *testing.T) {
	result := RunAgentCase(context.Background(), AgentCase{
		Name: "intentional mismatch",
		Runner: testRunner(agent.Plan{
			Intent:    "memory.search",
			ToolCalls: []agent.ToolCall{{Name: "fake.read"}},
		}, agent.ToolKindRead),
		Request:               testRequest(),
		WantHandled:           true,
		WantTextContains:      []string{"missing text"},
		WantRunStatus:         agent.RunStatusFailed,
		WantTerminationReason: agent.TerminationPlannerFailed,
	})

	if len(result.Failures) == 0 {
		t.Fatal("failures empty, want mismatch report")
	}
	joined := strings.Join(result.Failures, "\n")
	for _, want := range []string{"response text missing", "run status", "termination reason"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failures=%+v, want %q", result.Failures, want)
		}
	}
}

func testRunner(plan agent.Plan, kind agent.ToolKind) agent.Runner {
	return agent.Runner{
		Planner: &evalPlanner{plan: plan},
		Policy:  agent.RoutingPolicy{Policy: evalPolicy{}},
		Executor: agent.Executor{
			Tools:    agent.NewRegistry(evalTool{name: plan.ToolCalls[0].Name, kind: kind}),
			Answerer: evalAnswerer{},
			Policy:   agent.RoutingPolicy{Policy: evalPolicy{}},
		},
		NewRunID: func() string { return "run-eval" },
	}
}

func testRequest() agent.Request {
	return agent.Request{
		Surface:     agent.SurfaceGuildMention,
		GuildID:     "guild-id",
		ChannelID:   "channel-id",
		ActorUserID: "actor-id",
		Text:        "please search",
	}
}

type evalPolicy struct{}

func (evalPolicy) GuildPolicy(context.Context, string) (llmprovider.GuildPolicy, error) {
	return llmprovider.GuildPolicy{ToolRoutingMode: llmprovider.ToolRoutingEnabled}, nil
}

type evalPlanner struct {
	plan agent.Plan
}

func (p *evalPlanner) Plan(context.Context, agent.Request, []agent.ToolSpec) (agent.Plan, bool, error) {
	return p.plan, true, nil
}

type evalTool struct {
	name string
	kind agent.ToolKind
}

func (t evalTool) Spec() agent.ToolSpec {
	return agent.ToolSpec{Name: t.name, Kind: t.kind}
}

func (t evalTool) Execute(context.Context, agent.Request, agent.ToolCall) (agent.ToolResult, error) {
	return agent.ToolResult{Name: t.name, Summary: "tool summary"}, nil
}

type evalAnswerer struct{}

func (evalAnswerer) Answer(_ context.Context, _ agent.Request, _ agent.Plan, results []agent.ToolResult) (agent.Response, error) {
	if len(results) == 0 {
		return agent.Response{Text: "answer without tools"}, nil
	}
	return agent.Response{Text: "answer from " + results[0].Name}, nil
}
