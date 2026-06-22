package discord

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
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
	if findOption(last.Options, "view") == nil {
		t.Fatalf("last options = %+v, want view", last.Options)
	}
	if findOption(trace.Options, "live") == nil {
		t.Fatalf("trace options = %+v, want live command", trace.Options)
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
	response, err := agentTraceHandler(reader, nil)(context.Background(), agentTraceInteraction(nil))
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
	response, err := agentTraceHandler(reader, nil)(context.Background(), agentTraceInteraction([]InteractionOption{{Name: "visibility", Value: "public"}}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Ephemeral {
		t.Fatalf("response = %+v, want public visibility", response)
	}
}

func TestAgentTraceLastNoTrace(t *testing.T) {
	response, err := agentTraceHandler(&fakeAgentTraceReader{}, nil)(context.Background(), agentTraceInteraction(nil))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Content != "No agent trace found for you in this channel." || !response.Ephemeral {
		t.Fatalf("response = %+v, want private empty state", response)
	}
}

func TestAgentTraceLastDebugUsesEmbedDetails(t *testing.T) {
	reader := &fakeAgentTraceReader{run: agent.TraceRun{
		RunID:  "agentrun_1",
		Status: "succeeded",
		Events: []agent.TraceEvent{{
			Phase:      "tool",
			Status:     "succeeded",
			ToolName:   "web.search",
			Capability: "web.search",
			Details: map[string]string{
				"arg_query":      "news today",
				"result_count":   "0",
				"result_summary": `No search results found for query: "news today"`,
			},
		}, {
			Phase:  "answer",
			Status: "succeeded",
			Details: map[string]string{
				"answer_mode":     "fallback",
				"fallback_reason": "missing_required_citation",
			},
		}},
	}, ok: true}
	response, err := agentTraceHandler(reader, nil)(context.Background(), agentTraceInteraction([]InteractionOption{{Name: "view", Value: "debug"}}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !response.Ephemeral || len(response.Embeds) != 1 {
		t.Fatalf("response = %+v, want private embed", response)
	}
	rendered := traceEmbedText(response.Embeds[0])
	for _, want := range []string{"web.search", "news today", "results=`0`", "fallback=`missing_required_citation`"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("embed text = %q, want %q", rendered, want)
		}
	}
}

func TestAgentTraceLiveRunsRuntimeWithLiveSink(t *testing.T) {
	runtime := &fakeAgentTraceRuntime{}
	response := agentTraceLiveResponse(runtime, agentTraceLiveInteraction("search web"))
	if !response.Deferred || !response.Ephemeral || response.AfterRespond == nil {
		t.Fatalf("response = %+v, want deferred private live response", response)
	}
	editor := &fakeInteractionResponseEditor{}
	response.AfterRespond(context.Background(), editor)
	if runtime.request.Text != "search web" || runtime.request.TraceSink == nil {
		t.Fatalf("runtime request = %+v, want prompt and trace sink", runtime.request)
	}
	if len(editor.edits) == 0 {
		t.Fatal("expected live debug edits")
	}
	if got := traceEmbedText((*editor.edits[len(editor.edits)-1].Embeds)[0]); !strings.Contains(got, "web.search") || !strings.Contains(got, "answer") {
		t.Fatalf("final live embed = %q, want tool and answer trace", got)
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

func agentTraceLiveInteraction(prompt string) Interaction {
	return Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
		Name:      "agent",
		Options: []InteractionOption{{
			Name: "trace",
			Options: []InteractionOption{{
				Name: "live",
				Options: []InteractionOption{{
					Name:  "prompt",
					Value: prompt,
				}},
			}},
		}},
	}
}

func traceEmbedText(embed *discordgo.MessageEmbed) string {
	if embed == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(embed.Title)
	b.WriteString("\n")
	b.WriteString(embed.Description)
	for _, field := range embed.Fields {
		b.WriteString("\n")
		b.WriteString(field.Name)
		b.WriteString("\n")
		b.WriteString(field.Value)
	}
	return b.String()
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

type fakeAgentTraceRuntime struct {
	request agent.Request
}

func (r *fakeAgentTraceRuntime) Run(ctx context.Context, request agent.Request) (agent.Response, error) {
	r.request = request
	_ = request.TraceSink.RecordTraceEvent(ctx, request, agent.TraceEvent{
		RunID:    "agentrun_live",
		Phase:    "tool",
		Status:   "succeeded",
		ToolName: "web.search",
		Details:  map[string]string{"arg_query": "search web", "result_count": "1"},
	})
	_ = request.TraceSink.RecordTraceEvent(ctx, request, agent.TraceEvent{
		RunID:   "agentrun_live",
		Phase:   "answer",
		Status:  "succeeded",
		Details: map[string]string{"answer_mode": "fallback"},
	})
	return agent.Response{Text: "done", RunID: "agentrun_live", RunStatus: agent.RunStatusSucceeded}, nil
}

type fakeInteractionResponseEditor struct {
	edits []*discordgo.WebhookEdit
}

func (e *fakeInteractionResponseEditor) EditInteractionResponse(ctx context.Context, edit *discordgo.WebhookEdit) error {
	e.edits = append(e.edits, edit)
	return nil
}
