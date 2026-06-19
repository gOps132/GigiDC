package discord

import (
	"context"
	"testing"

	"github.com/gOps132/GigiDC/internal/agent"
)

func TestAskCommandShape(t *testing.T) {
	command := AskCommand(&fakeAgentRuntime{})

	if command.Name != "ask" || command.Handle == nil {
		t.Fatalf("command = %+v, want ask handler", command)
	}
	for _, name := range []string{"question", "context", "visibility"} {
		if findOption(command.Options, name) == nil {
			t.Fatalf("options = %+v, want %s", command.Options, name)
		}
	}
}

func TestAskCommandRoutesToAgentRuntime(t *testing.T) {
	runtime := &fakeAgentRuntime{response: agent.Response{Text: "answer"}}
	handler := askHandler(runtime)

	response, err := handler(context.Background(), Interaction{
		GuildID:          "guild-id",
		ChannelID:        "channel-id",
		UserID:           "user-id",
		RoleIDs:          []string{"role-id"},
		HasAdministrator: true,
		Options: []InteractionOption{
			{Name: "question", Value: "what happened?"},
			{Name: "context", Value: "channel"},
			{Name: "visibility", Value: "private"},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Content != "answer" || !response.Ephemeral {
		t.Fatalf("response = %+v, want private answer", response)
	}
	if runtime.request.Text != "what happened?" || runtime.request.ContextScope != "channel" {
		t.Fatalf("request = %+v, want question/context", runtime.request)
	}
	if runtime.request.GuildID != "guild-id" || runtime.request.ActorUserID != "user-id" || !runtime.request.HasAdministrator {
		t.Fatalf("request = %+v, want interaction context", runtime.request)
	}
}

func TestAskCommandDefaultsToNoContextPublic(t *testing.T) {
	runtime := &fakeAgentRuntime{response: agent.Response{Text: "answer"}}

	response, err := askHandler(runtime)(context.Background(), Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
		Options:   []InteractionOption{{Name: "question", Value: "hello"}},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Ephemeral {
		t.Fatalf("response = %+v, want public default", response)
	}
	if runtime.request.ContextScope != "none" {
		t.Fatalf("context = %q, want none", runtime.request.ContextScope)
	}
}

func TestAskCommandDefaultsChannelContextToPrivate(t *testing.T) {
	runtime := &fakeAgentRuntime{response: agent.Response{Text: "memory answer"}}

	response, err := askHandler(runtime)(context.Background(), Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
		Options: []InteractionOption{
			{Name: "question", Value: "what did we say?"},
			{Name: "context", Value: "channel"},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !response.Ephemeral {
		t.Fatalf("response = %+v, want private channel-context default", response)
	}
	if runtime.request.ContextScope != "channel" {
		t.Fatalf("context = %q, want channel", runtime.request.ContextScope)
	}
}

func TestAskCommandHonorsPrivateAgentResponse(t *testing.T) {
	runtime := &fakeAgentRuntime{response: agent.Response{Text: "private answer", Visibility: agent.VisibilityPrivate}}

	response, err := askHandler(runtime)(context.Background(), Interaction{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "user-id",
		Options: []InteractionOption{
			{Name: "question", Value: "what did we say?"},
			{Name: "visibility", Value: "public"},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !response.Ephemeral {
		t.Fatalf("response = %+v, want private agent response to force ephemeral", response)
	}
}

func TestAskCommandRequiresQuestion(t *testing.T) {
	response, err := askHandler(&fakeAgentRuntime{})(context.Background(), Interaction{})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if response.Content != "Question is required." || !response.Ephemeral {
		t.Fatalf("response = %+v, want private validation", response)
	}
}
