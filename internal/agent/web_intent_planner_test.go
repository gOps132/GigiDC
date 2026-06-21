package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/llm"
)

func TestWebIntentPlannerRoutesExplicitWebSearch(t *testing.T) {
	request := agentTestRequest()
	request.Text = "search the web for latest Go release notes"

	plan, ok, err := (WebIntentPlanner{}).Plan(context.Background(), request, []ToolSpec{{Name: ToolWebSearch}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != ToolWebSearch || len(plan.ToolCalls) != 1 || plan.ToolCalls[0].Args["query"] != "latest Go release notes" {
		t.Fatalf("plan=%+v ok=%v, want web.search with cleaned query", plan, ok)
	}
}

func TestWebIntentPlannerRoutesSpecificURLFetch(t *testing.T) {
	request := agentTestRequest()
	request.Text = "summarize https://go.dev/doc/go1.25"

	plan, ok, err := (WebIntentPlanner{}).Plan(context.Background(), request, []ToolSpec{{Name: ToolWebFetch}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != ToolWebFetch || len(plan.ToolCalls) != 1 || plan.ToolCalls[0].Args["url"] != "https://go.dev/doc/go1.25" {
		t.Fatalf("plan=%+v ok=%v, want web.fetch", plan, ok)
	}
}

func TestWebIntentPlannerAnswersToolListQuestion(t *testing.T) {
	request := agentTestRequest()
	request.Text = "what tools can you use?"

	plan, ok, err := (WebIntentPlanner{}).Plan(context.Background(), request, []ToolSpec{{Name: ToolWebSearch}, {Name: ToolJobsList}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != "tools.list" || plan.ClarifyingQuestion == "" {
		t.Fatalf("plan=%+v ok=%v, want tools list response", plan, ok)
	}
	for _, want := range []string{ToolWebSearch, ToolJobsList} {
		if !strings.Contains(plan.ClarifyingQuestion, want) {
			t.Fatalf("response=%q, want %q", plan.ClarifyingQuestion, want)
		}
	}
}

func TestMultiPlannerRoutesExplicitWebBeforeLLM(t *testing.T) {
	request := agentTestRequest()
	request.Text = "search web for latest Go release notes"
	llmRuntime := &fakeAgentTextRuntime{response: llm.TextResponse{Text: `{"intent":"answer_from_prior","tool_calls":[]}`}}

	plan, ok, err := (MultiPlanner{
		WebIntentPlanner{},
		LLMPlanner{Runtime: llmRuntime},
	}).Plan(context.Background(), request, []ToolSpec{{Name: ToolWebSearch}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || len(plan.ToolCalls) != 1 || plan.ToolCalls[0].Name != ToolWebSearch {
		t.Fatalf("plan=%+v ok=%v, want deterministic web.search before LLM", plan, ok)
	}
	if llmRuntime.req.Input != "" {
		t.Fatalf("LLM planner was called, request=%+v", llmRuntime.req)
	}
}
