package agent

import (
	"context"
	"strconv"

	"github.com/gOps132/GigiDC/internal/assistant"
)

type SemanticMemoryPlanner interface {
	Plan(context.Context, assistant.SemanticMemoryInput) (assistant.MemoryPlan, bool, error)
}

type SemanticMemoryPlannerAdapter struct {
	Planner SemanticMemoryPlanner
}

func (a SemanticMemoryPlannerAdapter) Plan(ctx context.Context, request Request, specs []ToolSpec) (Plan, bool, error) {
	if a.Planner == nil {
		return Plan{}, false, nil
	}
	memoryPlan, ok, err := a.Planner.Plan(ctx, assistant.SemanticMemoryInput{
		GuildID:     request.GuildID,
		ChannelID:   request.ChannelID,
		ActorUserID: request.ActorUserID,
		Text:        request.Text,
	})
	if err != nil || !ok {
		return Plan{}, ok, err
	}
	switch memoryPlan.Intent {
	case assistant.MemoryIntentCount:
		return Plan{
			Intent: "memory.count",
			ToolCalls: []ToolCall{{
				Name: ToolMemoryCount,
				Args: map[string]string{
					"target_user_id": memoryPlan.TargetUserID,
					"text":           memoryPlan.Text,
				},
			}},
		}, true, nil
	case assistant.MemoryIntentSearch:
		return Plan{
			Intent: "memory.search",
			ToolCalls: []ToolCall{{
				Name: ToolMemorySearch,
				Args: map[string]string{
					"query": memoryPlan.Query,
					"limit": strconv.Itoa(memoryPlan.Limit),
				},
			}},
		}, true, nil
	default:
		return Plan{}, false, nil
	}
}
