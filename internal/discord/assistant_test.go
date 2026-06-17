package discord

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/assistant"
)

func TestAssistantFallbackHandlerAdaptsDiscordMessage(t *testing.T) {
	responder := &fakeAssistantResponder{response: assistant.Response{Text: "model reply"}}
	handler := AssistantFallbackHandler(responder, nil)

	got, err := handler.HandleMessage(context.Background(), Message{
		Surface:   MessageSurfaceGuildMention,
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "actor-id",
		Text:      "hello",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if got.Content != "model reply" {
		t.Fatalf("response = %+v, want assistant reply", got)
	}
	if responder.message.Surface != assistant.SurfaceGuildMention || responder.message.GuildID != "guild-id" || responder.message.ActorUserID != "actor-id" || responder.message.Text != "hello" {
		t.Fatalf("assistant message = %+v, want mapped Discord message", responder.message)
	}
}

func TestAssistantFallbackHandlerUsesFallbackWhenResponderMissing(t *testing.T) {
	handler := AssistantFallbackHandler(nil, MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		return MessageResponse{Content: "fallback"}, nil
	}))

	got, err := handler.HandleMessage(context.Background(), Message{Text: "hello"})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if got.Content != "fallback" {
		t.Fatalf("response = %+v, want fallback", got)
	}
}

func TestAssistantFallbackHandlerPropagatesResponderError(t *testing.T) {
	handler := AssistantFallbackHandler(&fakeAssistantResponder{err: errors.New("provider down")}, nil)

	_, err := handler.HandleMessage(context.Background(), Message{Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "provider down") {
		t.Fatalf("error = %v, want responder error", err)
	}
}

type fakeAssistantResponder struct {
	message  assistant.Message
	response assistant.Response
	err      error
}

func (r *fakeAssistantResponder) Reply(_ context.Context, message assistant.Message) (assistant.Response, error) {
	r.message = message
	return r.response, r.err
}
