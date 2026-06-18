package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/memory"
)

func TestPlanningHandlerSkipsWhenRoutingOff(t *testing.T) {
	planner := &fakePlanner{ok: true}
	handler := PlanningHandler{
		Planner: planner,
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingOff},
		Tools:   NewRegistry(),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if handled || response.Text != "" || planner.called {
		t.Fatalf("response=%+v handled=%v planner.called=%v, want skipped", response, handled, planner.called)
	}
}

func TestPlanningHandlerChecksCapabilityBeforePlanner(t *testing.T) {
	planner := &fakePlanner{ok: true}
	recorder := &fakeAgentAuditRecorder{}
	handler := PlanningHandler{
		Planner:                      planner,
		Policy:                       fakePolicy{mode: llmprovider.ToolRoutingEnabled},
		Checker:                      fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: false, Reason: capability.ReasonMissingCapability}},
		Recorder:                     recorder,
		RequiredCapabilityBeforePlan: "memory.read.guild",
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || response.Text != "Permission denied for agent tools." {
		t.Fatalf("response=%+v handled=%v, want denied", response, handled)
	}
	if planner.called {
		t.Fatalf("planner called before capability check")
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusDenied {
		t.Fatalf("events=%+v, want denied audit", recorder.events)
	}
}

func TestPlanningHandlerDryRunDoesNotExecuteTool(t *testing.T) {
	tool := &fakeTool{}
	handler := PlanningHandler{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "memory.count", ToolCalls: []ToolCall{{Name: "fake.tool"}}}},
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingDryRun},
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
		Tools:   NewRegistry(tool),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || !strings.Contains(response.Text, "dry-run") {
		t.Fatalf("response=%+v handled=%v, want dry-run", response, handled)
	}
	if tool.called {
		t.Fatalf("tool executed in dry-run")
	}
}

func TestPlanningHandlerExecutesToolWhenEnabled(t *testing.T) {
	handler := PlanningHandler{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "memory.count", ToolCalls: []ToolCall{{Name: ToolMemoryCount, Args: map[string]string{"text": "postgres"}}}}},
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingEnabled},
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
		Tools: NewRegistry(MemoryCountTool{
			Store:   &fakeAgentMemoryStore{count: memory.CountResult{Count: 3}},
			Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
		}),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || response.Text != "Messages mentioned `postgres` 3 times in this channel." {
		t.Fatalf("response=%+v handled=%v, want memory count", response, handled)
	}
}

func TestMemoryRecentToolExecutes(t *testing.T) {
	store := &fakeAgentMemoryStore{results: []memory.SearchResult{{MessageID: "message-id"}}}
	tool := MemoryRecentTool{
		Store:   store,
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
	}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{Name: ToolMemoryRecent, Args: map[string]string{"limit": "50"}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Name != ToolMemoryRecent || result.Data["messages"] != "1" {
		t.Fatalf("result = %+v, want recent summary", result)
	}
	if store.recentReq.Limit != 25 {
		t.Fatalf("limit = %d, want clamped 25", store.recentReq.Limit)
	}
}

func TestPlanningHandlerMasksPlannerError(t *testing.T) {
	handler := PlanningHandler{
		Planner: &fakePlanner{err: errors.New("provider raw error")},
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingEnabled},
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || response.Text != "Agent routing failed." {
		t.Fatalf("response=%+v handled=%v, want masked failure", response, handled)
	}
}

func agentTestRequest() Request {
	return Request{
		Surface:     SurfaceGuildMention,
		GuildID:     "guild-id",
		ChannelID:   "channel-id",
		ActorUserID: "actor-id",
		Text:        "count postgres",
	}
}

type fakePlanner struct {
	called bool
	plan   Plan
	ok     bool
	err    error
}

func (p *fakePlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	p.called = true
	return p.plan, p.ok, p.err
}

type fakePolicy struct {
	mode llmprovider.ToolRoutingMode
	err  error
}

func (p fakePolicy) GuildPolicy(ctx context.Context, guildID string) (llmprovider.GuildPolicy, error) {
	if p.err != nil {
		return llmprovider.GuildPolicy{}, p.err
	}
	return llmprovider.GuildPolicy{GuildID: guildID, ToolRoutingMode: p.mode}, nil
}

type fakeAgentCapabilityChecker struct {
	decision capability.Decision
	err      error
}

func (c fakeAgentCapabilityChecker) Check(ctx context.Context, subject capability.Subject, required capability.Capability) (capability.Decision, error) {
	decision := c.decision
	decision.Capability = required
	return decision, c.err
}

type fakeAgentAuditRecorder struct {
	events []audit.Event
}

func (r *fakeAgentAuditRecorder) Record(ctx context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

type fakeTool struct {
	called bool
}

func (t *fakeTool) Spec() ToolSpec {
	return ToolSpec{Name: "fake.tool", Kind: ToolKindRead}
}

func (t *fakeTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	t.called = true
	return ToolResult{Name: "fake.tool", Summary: "fake result"}, nil
}

type fakeAgentMemoryStore struct {
	count     memory.CountResult
	results   []memory.SearchResult
	recentReq memory.RecentRequest
}

func (s *fakeAgentMemoryStore) CountMentions(ctx context.Context, req memory.CountRequest) (memory.CountResult, error) {
	return s.count, nil
}

func (s *fakeAgentMemoryStore) SearchMessages(ctx context.Context, req memory.SearchRequest) ([]memory.SearchResult, error) {
	return s.results, nil
}

func (s *fakeAgentMemoryStore) RecentMessages(ctx context.Context, req memory.RecentRequest) ([]memory.SearchResult, error) {
	s.recentReq = req
	return s.results, nil
}
