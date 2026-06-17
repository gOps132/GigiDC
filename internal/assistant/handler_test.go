package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/llm"
	llmprovider "github.com/gOps132/GigiDC/internal/llm/provider"
)

func TestHandlerRepliesToGuildMentionWithGuildChatProfile(t *testing.T) {
	runtime := &fakeRuntime{response: llm.TextResponse{Text: "Hello from model."}}
	handler := Handler{Runtime: runtime, Instructions: "custom instructions", MaxOutputTokens: 44}

	got, err := handler.Reply(context.Background(), Message{
		Surface:     SurfaceGuildMention,
		GuildID:     "guild-id",
		ChannelID:   "channel-id",
		ActorUserID: "actor-id",
		Text:        " hello ",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if got.Text != "Hello from model." {
		t.Fatalf("response = %+v, want model text", got)
	}
	if runtime.req.Owner.OwnerType != llmprovider.OwnerGuild || runtime.req.Owner.GuildID != "guild-id" || runtime.req.Purpose != llmprovider.PurposeChat {
		t.Fatalf("runtime req = %+v, want guild chat", runtime.req)
	}
	if runtime.req.ActorUserID != "actor-id" || runtime.req.ChannelID != "channel-id" || runtime.req.Input != "hello" || runtime.req.Instructions != "custom instructions" || runtime.req.MaxOutputTokens != 44 {
		t.Fatalf("runtime req = %+v, want message context", runtime.req)
	}
}

func TestHandlerRecordsMetadataOnlyConversationTurns(t *testing.T) {
	recorder := &fakeConversationRecorder{}
	runtime := &fakeRuntime{response: llm.TextResponse{
		Text:       "Hello from model.",
		RequestID:  "request-id",
		ProviderID: "openai",
		ModelID:    "model-id",
	}}
	handler := Handler{Runtime: runtime, Recorder: recorder}

	got, err := handler.Reply(context.Background(), Message{
		Surface:     SurfaceGuildMention,
		GuildID:     "guild-id",
		ChannelID:   "channel-id",
		ActorUserID: "actor-id",
		Text:        "hello",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if got.Text != "Hello from model." {
		t.Fatalf("response = %+v, want model text", got)
	}
	if len(recorder.turns) != 2 {
		t.Fatalf("turns = %+v, want user and assistant turns", recorder.turns)
	}
	if recorder.turns[0].Role != TurnRoleUser || recorder.turns[0].ContentChars != 5 || recorder.turns[1].Role != TurnRoleAssistant || recorder.turns[1].ContentChars != 17 {
		t.Fatalf("turns = %+v, want metadata-only turn counts", recorder.turns)
	}
	for _, turn := range recorder.turns {
		if turn.RequestID != "request-id" || turn.ProviderID != "openai" || turn.ModelID != "model-id" || turn.Surface != SurfaceGuildMention {
			t.Fatalf("turn = %+v, want linked metadata", turn)
		}
	}
}

func TestHandlerDoesNotUseLLMForDMs(t *testing.T) {
	runtime := &fakeRuntime{}
	handler := Handler{Runtime: runtime}

	got, err := handler.Reply(context.Background(), Message{
		Surface:     SurfaceDM,
		ActorUserID: "actor-id",
		Text:        "hello",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if !strings.Contains(got.Text, "DM reasoning are not enabled yet") {
		t.Fatalf("response = %+v, want safe DM limitation", got)
	}
	if runtime.calls != 0 {
		t.Fatalf("runtime calls = %d, want none for DM", runtime.calls)
	}
}

func TestHandlerPropagatesRuntimeError(t *testing.T) {
	handler := Handler{Runtime: &fakeRuntime{err: errors.New("provider down")}}

	_, err := handler.Reply(context.Background(), Message{
		Surface:     SurfaceGuildMention,
		GuildID:     "guild-id",
		ActorUserID: "actor-id",
		Text:        "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "provider down") {
		t.Fatalf("error = %v, want runtime error", err)
	}
}

func TestHandlerTruncatesLongResponses(t *testing.T) {
	handler := Handler{
		Runtime:          &fakeRuntime{response: llm.TextResponse{Text: "abcdef"}},
		MaxResponseRunes: 3,
	}

	got, err := handler.Reply(context.Background(), Message{
		Surface:     SurfaceGuildMention,
		GuildID:     "guild-id",
		ActorUserID: "actor-id",
		Text:        "hello",
	})
	if err != nil {
		t.Fatalf("Reply returned error: %v", err)
	}
	if got.Text != "abc..." {
		t.Fatalf("response = %+v, want truncated text", got)
	}
}

func TestHandlerRejectsMissingRuntimeForGuildMention(t *testing.T) {
	handler := Handler{}

	_, err := handler.Reply(context.Background(), Message{
		Surface:     SurfaceGuildMention,
		GuildID:     "guild-id",
		ActorUserID: "actor-id",
		Text:        "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "assistant runtime is required") {
		t.Fatalf("error = %v, want runtime requirement", err)
	}
}

type fakeRuntime struct {
	calls    int
	req      llm.GenerateTextRequest
	response llm.TextResponse
	err      error
}

func (r *fakeRuntime) GenerateText(_ context.Context, req llm.GenerateTextRequest) (llm.TextResponse, error) {
	r.calls++
	r.req = req
	return r.response, r.err
}

type fakeConversationRecorder struct {
	turns []ConversationTurn
	err   error
}

func (r *fakeConversationRecorder) RecordTurn(_ context.Context, turn ConversationTurn) error {
	r.turns = append(r.turns, turn)
	return r.err
}
