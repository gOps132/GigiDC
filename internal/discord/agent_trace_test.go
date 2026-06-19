package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/agent"
)

func TestAgentCommandsExposeTraceLast(t *testing.T) {
	commands := AgentCommands(&fakeAgentTraceReader{}, nil, nil)
	if len(commands) != 1 || commands[0].Name != "agent" {
		t.Fatalf("commands = %+v, want agent command", commands)
	}
	trace := findOption(commands[0].Options, "trace")
	if trace == nil {
		t.Fatalf("options = %+v, want trace group", commands[0].Options)
	}
	last := findOption(trace.Options, "last")
	if last == nil {
		t.Fatalf("trace options = %+v, want last command", trace.Options)
	}
	if findOption(last.Options, "visibility") == nil {
		t.Fatalf("last options = %+v, want visibility", last.Options)
	}
}

func TestAgentTraceLastFormatsScopedRun(t *testing.T) {
	reader := &fakeAgentTraceReader{run: agent.TraceRun{
		RunID:  "agentrun_1",
		Status: "succeeded",
		Events: []agent.TraceEvent{{
			StepIndex:   1,
			Phase:       "tool",
			Status:      "succeeded",
			ToolName:    "memory.recent",
			ToolKind:    "read",
			Capability:  "memory.read.guild",
			RoutingMode: "enabled",
		}},
	}, ok: true}
	response, err := agentTraceHandler(reader)(context.Background(), agentTraceInteraction(nil))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !response.Ephemeral || !strings.Contains(response.Content, "agentrun_1") || !strings.Contains(response.Content, "tool=`memory.recent`") {
		t.Fatalf("response = %+v, want private trace details", response)
	}
	if reader.query.GuildID != "guild-id" || reader.query.ChannelID != "channel-id" || reader.query.ActorUserID != "user-id" {
		t.Fatalf("query = %+v, want interaction scope", reader.query)
	}
}

func TestAgentTraceLastSupportsPublicVisibility(t *testing.T) {
	reader := &fakeAgentTraceReader{run: agent.TraceRun{RunID: "agentrun_1"}, ok: true}
	response, err := agentTraceHandler(reader)(context.Background(), agentTraceInteraction([]InteractionOption{{Name: "visibility", Value: "public"}}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Ephemeral {
		t.Fatalf("response = %+v, want public visibility", response)
	}
}

func TestAgentTraceLastNoTrace(t *testing.T) {
	response, err := agentTraceHandler(&fakeAgentTraceReader{})(context.Background(), agentTraceInteraction(nil))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Content != "No agent trace found for you in this channel." || !response.Ephemeral {
		t.Fatalf("response = %+v, want private empty state", response)
	}
}

func agentTraceInteraction(options []InteractionOption) Interaction {
	return Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
		Name:      "agent",
		Options: []InteractionOption{{
			Name: "trace",
			Options: []InteractionOption{{
				Name:    "last",
				Options: options,
			}},
		}},
	}
}

type fakeAgentTraceReader struct {
	query agent.TraceQuery
	run   agent.TraceRun
	ok    bool
}

func (r *fakeAgentTraceReader) LastTrace(ctx context.Context, query agent.TraceQuery) (agent.TraceRun, bool, error) {
	r.query = query
	return r.run, r.ok, nil
}
