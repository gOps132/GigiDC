package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestMultiPlannerUsesFirstHandledPlan(t *testing.T) {
	second := &fakePlanner{ok: true, plan: Plan{Intent: "second"}}
	planner := MultiPlanner{
		&fakePlanner{},
		second,
		&fakePlanner{err: errors.New("should not run")},
	}

	plan, ok, err := planner.Plan(context.Background(), agentTestRequest(), nil)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != "second" || !second.called {
		t.Fatalf("plan=%+v ok=%v called=%v, want second plan", plan, ok, second.called)
	}
}

func TestMultiPlannerLetsLLMHandleConversationalGuildMention(t *testing.T) {
	request := agentTestRequest()
	request.Text = "can you summarize the recent chats?"
	planningRuntime := &fakeAgentTextRuntime{response: llm.TextResponse{Text: `{"intent":"summarize_recent_chat","tool_calls":[{"name":"memory.recent","args":{"limit":"25"}}]}`}}
	planner := MultiPlanner{
		LLMPlanner{Runtime: planningRuntime},
	}

	plan, ok, err := planner.Plan(context.Background(), request, []ToolSpec{{Name: ToolMemoryRecent, Description: "Recent messages"}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != IntentSummarizeRecentChat || plan.ToolCalls[0].Name != ToolMemoryRecent || plan.ToolCalls[0].Args["limit"] != "25" {
		t.Fatalf("plan=%+v ok=%v, want LLM recent-chat summary plan", plan, ok)
	}
	if planningRuntime.req.Purpose != llmprovider.PurposeRouting || !strings.Contains(planningRuntime.req.Input, "can you summarize the recent chats?") {
		t.Fatalf("request=%+v, want conversational text routed through LLM planner", planningRuntime.req)
	}
}

func TestMultiPlannerLetsLLMHandleSlashLikeGuildMentionText(t *testing.T) {
	request := agentTestRequest()
	request.Text = "/plugins dry-run play never gonna give you up"
	planningRuntime := &fakeAgentTextRuntime{response: llm.TextResponse{Text: `{"intent":"plugin_plan","tool_calls":[{"name":"plugins.plan","args":{"text":"play never gonna give you up"}}]}`}}
	planner := MultiPlanner{
		LLMPlanner{Runtime: planningRuntime},
	}

	plan, ok, err := planner.Plan(context.Background(), request, []ToolSpec{{Name: ToolPluginsPlan, Description: "Plan an external app command"}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != "plugin_plan" || plan.ToolCalls[0].Name != ToolPluginsPlan {
		t.Fatalf("plan=%+v ok=%v, want LLM plugin plan", plan, ok)
	}
	if planningRuntime.req.Purpose != llmprovider.PurposeRouting || !strings.Contains(planningRuntime.req.Input, "/plugins dry-run") {
		t.Fatalf("request=%+v, want slash-like mention text routed through LLM planner", planningRuntime.req)
	}
}

func TestPlanningHandlerUsesLLMPlannerAndAnswererForConversationalRecentChat(t *testing.T) {
	request := agentTestRequest()
	request.Text = "can you summarize the recent chats?"
	planningRuntime := &fakeAgentTextRuntime{response: llm.TextResponse{Text: `{"intent":"summarize_recent_chat","tool_calls":[{"name":"memory.recent","args":{"limit":"25"}}]}`}}
	answerRuntime := &fakeAgentTextRuntime{response: llm.TextResponse{Text: "Recent chat centered on deploy follow-up. [S1]"}}
	memoryTool := &fakeTool{
		name: ToolMemoryRecent,
		result: ToolResult{
			Name:    ToolMemoryRecent,
			Summary: "Recent retained full-mode messages in this channel (2):\n- [S1] alice: deploy follow-up\n- [S2] bob: ack",
			Data:    map[string]string{"citation_labels": "S1,S2"},
		},
	}
	handler := PlanningHandler{
		Planner: MultiPlanner{
			LLMPlanner{Runtime: planningRuntime},
		},
		Answerer: LLMAnswerer{Runtime: answerRuntime},
		Policy:   fakePolicy{mode: llmprovider.ToolRoutingEnabled},
		Tools:    NewRegistry(memoryTool),
	}

	response, handled, err := handler.HandleAgentRequest(context.Background(), request)
	if err != nil {
		t.Fatalf("HandleAgentRequest returned error: %v", err)
	}
	if !handled || response.Text != "Recent chat centered on deploy follow-up. [S1]" {
		t.Fatalf("response=%+v handled=%v, want synthesized LLM answer", response, handled)
	}
	if !memoryTool.called {
		t.Fatalf("memory.recent tool was not called")
	}
	if planningRuntime.req.Purpose != llmprovider.PurposeRouting || answerRuntime.req.Purpose != llmprovider.PurposeChat {
		t.Fatalf("planning=%+v answer=%+v, want routing planner then chat answerer", planningRuntime.req, answerRuntime.req)
	}
	if strings.Contains(response.Text, "Recent retained full-mode messages") || !strings.Contains(answerRuntime.req.Input, "Do not return the raw tool-result bullet list") {
		t.Fatalf("response=%q input=%q, want answerer synthesis rather than raw memory list", response.Text, answerRuntime.req.Input)
	}
}
