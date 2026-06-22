package agent

import (
	"context"
	"errors"
	"strings"
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

func TestRunnerFallsThroughWhenPlannerNeedsNoTools(t *testing.T) {
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "chat"}}
	answerer := &fakeAnswerer{}
	runner := Runner{
		Planner: planner,
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Answerer: answerer,
		},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if handled || response.Text != "" || !planner.called || answerer.called {
		t.Fatalf("response=%+v handled=%v planner.called=%v answerer.called=%v, want chat fallback", response, handled, planner.called, answerer.called)
	}
}

func TestRunnerRecordsTraceWhenPlannerReturnsNoPlan(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	planner := &fakePlanner{ok: false, plan: Plan{Trace: map[string]string{"llm_attempt": "repair", "llm_provider": "openai"}}}
	runner := Runner{
		Planner: planner,
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Trace:   Trace{Recorder: recorder},
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent routing produced no plan." || !planner.called {
		t.Fatalf("response=%+v handled=%v planner.called=%v, want handled no-plan response", response, handled, planner.called)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "agent.plan" || recorder.events[0].Reason != "no_plan" || recorder.events[0].Metadata["planner"] != "agent" || recorder.events[0].Metadata["llm_attempt"] != "repair" {
		t.Fatalf("events=%+v, want no-plan trace", recorder.events)
	}
}

func TestRunnerFallsThroughWhenAutoContextNoToolPlanIsPlainChat(t *testing.T) {
	fetcher := &fakeContextFetcher{pack: contextbroker.Pack{Snippets: []contextbroker.Snippet{{ID: "m1", Text: "recent unrelated context"}}}}
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "chat"}}
	answerer := &fakeAnswerer{}
	runner := Runner{
		Planner:        planner,
		ContextFetcher: fetcher,
		Policy:         RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Answerer: answerer,
		},
	}
	request := agentTestRequest()
	request.ContextScope = "channel-auto"

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if handled || response.Text != "" || !fetcher.called || !planner.called || !planner.sawContext || answerer.called {
		t.Fatalf("response=%+v handled=%v fetcher.called=%v planner.called=%v planner.sawContext=%v answerer.called=%v, want chat fallback after auto context", response, handled, fetcher.called, planner.called, planner.sawContext, answerer.called)
	}
}

func TestRunnerKeepsNoToolPlanWhenPriorRunExists(t *testing.T) {
	answerer := &fakeAnswerer{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "answer_from_prior"}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Answerer: answerer,
		},
	}
	request := agentTestRequest()
	request.PriorRun = &RunSnapshot{Intent: "memory.recent", ResponseText: "prior answer"}

	response, handled, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "answerer response" || !answerer.called {
		t.Fatalf("response=%+v handled=%v answerer.called=%v, want prior answer", response, handled, answerer.called)
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
	if handled || response.Text != "" {
		t.Fatalf("response=%+v handled=%v, want chat fallback", response, handled)
	}
	if !planner.called || planner.sawContext {
		t.Fatalf("planner.called=%v planner.sawContext=%v, want planner without context", planner.called, planner.sawContext)
	}
	if len(recorder.events) != 2 || recorder.events[0].Kind != "agent.context" || recorder.events[0].Status != audit.StatusDenied || recorder.events[1].Kind != "agent.plan" || recorder.events[1].Reason != "no_tools" {
		t.Fatalf("events=%+v, want denied optional context trace then no-tool plan", recorder.events)
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
	if len(recorder.events) != 2 || recorder.events[1].Kind != "agent.tool" || recorder.events[1].Status != audit.StatusFailed {
		t.Fatalf("events=%+v, want failed tool trace", recorder.events)
	}
	if _, ok := recorder.events[1].Metadata["error_message"]; ok {
		t.Fatalf("events=%+v, want generic tool error masked", recorder.events)
	}
}

func TestExecutorRecordsSafeWebSearchFailureTrace(t *testing.T) {
	recorder := &fakeAgentAuditRecorder{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "search", ToolCalls: []ToolCall{{Name: ToolWebSearch, Args: map[string]string{"query": "news today"}}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(&fakeTool{name: ToolWebSearch, err: WebSearchError{Provider: "duckduckgo", Err: errSearchProviderChallenge}}),
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
	metadata := recorder.events[1].Metadata
	if metadata["error_provider"] != "duckduckgo" || metadata["error_kind"] != "provider_challenge" || !strings.Contains(metadata["error_message"], "search provider challenge") {
		t.Fatalf("metadata=%+v, want safe web search error metadata", metadata)
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
	if len(recorder.events) != 2 || recorder.events[1].Status != audit.StatusDenied || recorder.events[1].Metadata["decision_reason"] == "" {
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
	if len(recorder.events) != 2 || recorder.events[1].Reason != "confirmation_required" {
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

func TestTraceSanitizesMetadataBeforeDurableStep(t *testing.T) {
	store := &fakeRunStore{}
	trace := Trace{Store: store, RunID: "run-id", Source: "agent-test"}

	if err := trace.Record(context.Background(), agentTestRequest(), "agent.tool", audit.StatusSucceeded, "", map[string]string{
		"tool":    "memory.search",
		"api_key": "sk-secret",
		"value":   "sk-secret",
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if len(store.steps) != 1 {
		t.Fatalf("steps=%+v, want one durable step", store.steps)
	}
	if _, ok := store.steps[0].Observation["api_key"]; ok {
		t.Fatalf("observation=%+v, want sensitive key removed", store.steps[0].Observation)
	}
	if store.steps[0].Observation["value"] != "[REDACTED]" {
		t.Fatalf("observation=%+v, want sensitive value redacted", store.steps[0].Observation)
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
	if len(recorder.events) != 4 {
		t.Fatalf("events=%+v, want plan + 2 tools + answer", recorder.events)
	}
	for index, event := range recorder.events {
		if event.Metadata["run_id"] != "run-1" || event.Metadata["source"] != "agent-test" {
			t.Fatalf("event=%+v, want run/source metadata", event)
		}
		if index == 0 {
			continue
		}
		wantStep := string(rune('0' + index))
		if event.Metadata["step_index"] != wantStep {
			t.Fatalf("event=%+v, want step_index %s", event, wantStep)
		}
	}
}

func TestTraceRecordsSafeLastRun(t *testing.T) {
	store := NewMemoryTraceStore(10)
	trace := Trace{Sink: store, Source: "agent-test", RunID: "run-1", Step: 2}
	if err := trace.Record(context.Background(), agentTestRequest(), "agent.tool", audit.StatusSucceeded, "", map[string]string{
		"tool":       "memory.recent",
		"kind":       "read",
		"capability": "memory.read.guild",
	}); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	run, ok, err := store.LastTrace(context.Background(), TraceQuery{GuildID: "guild-id", ChannelID: "channel-id", ActorUserID: "actor-id"})
	if err != nil {
		t.Fatalf("LastTrace returned error: %v", err)
	}
	if !ok || run.RunID != "run-1" || len(run.Events) != 1 {
		t.Fatalf("run=%+v ok=%v, want one trace event", run, ok)
	}
	event := run.Events[0]
	if event.Phase != "tool" || event.StepIndex != 2 || event.ToolName != "memory.recent" || event.Capability != "memory.read.guild" {
		t.Fatalf("event=%+v, want safe tool metadata", event)
	}
	if event.Details["tool"] != "memory.recent" || event.Details["capability"] != "memory.read.guild" {
		t.Fatalf("details=%+v, want copied safe details", event.Details)
	}
}

func TestTraceStoreScopesDuplicateRunIDs(t *testing.T) {
	store := NewMemoryTraceStore(10)
	first := agentTestRequest()
	second := agentTestRequest()
	second.ActorUserID = "other-actor"
	trace := Trace{Sink: store, RunID: "same-run", Step: 1}
	if err := trace.Record(context.Background(), first, "agent.tool", audit.StatusSucceeded, "", map[string]string{"tool": "memory.recent"}); err != nil {
		t.Fatalf("Record first returned error: %v", err)
	}
	if err := trace.Record(context.Background(), second, "agent.tool", audit.StatusSucceeded, "", map[string]string{"tool": "plugins.enabled"}); err != nil {
		t.Fatalf("Record second returned error: %v", err)
	}
	run, ok, err := store.LastTrace(context.Background(), TraceQuery{GuildID: "guild-id", ChannelID: "channel-id", ActorUserID: "actor-id"})
	if err != nil {
		t.Fatalf("LastTrace returned error: %v", err)
	}
	if !ok || len(run.Events) != 1 || run.Events[0].ToolName != "memory.recent" {
		t.Fatalf("run=%+v ok=%v, want first actor trace only", run, ok)
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
	if len(recorder.events) != 3 || recorder.events[2].Kind != "agent.answer" || recorder.events[2].Reason != "llm_budget_exceeded" {
		t.Fatalf("events=%+v, want answer budget trace", recorder.events)
	}
}

func TestRunnerPersistsRunLifecycleAndSteps(t *testing.T) {
	store := &fakeRunStore{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "multi", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(&fakeTool{}),
		},
		Trace:    Trace{Source: "agent-test"},
		Limits:   Limits{MaxSteps: 4, MaxToolCalls: 2, Budget: Budget{MaxLLMCalls: 1, MaxInputTokens: 3000, MaxOutputTokens: 500}},
		RunStore: store,
		NewRunID: func() string { return "run-1" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "fake result" {
		t.Fatalf("response=%+v handled=%v, want handled tool result", response, handled)
	}
	if len(store.started) != 1 || store.started[0].ID != "run-1" || store.started[0].MaxSteps != 4 || store.started[0].MaxToolCalls != 2 || store.started[0].MaxLLMCalls != 1 || store.started[0].MaxInputTokens != 3000 || store.started[0].MaxOutputTokens != 500 {
		t.Fatalf("started=%+v, want run with budgets", store.started)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusSucceeded || store.completed[0].reason != TerminationCompleted {
		t.Fatalf("completed=%+v, want succeeded completed run", store.completed)
	}
	if len(store.steps) != 3 || store.steps[0].Kind != "agent.plan" || store.steps[1].Kind != "agent.tool" || store.steps[2].Kind != "agent.answer" || store.steps[0].RunID != "run-1" {
		t.Fatalf("steps=%+v, want plan, tool, and answer durable steps", store.steps)
	}
}

func TestRunnerCompletesRunWithDryRunTermination(t *testing.T) {
	store := &fakeRunStore{}
	runner := Runner{
		Planner:  &fakePlanner{ok: true, plan: Plan{Intent: "dry", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:   RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingDryRun}},
		Executor: Executor{Tools: NewRegistry(&fakeTool{})},
		RunStore: store,
		NewRunID: func() string { return "run-dry" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || !strings.Contains(response.Text, "dry-run") {
		t.Fatalf("response=%+v handled=%v, want dry-run response", response, handled)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusDryRun || store.completed[0].reason != TerminationDryRun {
		t.Fatalf("completed=%+v, want dry-run completion", store.completed)
	}
}

func TestRunnerStopsWhenDurableRunCanceled(t *testing.T) {
	store := &fakeRunStore{canceled: true}
	planner := &fakePlanner{ok: true}
	runner := Runner{
		Planner:  planner,
		Policy:   RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		RunStore: store,
		NewRunID: func() string { return "run-cancel" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent run was canceled." {
		t.Fatalf("response=%+v handled=%v, want canceled response", response, handled)
	}
	if planner.called {
		t.Fatal("planner called after run cancellation")
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusCanceled || store.completed[0].reason != TerminationCanceled {
		t.Fatalf("completed=%+v, want canceled completion", store.completed)
	}
}

func TestRunnerCompletesWriteToolAsConfirmationRequired(t *testing.T) {
	store := &fakeRunStore{}
	runner := Runner{
		Planner:  &fakePlanner{ok: true, plan: Plan{Intent: "write", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:   RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{Tools: NewRegistry(&fakeTool{kind: ToolKindWrite})},
		RunStore: store,
		NewRunID: func() string { return "run-confirm" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || !strings.Contains(response.Text, "confirmation is required") {
		t.Fatalf("response=%+v handled=%v, want confirmation response", response, handled)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusConfirmationRequired || store.completed[0].reason != TerminationConfirmationRequired {
		t.Fatalf("completed=%+v, want confirmation-required completion", store.completed)
	}
}

func TestRunnerCompletesToolDenialAsDenied(t *testing.T) {
	store := &fakeRunStore{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "denied", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools:  NewRegistry(&fakeTool{capability: "memory.read.guild"}),
			Policy: RoutingPolicy{Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: false, Reason: capability.ReasonMissingCapability}}},
		},
		RunStore: store,
		NewRunID: func() string { return "run-deny" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Permission denied for agent tool." {
		t.Fatalf("response=%+v handled=%v, want denied response", response, handled)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusDenied || store.completed[0].reason != TerminationPermissionDenied {
		t.Fatalf("completed=%+v, want denied completion", store.completed)
	}
}

func TestRunnerStopsBetweenToolsWhenRunCanceled(t *testing.T) {
	store := &fakeRunStore{cancelAfterStepCount: 1}
	secondTool := &fakeTool{name: "tool.b"}
	answerer := &fakeAnswerer{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "cancel", ToolCalls: []ToolCall{
			{Name: "tool.a"},
			{Name: "tool.b"},
		}}},
		Policy: RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools:    NewRegistry(&fakeTool{name: "tool.a"}, secondTool),
			Answerer: answerer,
		},
		RunStore: store,
		NewRunID: func() string { return "run-cancel-mid" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent run was canceled." {
		t.Fatalf("response=%+v handled=%v, want canceled response", response, handled)
	}
	if secondTool.called || answerer.called {
		t.Fatalf("secondTool.called=%v answerer.called=%v, want stop before next tool/answer", secondTool.called, answerer.called)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusCanceled || store.completed[0].reason != TerminationCanceled {
		t.Fatalf("completed=%+v, want canceled completion", store.completed)
	}
}

func TestRunnerStopsBeforeAnswerWhenRunCanceled(t *testing.T) {
	store := &fakeRunStore{cancelAfterStepCount: 1}
	answerer := &fakeAnswerer{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "cancel", ToolCalls: []ToolCall{
			{Name: "fake.tool"},
		}}},
		Policy: RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools:    NewRegistry(&fakeTool{}),
			Answerer: answerer,
		},
		RunStore: store,
		NewRunID: func() string { return "run-cancel-answer" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent run was canceled." {
		t.Fatalf("response=%+v handled=%v, want canceled response", response, handled)
	}
	if answerer.called {
		t.Fatal("answerer called after run cancellation")
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusCanceled || store.completed[0].reason != TerminationCanceled {
		t.Fatalf("completed=%+v, want canceled completion", store.completed)
	}
}

func TestRunnerDoesNotClassifyTerminationFromResponseText(t *testing.T) {
	store := &fakeRunStore{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "text", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{Tools: NewRegistry(&fakeTool{
			result: ToolResult{Name: "fake.tool", Summary: "no failures found"},
		})},
		RunStore: store,
		NewRunID: func() string { return "run-text" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "no failures found" {
		t.Fatalf("response=%+v handled=%v, want normal result text", response, handled)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusSucceeded || store.completed[0].reason != TerminationCompleted {
		t.Fatalf("completed=%+v, want typed success despite response text", store.completed)
	}
}

func TestRunnerCreatesPendingConfirmationRecordForWriteTool(t *testing.T) {
	store := &fakeRunStore{}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "write", ToolCalls: []ToolCall{{
			Name: "fake.write",
			Args: map[string]string{"target": "message-id", "api_key": "sk-secret"},
		}}}},
		Policy: RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(&fakeTool{name: "fake.write", kind: ToolKindWrite}),
		},
		RunStore: store,
		NewRunID: func() string { return "run-confirm" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || !strings.Contains(response.Text, "confirmation is required") {
		t.Fatalf("response=%+v handled=%v, want confirmation response", response, handled)
	}
	if len(store.confirmations) != 1 || store.confirmations[0].RunID != "run-confirm" || store.confirmations[0].Status != ConfirmationStatusPending || store.confirmations[0].ToolName != "fake.write" {
		t.Fatalf("confirmations=%+v, want pending write confirmation", store.confirmations)
	}
	if _, ok := store.confirmations[0].Payload["api_key"]; ok {
		t.Fatalf("confirmation payload=%+v, want sensitive key removed", store.confirmations[0].Payload)
	}
}

func TestRunnerFailsWhenPendingConfirmationCannotBeStored(t *testing.T) {
	store := &fakeRunStore{confirmErr: errors.New("db down")}
	runner := Runner{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "write", ToolCalls: []ToolCall{{Name: "fake.write"}}}},
		Policy:  RoutingPolicy{Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled}},
		Executor: Executor{
			Tools: NewRegistry(&fakeTool{name: "fake.write", kind: ToolKindWrite}),
		},
		RunStore: store,
		NewRunID: func() string { return "run-confirm-fail" },
	}

	response, handled, err := runner.Run(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !handled || response.Text != "Agent confirmation failed." {
		t.Fatalf("response=%+v handled=%v, want confirmation failure", response, handled)
	}
	if len(store.completed) != 1 || store.completed[0].status != RunStatusFailed || store.completed[0].reason != TerminationExecutorFailed {
		t.Fatalf("completed=%+v, want failed completion", store.completed)
	}
}

type fakeRunStore struct {
	started              []RunRecord
	completed            []runCompletion
	steps                []StepRecord
	confirmations        []ConfirmationRecord
	canceled             bool
	cancelAfterStepCount int
	confirmErr           error
}

type runCompletion struct {
	id     string
	status RunStatus
	reason TerminationReason
}

func (s *fakeRunStore) StartRun(_ context.Context, record RunRecord) error {
	s.started = append(s.started, record)
	return nil
}

func (s *fakeRunStore) CompleteRun(_ context.Context, runID string, status RunStatus, reason TerminationReason) error {
	s.completed = append(s.completed, runCompletion{id: runID, status: status, reason: reason})
	return nil
}

func (s *fakeRunStore) RecordStep(_ context.Context, record StepRecord) error {
	s.steps = append(s.steps, record)
	return nil
}

func (s *fakeRunStore) IsRunCanceled(_ context.Context, runID string) (bool, error) {
	return s.canceled || (s.cancelAfterStepCount > 0 && len(s.steps) >= s.cancelAfterStepCount), nil
}

func (s *fakeRunStore) RequestCancelRun(_ context.Context, runID string, actorID string) error {
	s.canceled = true
	return nil
}

func (s *fakeRunStore) CreateConfirmation(_ context.Context, record ConfirmationRecord) error {
	if s.confirmErr != nil {
		return s.confirmErr
	}
	s.confirmations = append(s.confirmations, record)
	return nil
}

func (s *fakeRunStore) PendingConfirmation(context.Context, string) (ConfirmationRecord, bool, error) {
	if len(s.confirmations) == 0 {
		return ConfirmationRecord{}, false, nil
	}
	return s.confirmations[0], true, nil
}

func (s *fakeRunStore) ResolveConfirmation(context.Context, string, ConfirmationStatus, string) error {
	return nil
}
