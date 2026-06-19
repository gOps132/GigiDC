package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/contextbroker"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestRunnerDryRunSkipsExecutor(t *testing.T) {
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "memory.count", ToolCalls: []ToolCall{{Name: "fake.tool"}}}}
	tool := &fakeTool{}
	runner := Runner{
		Planner: planner,
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingDryRun}},
		Executor: Executor{
			Tools: NewRegistry(tool),
		},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Planned agent tools: `fake.tool`. LLM tool routing is in `dry-run` mode." {
		t.Fatalf("response=%+v handled=%v, want dry-run", response, handled)
	}
	if !planner.called || tool.called {
		t.Fatalf("planner.called=%v tool.called=%v, want planner only", planner.called, tool.called)
	}
}

func TestRunnerRoutingOffSkipsPlanner(t *testing.T) {
	planner := &fakePlanner{ok: true}
	runner := Runner{
		Planner: planner,
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingOff}},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if handled || response.Text != "" || planner.called {
		t.Fatalf("response=%+v handled=%v planner.called=%v, want skip", response, handled, planner.called)
	}
}

func TestRunnerFetchesChannelContextBeforePlanner(t *testing.T) {
	fetcher := &fakeContextFetcher{pack: contextbroker.Pack{Snippets: []contextbroker.Snippet{{ID: "m1", Text: "postgres deploy"}}}}
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "answer_from_context"}}
	runner := Runner{
		Planner:        planner,
		ContextFetcher: fetcher,
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor:       Executor{},
	}
	request := agentTestRequest()
	request.ContextScope = "channel"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "No agent tool results." {
		t.Fatalf("response=%+v handled=%v, want handled fallback answer", response, handled)
	}
	if !fetcher.called || !planner.sawContext {
		t.Fatalf("fetcher.called=%v planner.sawContext=%v, want fetched context before planner", fetcher.called, planner.sawContext)
	}
}

func TestRunnerSkipsContextFetchWhenScopeNone(t *testing.T) {
	fetcher := &fakeContextFetcher{}
	planner := &fakePlanner{ok: true}
	runner := Runner{
		Planner:        planner,
		ContextFetcher: fetcher,
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
	}
	request := agentTestRequest()
	request.ContextScope = "none"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if handled || response.Text != "" || fetcher.called || planner.called {
		t.Fatalf("response=%+v handled=%v fetcher.called=%v planner.called=%v, want skip", response, handled, fetcher.called, planner.called)
	}
}

func TestRunnerMasksContextFetchErrorAndRecordsTrace(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	fetcher := &fakeContextFetcher{err: errors.New("raw database failure")}
	planner := &fakePlanner{ok: true}
	runner := Runner{
		Planner:        planner,
		ContextFetcher: fetcher,
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Trace:          Trace{Recorder: recorder},
	}
	request := agentTestRequest()
	request.ContextScope = "channel"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent context fetch failed." {
		t.Fatalf("response=%+v handled=%v, want masked context failure", response, handled)
	}
	if planner.called {
		t.Fatalf("planner called after context fetch failure")
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "agent.context" || recorder.events[0].Status != audit.StatusFailed {
		t.Fatalf("events=%+v, want failed context trace", recorder.events)
	}
}

func TestRunnerDeniesContextFetchPermission(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	fetcher := &fakeContextFetcher{err: ErrContextPermissionDenied}
	planner := &fakePlanner{ok: true}
	runner := Runner{
		Planner:        planner,
		ContextFetcher: fetcher,
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Trace:          Trace{Recorder: recorder},
	}
	request := agentTestRequest()
	request.ContextScope = "channel"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Permission denied for agent context." {
		t.Fatalf("response=%+v handled=%v, want permission denied", response, handled)
	}
	if planner.called {
		t.Fatalf("planner called after context permission denial")
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "agent.context" || recorder.events[0].Status != audit.StatusDenied {
		t.Fatalf("events=%+v, want denied context trace", recorder.events)
	}
}

func TestRunnerContinuesWhenOptionalChannelContextDenied(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	fetcher := &fakeContextFetcher{err: ErrContextPermissionDenied}
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "chat"}}
	runner := Runner{
		Planner:        planner,
		ContextFetcher: fetcher,
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Trace:          Trace{Recorder: recorder},
	}
	request := agentTestRequest()
	request.ContextScope = "channel-auto"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "No agent tool results." {
		t.Fatalf("response=%+v handled=%v, want planner fallback", response, handled)
	}
	if !planner.called || planner.sawContext {
		t.Fatalf("planner.called=%v planner.sawContext=%v, want planner without context", planner.called, planner.sawContext)
	}
	if len(recorder.events) != 2 || recorder.events[0].Kind != "agent.context" || recorder.events[0].Status != audit.StatusDenied {
		t.Fatalf("events=%+v, want denied optional context trace then answer", recorder.events)
	}
}

func TestRoutingPolicyNilPolicyDefaultsOff(t *testing.T) {
	mode, err := (RoutingPolicy{}).Mode(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("Mode returned error: %v", err)
	}
	if mode != llmprovider.ToolRoutingOff {
		t.Fatalf("mode = %q, want off", mode)
	}
}

func TestRegistryLookupNormalizesSpec(t *testing.T) {
	registry := NewRegistry(&fakeTool{kind: ToolKind("mutation")})

	_, spec, err := registry.Lookup(" fake.tool ")
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if spec.Name != "fake.tool" || spec.Kind != ToolKindRead {
		t.Fatalf("spec=%+v, want trimmed read spec", spec)
	}
	if _, _, err := registry.Lookup("missing.tool"); err == nil {
		t.Fatalf("Lookup unknown tool returned nil error")
	}
}

func TestExecutorMasksToolErrorAndRecordsTrace(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "fake", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(&fakeTool{err: errors.New("db raw failure")}),
			Trace: Trace{Recorder: recorder},
		},
		Trace: Trace{Recorder: recorder},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent tool failed." {
		t.Fatalf("response=%+v handled=%v, want masked tool failure", response, handled)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "agent.tool" || recorder.events[0].Status != audit.StatusFailed {
		t.Fatalf("events=%+v, want failed tool trace", recorder.events)
	}
}

func TestExecutorDeniesToolCapabilityBeforeExecute(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	tool := &fakeTool{capability: "memory.read.guild"}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "fake", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools:  NewRegistry(tool),
			Policy: RoutingPolicy{Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: false, Reason: capability.ReasonMissingCapability}}},
			Trace:  Trace{Recorder: recorder},
		},
		Trace: Trace{Recorder: recorder},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Permission denied for agent tool." {
		t.Fatalf("response=%+v handled=%v, want denied", response, handled)
	}
	if tool.called {
		t.Fatalf("tool executed after capability denied")
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusDenied || recorder.events[0].Metadata["decision_reason"] == "" {
		t.Fatalf("events=%+v, want denied tool trace", recorder.events)
	}
}

func TestExecutorRequiresConfirmationForWriteTool(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	tool := &fakeTool{kind: ToolKindWrite}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "write", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(tool),
			Trace: Trace{Recorder: recorder},
		},
		Trace: Trace{Recorder: recorder},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "I can plan that, but confirmation is required before running it." {
		t.Fatalf("response=%+v handled=%v, want confirmation guard", response, handled)
	}
	if tool.called {
		t.Fatalf("write tool executed without confirmation")
	}
	if len(recorder.events) != 1 || recorder.events[0].Reason != "confirmation_required" {
		t.Fatalf("events=%+v, want confirmation trace", recorder.events)
	}
}

func TestTraceSkipsEmptyActorAndAddsSource(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	trace := Trace{Recorder: recorder, Source: "agent-test"}

	if err := trace.Record(context.Background(), Request{}, "agent.test", audit.StatusSucceeded, "", nil); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("events=%+v, want skip empty actor", recorder.events)
	}
	if err := trace.Record(context.Background(), agentTestRequest(), "agent.test", audit.StatusSucceeded, "", nil); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if len(recorder.events) != 1 || recorder.events[0].Metadata["source"] != "agent-test" {
		t.Fatalf("events=%+v, want source metadata", recorder.events)
	}
}

func TestRunnerTraceIncludesRunIDAndOrderedSteps(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "multi", ToolCalls: []ToolCall{
			{Name: "tool.a"},
			{Name: "tool.b"},
		}}},
		Policy: RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(&fakeTool{name: "tool.a"}, &fakeTool{name: "tool.b"}),
		},
		Trace:    Trace{Recorder: recorder, Source: "agent-test"},
		NewRunID: func() string { return "run-1" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "fake result\nfake result" {
		t.Fatalf("response=%+v handled=%v, want combined results", response, handled)
	}
	if len(recorder.events) != 3 {
		t.Fatalf("events=%+v, want 2 tools + answer", recorder.events)
	}
	for index, event := range recorder.events {
		if event.Metadata["run_id"] != "run-1" || event.Metadata["source"] != "agent-test" {
			t.Fatalf("event=%+v, want run/source metadata", event)
		}
		wantStep := string(rune('1' + index))
		if event.Metadata["step_index"] != wantStep {
			t.Fatalf("event=%+v, want step_index %s", event, wantStep)
		}
	}
}

func TestRunnerChecksCapabilityBeforePlanner(t *testing.T) {
	planner := &fakePlanner{ok: true}
	runner := Runner{
		Planner: planner,
		Policy: RoutingPolicy{
			Policy:                       fakePolicy{mode: llmprovider.ToolRoutingEnabled},
			Checker:                      fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: false, Reason: capability.ReasonMissingCapability}},
			RequiredCapabilityBeforePlan: "memory.read.guild",
		},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Permission denied for agent tools." {
		t.Fatalf("response=%+v handled=%v, want denied", response, handled)
	}
	if planner.called {
		t.Fatalf("planner called before capability check")
	}
}

func TestRunnerStopsWhenToolBudgetExceeded(t *testing.T) {
	tool := &fakeTool{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "many", ToolCalls: []ToolCall{
			{Name: "fake.tool"},
			{Name: "fake.tool"},
		}}},
		Policy:   RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{Tools: NewRegistry(tool)},
		Limits:   Limits{MaxToolCalls: 1},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent tool budget exceeded." {
		t.Fatalf("response=%+v handled=%v, want budget response", response, handled)
	}
	if tool.called {
		t.Fatalf("tool called after budget exceeded")
	}
}

func TestRunnerStopsWhenStepBudgetExceeded(t *testing.T) {
	tool := &fakeTool{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "steps", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(tool),
		},
		Limits: Limits{MaxSteps: 2},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent step budget exceeded." {
		t.Fatalf("response=%+v handled=%v, want step budget response", response, handled)
	}
	if tool.called {
		t.Fatalf("tool called after step budget exceeded")
	}
}

func TestRunnerCountsContextFetchAgainstStepBudget(t *testing.T) {
	tool := &fakeTool{}
	runner := Runner{
		Planner:        &fakePlanner{ok: true, plan: Plan{Intent: "steps", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		ContextFetcher: &fakeContextFetcher{pack: contextbroker.Pack{Snippets: []contextbroker.Snippet{{ID: "m1", Text: "postgres"}}}},
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(tool),
		},
		Limits: Limits{MaxSteps: 3},
	}
	request := agentTestRequest()
	request.ContextScope = "channel"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent step budget exceeded." {
		t.Fatalf("response=%+v handled=%v, want step budget response", response, handled)
	}
	if tool.called {
		t.Fatalf("tool called after context-aware step budget exceeded")
	}
}

func TestRunnerSkipsAnswererWhenLLMBudgetExceeded(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	answerer := &fakeAnswerer{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "budget", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools:    NewRegistry(&fakeTool{}),
			Answerer: answerer,
		},
		Trace:    Trace{Recorder: recorder},
		Limits:   Limits{Budget: Budget{MaxLLMCalls: 1}},
		NewRunID: func() string { return "run-budget" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "fake result" {
		t.Fatalf("response=%+v handled=%v, want tool summary fallback", response, handled)
	}
	if answerer.called {
		t.Fatalf("answerer called after LLM budget exceeded")
	}
	if len(recorder.events) != 2 || recorder.events[1].Kind != "agent.answer" || recorder.events[1].Reason != "llm_budget_exceeded" {
		t.Fatalf("events=%+v, want answer budget trace", recorder.events)
	}
}
