package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
	"github.com/gOps132/GigiDC/internal/plugins"
)

func TestSemanticPluginPlannerBuildsManifestGroundedPlan(t *testing.T) {
	runtime := &fakeRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!play","arguments":"never gonna give you up"}`}}
	planner := SemanticPluginPlanner{Runtime: runtime}

	got, ok, err := planner.Plan(context.Background(), semanticPluginInputForLLM())
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok {
		t.Fatal("Plan returned no semantic plan")
	}
	if got.Command != "!play never gonna give you up" || got.Manifest.ID != "jockie-music" || got.Trigger.Value != "!play" {
		t.Fatalf("plan = %+v, want manifest-grounded command", got)
	}
	if runtime.req.Purpose != llmprovider.PurposeRouting || runtime.req.Owner.GuildID != "guild-id" || runtime.req.ActorUserID != "actor-id" {
		t.Fatalf("runtime req = %+v, want routing request", runtime.req)
	}
	if !strings.Contains(runtime.req.Input, "User message:") || !strings.Contains(runtime.req.Input, "plugin_id: jockie-music") {
		t.Fatalf("runtime input = %q, want semantic prompt", runtime.req.Input)
	}
}

func TestSemanticPluginPlannerNormalizesPolitePrefixBeforeLLM(t *testing.T) {
	runtime := &fakeRuntime{err: errors.New("runtime must not be called")}
	planner := SemanticPluginPlanner{Runtime: runtime}
	input := validSemanticPluginInput()
	input.Text = "please play never gonna give you up"

	got, ok, err := planner.Plan(context.Background(), input)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if !ok {
		t.Fatal("Plan returned no polite-prefix plan")
	}
	if got.Command != "!play never gonna give you up" {
		t.Fatalf("plan = %+v, want normalized command", got)
	}
	if runtime.calls != 0 {
		t.Fatalf("runtime calls = %d, want deterministic plan before LLM", runtime.calls)
	}
}

func TestSemanticPluginPlannerRejectsInventedTrigger(t *testing.T) {
	runtime := &fakeRuntime{response: llm.TextResponse{Text: `{"plugin_id":"jockie-music","trigger":"!ban","arguments":"someone"}`}}
	planner := SemanticPluginPlanner{Runtime: runtime}

	_, ok, err := planner.Plan(context.Background(), semanticPluginInputForLLM())
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if ok {
		t.Fatal("Plan accepted invented trigger")
	}
}

func TestSemanticPluginPlannerReturnsNoPlanForEmptyJSON(t *testing.T) {
	runtime := &fakeRuntime{response: llm.TextResponse{Text: `{}`}}
	planner := SemanticPluginPlanner{Runtime: runtime}

	_, ok, err := planner.Plan(context.Background(), semanticPluginInputForLLM())
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if ok {
		t.Fatal("Plan accepted empty proposal")
	}
}

func TestSemanticPluginPlannerPropagatesRuntimeError(t *testing.T) {
	planner := SemanticPluginPlanner{Runtime: &fakeRuntime{err: errors.New("routing down")}}

	_, _, err := planner.Plan(context.Background(), semanticPluginInputForLLM())
	if err == nil || !strings.Contains(err.Error(), "routing down") {
		t.Fatalf("error = %v, want runtime failure", err)
	}
}

func semanticPluginInputForLLM() SemanticPluginInput {
	input := validSemanticPluginInput()
	input.Text = "start the song never gonna give you up"
	return input
}

func validSemanticPluginInput() SemanticPluginInput {
	return SemanticPluginInput{
		GuildID:     "guild-id",
		ChannelID:   "channel-id",
		ActorUserID: "actor-id",
		Text:        "play never gonna give you up",
		Manifests: []plugins.Manifest{{
			ID:      "jockie-music",
			Name:    "Jockie Music",
			Version: "1.0.0",
			Triggers: []plugins.Trigger{{
				Kind:  "prefix",
				Value: "!play",
			}},
			Surfaces:    []string{"guild_text"},
			Permissions: []string{"plugin.install"},
		}},
	}
}
