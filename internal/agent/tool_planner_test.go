package agent

import (
	"context"
	"errors"
	"testing"
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

func TestHeuristicToolPlannerPlansUsage(t *testing.T) {
	request := agentTestRequest()
	request.Text = "how many LLM tokens used here?"

	plan, ok, err := HeuristicToolPlanner{}.Plan(context.Background(), request, []ToolSpec{{Name: ToolLLMUsageGuild}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.Intent != ToolLLMUsageGuild || plan.ToolCalls[0].Name != ToolLLMUsageGuild {
		t.Fatalf("plan=%+v ok=%v, want usage plan", plan, ok)
	}
}

func TestHeuristicToolPlannerPlansPermissionCheck(t *testing.T) {
	request := agentTestRequest()
	request.Text = "do I have `memory.read.guild`?"

	plan, ok, err := HeuristicToolPlanner{}.Plan(context.Background(), request, []ToolSpec{{Name: ToolPermissionsCheck}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.ToolCalls[0].Args["capability"] != "memory.read.guild" {
		t.Fatalf("plan=%+v ok=%v, want permission plan", plan, ok)
	}
}

func TestHeuristicToolPlannerPlansRecentMemory(t *testing.T) {
	request := agentTestRequest()
	request.Text = "summarize chat"

	plan, ok, err := HeuristicToolPlanner{}.Plan(context.Background(), request, []ToolSpec{{Name: ToolMemoryRecent}})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || plan.ToolCalls[0].Name != ToolMemoryRecent || plan.ToolCalls[0].Args["limit"] != "10" {
		t.Fatalf("plan=%+v ok=%v, want recent memory plan", plan, ok)
	}
}

func TestHeuristicToolPlannerSkipsUnavailableTool(t *testing.T) {
	request := agentTestRequest()
	request.Text = "show enabled plugins"

	plan, ok, err := HeuristicToolPlanner{}.Plan(context.Background(), request, nil)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if ok || plan.Intent != "" {
		t.Fatalf("plan=%+v ok=%v, want skip", plan, ok)
	}
}
