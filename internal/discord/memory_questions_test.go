package discord

import (
	"context"
	"testing"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/memory"
)

func TestMemoryQuestionHandlerAnswersGuildMentionCount(t *testing.T) {
	counter := &fakeMemoryCounter{result: memory.CountResult{Count: 4}}
	checker := &fakeCapabilityChecker{decision: capability.Decision{Allowed: true, Capability: "memory.read.guild", Reason: capability.ReasonAdminOverride}}
	recorder := &fakeAuditRecorder{}
	handler := MemoryQuestionHandler(counter, checker, recorder, CoreMessageHandler())

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface:   MessageSurfaceGuildMention,
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		UserID:    "actor-id",
		Text:      `how many times did <@1234> mention "Postgres"?`,
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "<@1234> mentioned `postgres` 4 times in this channel." {
		t.Fatalf("response = %q, want count answer", response.Content)
	}
	if counter.req.GuildID != "guild-id" || counter.req.ChannelID != "channel-id" || counter.req.AuthorUserID != "1234" || counter.req.Text != "Postgres" {
		t.Fatalf("count req = %+v, want parsed guild/channel/user/text", counter.req)
	}
	if checker.subject.GuildID != "guild-id" || checker.subject.UserID != "actor-id" {
		t.Fatalf("checker subject = %+v, want requester subject", checker.subject)
	}
	if len(recorder.events) != 1 || recorder.events[0].Kind != "discord.memory.count" || recorder.events[0].Status != audit.StatusSucceeded || recorder.events[0].Metadata["target_user_id"] != "1234" {
		t.Fatalf("audit events = %+v, want safe memory count audit", recorder.events)
	}
	if _, ok := recorder.events[0].Metadata["text"]; ok {
		t.Fatalf("audit metadata = %+v, must not store query text", recorder.events[0].Metadata)
	}
}

func TestMemoryQuestionHandlerFallsBackForNonMemoryQuestion(t *testing.T) {
	handler := MemoryQuestionHandler(&fakeMemoryCounter{}, &fakeCapabilityChecker{}, nil, MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		return MessageResponse{Content: "fallback"}, nil
	}))

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "actor-id",
		Text:    "hello",
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "fallback" {
		t.Fatalf("response = %q, want fallback", response.Content)
	}
}

func TestMemoryQuestionHandlerDeniesMissingCapability(t *testing.T) {
	recorder := &fakeAuditRecorder{}
	handler := MemoryQuestionHandler(
		&fakeMemoryCounter{},
		&fakeCapabilityChecker{decision: capability.Decision{Allowed: false, Capability: "memory.read.guild", Reason: capability.ReasonMissingCapability}},
		recorder,
		CoreMessageHandler(),
	)

	response, err := handler.HandleMessage(context.Background(), Message{
		Surface: MessageSurfaceGuildMention,
		GuildID: "guild-id",
		UserID:  "actor-id",
		Text:    `how many times did <@1234> mention "postgres"?`,
	})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "Permission denied for memory." {
		t.Fatalf("response = %q, want permission denial", response.Content)
	}
	if len(recorder.events) != 1 || recorder.events[0].Status != audit.StatusDenied {
		t.Fatalf("audit events = %+v, want denied memory audit", recorder.events)
	}
}

type fakeMemoryCounter struct {
	req    memory.CountRequest
	result memory.CountResult
	err    error
}

func (c *fakeMemoryCounter) CountMentions(ctx context.Context, req memory.CountRequest) (memory.CountResult, error) {
	c.req = req
	return c.result, c.err
}
