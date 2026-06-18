package discord

import (
	"context"
	"testing"

	"github.com/gOps132/GigiDC/internal/agent"
)

func TestAgentMessageHandlerAdaptsDiscordMessage(t *testing.T) {
	runtime := &fakeAgentRuntime{response: agent.Response{Text: "agent answer"}}
	handler := AgentMessageHandler(runtime, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface:          MessageSurfaceGuildMention,
		GuildID:          "guild-id",
		ChannelID:        "channel-id",
		UserID:           "user-id",
		RoleIDs:          []string{"role-1"},
		HasAdministrator: true,
		Text:             "hello",
		RawContent:       "<@bot> hello",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "agent answer" {
		t.Fatalf("response = %+v, want agent answer", response)
	}
	if runtime.request.GuildID != "guild-id" || runtime.request.ActorUserID != "user-id" || runtime.request.RawText != "<@bot> hello" {
		t.Fatalf("request = %+v, want adapted IDs/raw text", runtime.request)
	}
	if len(runtime.request.RoleIDs) != 1 || runtime.request.RoleIDs[0] != "role-1" || !runtime.request.HasAdministrator {
		t.Fatalf("request = %+v, want authority context", runtime.request)
	}
}

func TestAgentMessageHandlerUsesFallbackWhenRuntimeMissing(t *testing.T) {
	response, err := AgentMessageHandler(nil, CoreMessageHandler()).HandleMessage(context.Background(), Message{Text: "ping"})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "pong" {
		t.Fatalf("response = %+v, want fallback pong", response)
	}
}

func TestAgentMessageHandlerUsesFallbackWhenAgentEmpty(t *testing.T) {
	handler := AgentMessageHandler(&fakeAgentRuntime{}, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), Message{Text: "ping"})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "pong" {
		t.Fatalf("response = %+v, want fallback pong", response)
	}
}

type fakeAgentRuntime struct {
	request  agent.Request
	response agent.Response
	err      error
}

func (r *fakeAgentRuntime) Run(ctx context.Context, request agent.Request) (agent.Response, error) {
	r.request = request
	return r.response, r.err
}
