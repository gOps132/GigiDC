package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/contextbroker"
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

func TestPlanningHandlerBuildsContextPackBeforePlanner(t *testing.T) {
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "context"}}
	request := agentTestRequest()
	request.ContextScope = "channel"
	request.ContextSnippets = []contextbroker.Snippet{{ID: "m1", Source: "discord:channel-id", Text: "postgres context"}}
	handler := PlanningHandler{
		Planner: planner,
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingDryRun},
		Tools:   NewRegistry(&fakeTool{}),
	}

	_, _, err := handler.HandleAgentRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !planner.called || len(planner.request.ContextPack.Items) != 1 || planner.request.ContextPack.Items[0].Citation.Label != "S1" {
		t.Fatalf("planner request = %+v, want built context pack before planning", planner.request)
	}
}

func TestPlanningHandlerLoadsContextProviderBeforePlanner(t *testing.T) {
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "context"}}
	provider := fakeContextProvider{
		snippets: []contextbroker.Snippet{{ID: "m1", Source: "discord:channel:channel-id", Text: "postgres context"}},
	}
	request := agentTestRequest()
	request.ContextScope = "channel"
	handler := PlanningHandler{
		Planner:         planner,
		Policy:          fakePolicy{mode: llmprovider.ToolRoutingDryRun},
		Tools:           NewRegistry(&fakeTool{}),
		ContextProvider: provider,
	}

	_, _, err := handler.HandleAgentRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !planner.called || len(planner.request.ContextPack.Items) != 1 || planner.request.ContextPack.Items[0].Snippet.Text != "postgres context" {
		t.Fatalf("planner request = %+v, want provider context pack before planning", planner.request)
	}
}

func TestPlanningHandlerSkipsContextPackWhenScopeNone(t *testing.T) {
	planner := &fakePlanner{ok: true, plan: Plan{Intent: "context"}}
	request := agentTestRequest()
	request.ContextScope = "none"
	request.ContextSnippets = []contextbroker.Snippet{{ID: "m1", Source: "discord:channel-id", Text: "postgres context"}}
	handler := PlanningHandler{
		Planner: planner,
		Policy:  fakePolicy{mode: llmprovider.ToolRoutingDryRun},
		Tools:   NewRegistry(&fakeTool{}),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if handled || response.Text != "" || planner.called {
		t.Fatalf("response=%+v handled=%v planner.called=%v, want skipped context none", response, handled, planner.called)
	}
}

func TestMemoryContextProviderLoadsRecentSnippetsWhenAllowed(t *testing.T) {
	store := &fakeAgentMemoryStore{results: []memory.SearchResult{{
		MessageID:    "m-1",
		ChannelID:    "channel-id",
		AuthorUserID: "user-id",
		Text:         "postgres outage notes",
		CreatedAt:    time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
	}}}
	provider := MemoryContextProvider{
		Store:   store,
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
		Limit:   50,
	}
	request := agentTestRequest()
	request.ContextScope = "channel"

	snippets, err := provider.LoadContextSnippets(context.Background(), request)
	if err != nil {
		t.Fatalf("LoadContextSnippets returned error: %v", err)
	}
	if len(snippets) != 1 || snippets[0].ID != "message:m-1" || snippets[0].Source != "discord:channel:channel-id" || snippets[0].AuthorID != "user-id" || !strings.Contains(snippets[0].Text, "postgres outage notes") {
		t.Fatalf("snippets=%+v, want discord memory snippet", snippets)
	}
	if store.recentReq.Limit != maxMemoryContextLimit {
		t.Fatalf("limit=%d, want clamped memory context limit", store.recentReq.Limit)
	}
}

func TestMemoryContextProviderSkipsWhenDenied(t *testing.T) {
	store := &fakeAgentMemoryStore{results: []memory.SearchResult{{MessageID: "m-1", ChannelID: "channel-id", Text: "hidden"}}}
	provider := MemoryContextProvider{
		Store:   store,
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: false, Reason: capability.ReasonMissingCapability}},
	}
	request := agentTestRequest()
	request.ContextScope = "channel"

	snippets, err := provider.LoadContextSnippets(context.Background(), request)
	if err != nil {
		t.Fatalf("LoadContextSnippets returned error: %v", err)
	}
	if len(snippets) != 0 || store.recentReq.GuildID != "" {
		t.Fatalf("snippets=%+v recentReq=%+v, want no memory fetch without capability", snippets, store.recentReq)
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

func TestPlanningHandlerPassesRunnerLimits(t *testing.T) {
	tool := &fakeTool{}
	handler := PlanningHandler{
		Planner: &fakePlanner{ok: true, plan: Plan{Intent: "many", ToolCalls: []ToolCall{
			{Name: "fake.tool"},
			{Name: "fake.tool"},
		}}},
		Policy: fakePolicy{mode: llmprovider.ToolRoutingEnabled},
		Tools:  NewRegistry(tool),
		Limits: Limits{MaxToolCalls: 1},
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), agentTestRequest())
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || response.Text != "Agent tool budget exceeded." {
		t.Fatalf("response=%+v handled=%v, want limit response", response, handled)
	}
	if tool.called {
		t.Fatalf("tool executed after handler limit")
	}
}

func TestMemoryRecentToolExecutes(t *testing.T) {
	store := &fakeAgentMemoryStore{results: []memory.SearchResult{{
		MessageID:      "message-id",
		GuildID:        "guild-id",
		ChannelID:      "channel-id",
		AuthorUserID:   "user-id",
		Text:           "postgres is neat",
		CreatedAt:      time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
		RetentionUntil: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
		RetrievedAt:    time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
	}}}
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
	if result.Data["message_ids"] != "message-id" || result.Data["citation_labels"] != "S1" || result.Data["source_ids"] != "discord:channel:channel-id:message:message-id" || result.Data["restore_handles"] == "" || result.Data["retention_untils"] == "" || result.Data["retrieved_at"] == "" {
		t.Fatalf("data = %+v, want cited evidence payload", result.Data)
	}
	if _, ok := result.Data["snippets"]; ok || !strings.Contains(result.Summary, "[S1]") {
		t.Fatalf("result = %+v, want cited summary without raw snippet data", result)
	}
	if store.recentReq.Limit != 25 {
		t.Fatalf("limit = %d, want clamped 25", store.recentReq.Limit)
	}
}

func TestMemorySearchToolIncludesResultDetails(t *testing.T) {
	store := &fakeAgentMemoryStore{results: []memory.SearchResult{{
		MessageID:      "m-1",
		GuildID:        "guild-id",
		ChannelID:      "channel-id",
		AuthorUserID:   "user-id",
		Text:           "postgres outage thread",
		CreatedAt:      time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
		RetentionUntil: time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC),
		RetrievedAt:    time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
	}}}
	tool := MemorySearchTool{
		Store:   store,
		Checker: fakeAgentCapabilityChecker{decision: capability.Decision{Allowed: true, Reason: capability.ReasonRoleGrant}},
	}

	result, err := tool.Execute(context.Background(), agentTestRequest(), ToolCall{Name: ToolMemorySearch, Args: map[string]string{"query": "postgres"}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Data["matches"] != "1" || result.Data["message_ids"] != "m-1" || result.Data["citation_labels"] != "S1" || result.Data["source_ids"] != "discord:channel:channel-id:message:m-1" || !strings.Contains(result.Summary, "[S1]") {
		t.Fatalf("result = %+v, want cited search evidence payload", result)
	}
	if _, ok := result.Data["snippets"]; ok {
		t.Fatalf("result = %+v, want no raw snippet data", result)
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
	called  bool
	request Request
	plan    Plan
	ok      bool
	err     error
}

func (p *fakePlanner) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	p.called = true
	p.request = request
	return p.plan, p.ok, p.err
}

type fakeContextProvider struct {
	snippets []contextbroker.Snippet
	err      error
}

func (p fakeContextProvider) LoadContextSnippets(context.Context, Request) ([]contextbroker.Snippet, error) {
	return p.snippets, p.err
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

type fakeAnswerer struct {
	called bool
}

func (a *fakeAnswerer) Answer(ctx context.Context, request Request, plan Plan, results []ToolResult) (Response, error) {
	a.called = true
	return Response{Text: "answerer response"}, nil
}

type fakeTool struct {
	name       string
	called     bool
	err        error
	kind       ToolKind
	capability string
	result     ToolResult
}

func (t *fakeTool) Spec() ToolSpec {
	name := t.name
	if name == "" {
		name = "fake.tool"
	}
	kind := t.kind
	if kind == "" {
		kind = ToolKindRead
	}
	return ToolSpec{Name: name, Kind: kind, Capability: t.capability}
}

func (t *fakeTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	t.called = true
	if t.err != nil {
		return ToolResult{}, t.err
	}
	if t.result.Name != "" || t.result.Summary != "" || t.result.Data != nil {
		return t.result, nil
	}
	return ToolResult{Name: t.Spec().Name, Summary: "fake result"}, nil
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
