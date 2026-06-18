package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestSemanticMemoryPlannerMapsCountJSON(t *testing.T) {
	runtime := &fakeRuntime{response: llm.TextResponse{Text: `{"intent":"count","target_user_id":"123","text":"postgres","scope":"this-channel"}`}}
	planner := SemanticMemoryPlanner{Runtime: runtime}

	got, ok, err := planner.Plan(context.Background(), SemanticMemoryInput{GuildID: "guild-id", ChannelID: "channel-id", ActorUserID: "actor-id", Text: `wassup how many times did <@123> mentioned "postgres"?`})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || got.Intent != MemoryIntentCount || got.TargetUserID != "123" || got.Text != "postgres" || got.Scope != "this-channel" {
		t.Fatalf("plan = %+v ok=%v, want count plan", got, ok)
	}
	if runtime.req.Purpose != llmprovider.PurposeRouting || runtime.req.Owner.GuildID != "guild-id" {
		t.Fatalf("runtime req = %+v, want routing request", runtime.req)
	}
}

func TestSemanticMemoryPlannerMapsSearchJSON(t *testing.T) {
	runtime := &fakeRuntime{response: llm.TextResponse{Text: `{"intent":"search","query":"postgres outage","limit":50,"scope":"this-channel"}`}}
	planner := SemanticMemoryPlanner{Runtime: runtime}

	got, ok, err := planner.Plan(context.Background(), SemanticMemoryInput{GuildID: "guild-id", ChannelID: "channel-id", ActorUserID: "actor-id", Text: "what did we say about postgres outage"})
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok || got.Intent != MemoryIntentSearch || got.Query != "postgres outage" || got.Limit != 25 {
		t.Fatalf("plan = %+v ok=%v, want clamped search plan", got, ok)
	}
}

func TestSemanticMemoryPlannerRejectsInvalidOutput(t *testing.T) {
	tests := []string{
		`{}`,
		`{"intent":"delete","query":"postgres","scope":"this-channel"}`,
		`{"intent":"search","query":"postgres","scope":"server"}`,
		`{"intent":"count","text":"postgres","scope":"this-channel"}`,
	}
	for _, output := range tests {
		t.Run(output, func(t *testing.T) {
			planner := SemanticMemoryPlanner{Runtime: &fakeRuntime{response: llm.TextResponse{Text: output}}}
			_, ok, err := planner.Plan(context.Background(), SemanticMemoryInput{GuildID: "guild-id", ChannelID: "channel-id", ActorUserID: "actor-id", Text: "memory request"})
			if err != nil {
				t.Fatalf("Plan returned error: %v", err)
			}
			if ok {
				t.Fatalf("ok = true, want rejected output")
			}
		})
	}
}

func TestSemanticMemoryPlannerPropagatesRuntimeError(t *testing.T) {
	planner := SemanticMemoryPlanner{Runtime: &fakeRuntime{err: errors.New("routing down")}}

	_, _, err := planner.Plan(context.Background(), SemanticMemoryInput{GuildID: "guild-id", ChannelID: "channel-id", ActorUserID: "actor-id", Text: "memory request"})
	if err == nil || !strings.Contains(err.Error(), "routing down") {
		t.Fatalf("error = %v, want runtime error", err)
	}
}
