package discord

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/memory"
)

func TestMessageRouterRoutesDMs(t *testing.T) {
	var got Message
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		got = message
		return MessageResponse{Content: "dm-ok"}, nil
	}), nil)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	sender := &fakeMessageSender{}

	err = router.HandleMessage(context.Background(), sender, messageCreate("", "channel-id", "user-id", false, "hello gigi"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if got.Surface != MessageSurfaceDM || got.GuildID != "" || got.ChannelID != "channel-id" || got.UserID != "user-id" || got.Text != "hello gigi" {
		t.Fatalf("message context = %+v", got)
	}
	if sender.content != "dm-ok" || sender.channelID != "channel-id" {
		t.Fatalf("sent channel/content = %q/%q, want channel-id/dm-ok", sender.channelID, sender.content)
	}
}

func TestMessageRouterRoutesGuildMentions(t *testing.T) {
	var got Message
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		got = message
		return MessageResponse{Content: "mention-ok"}, nil
	}), nil)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	sender := &fakeMessageSender{}

	err = router.HandleMessage(context.Background(), sender, messageCreate("guild-id", "channel-id", "user-id", false, "<@bot-id> hello there"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if got.Surface != MessageSurfaceGuildMention || got.GuildID != "guild-id" || got.ChannelID != "channel-id" || got.UserID != "user-id" || got.Text != "hello there" {
		t.Fatalf("message context = %+v", got)
	}
	if sender.content != "mention-ok" {
		t.Fatalf("sent content = %q, want mention-ok", sender.content)
	}
}

func TestMessageRouterRoutesNicknameMentions(t *testing.T) {
	var got Message
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		got = message
		return MessageResponse{Content: "mention-ok"}, nil
	}), nil)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}

	err = router.HandleMessage(context.Background(), &fakeMessageSender{}, messageCreate("guild-id", "channel-id", "user-id", false, "<@!bot-id> ping"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if got.Text != "ping" {
		t.Fatalf("text = %q, want ping", got.Text)
	}
}

func TestMessageRouterIncludesGuildMemberAuthorityContext(t *testing.T) {
	var got Message
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		got = message
		return MessageResponse{Content: "mention-ok"}, nil
	}), nil)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	event := messageCreate("guild-id", "channel-id", "user-id", false, "<@bot-id> play song")
	event.Member = &discordgo.Member{
		Roles:       []string{"role-id"},
		Permissions: discordgo.PermissionAdministrator,
	}

	err = router.HandleMessage(context.Background(), &fakeMessageSender{}, event)
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if len(got.RoleIDs) != 1 || got.RoleIDs[0] != "role-id" || !got.HasAdministrator {
		t.Fatalf("authority context = roles:%+v admin:%v, want role-id/admin", got.RoleIDs, got.HasAdministrator)
	}
}

func TestMessageRouterIgnoresUnroutedMessages(t *testing.T) {
	calls := 0
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		calls++
		return MessageResponse{Content: "unexpected"}, nil
	}), nil)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	sender := &fakeMessageSender{}

	if err := router.HandleMessage(context.Background(), sender, messageCreate("guild-id", "channel-id", "user-id", false, "plain channel message")); err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if err := router.HandleMessage(context.Background(), sender, messageCreate("guild-id", "channel-id", "bot-user", true, "<@bot-id> hello")); err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if err := router.HandleMessage(context.Background(), sender, nil); err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("handler calls = %d, want 0", calls)
	}
	if sender.content != "" {
		t.Fatalf("sent content = %q, want none", sender.content)
	}
}

func TestMessageRouterEnqueuesGuildMemoryIngestForUnroutedGuildMessages(t *testing.T) {
	calls := 0
	ingestor := &fakeGuildMemoryIngestor{}
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		calls++
		return MessageResponse{Content: "unexpected"}, nil
	}), nil, ingestor)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	sender := &fakeMessageSender{}

	if err := router.HandleMessage(context.Background(), sender, messageCreate("guild-id", "channel-id", "user-id", false, "plain channel message")); err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if calls != 0 || sender.content != "" {
		t.Fatalf("calls/content = %d/%q, want no routed response", calls, sender.content)
	}
	if len(ingestor.events) != 1 || ingestor.events[0].GuildID != "guild-id" || ingestor.events[0].Content != "plain channel message" {
		t.Fatalf("ingest events = %+v, want guild memory event", ingestor.events)
	}
}

func TestMessageRouterEnqueuesGuildMemoryDeleteEvents(t *testing.T) {
	ingestor := &fakeGuildMemoryIngestor{}
	router, err := NewMessageRouter("bot-id", CoreMessageHandler(), nil, ingestor)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}

	if err := router.HandleMessageDelete(context.Background(), &discordgo.MessageDelete{Message: &discordgo.Message{ID: "message-id", GuildID: "guild-id", ChannelID: "channel-id"}}); err != nil {
		t.Fatalf("HandleMessageDelete returned error: %v", err)
	}
	if len(ingestor.events) != 1 || !ingestor.events[0].Deleted || ingestor.events[0].GuildID != "guild-id" || ingestor.events[0].MessageID != "message-id" {
		t.Fatalf("ingest events = %+v, want guild memory delete event", ingestor.events)
	}
}

func TestMessageRouterEnqueuesGuildMemoryBulkDeleteEvents(t *testing.T) {
	ingestor := &fakeGuildMemoryIngestor{}
	router, err := NewMessageRouter("bot-id", CoreMessageHandler(), nil, ingestor)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}

	if err := router.HandleMessageDeleteBulk(context.Background(), &discordgo.MessageDeleteBulk{
		Messages:  []string{"m1", "m2"},
		GuildID:   "guild-id",
		ChannelID: "channel-id",
	}); err != nil {
		t.Fatalf("HandleMessageDeleteBulk returned error: %v", err)
	}
	if len(ingestor.events) != 2 || !ingestor.events[0].Deleted || ingestor.events[1].MessageID != "m2" {
		t.Fatalf("ingest events = %+v, want bulk delete tombstones", ingestor.events)
	}
}

func TestMessageRouterDoesNotBlockOnFullMemoryQueue(t *testing.T) {
	ingestor := &fakeGuildMemoryIngestor{returnValue: false}
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		return MessageResponse{Content: "mention-ok"}, nil
	}), nil, ingestor)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- router.HandleMessage(context.Background(), &fakeMessageSender{}, messageCreate("guild-id", "channel-id", "user-id", false, "<@bot-id> hello"))
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("HandleMessage returned error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("HandleMessage blocked on memory ingest")
	}
}

func TestMessageRouterDoesNotIngestDMOrBotMessages(t *testing.T) {
	ingestor := &fakeGuildMemoryIngestor{}
	router, err := NewMessageRouter("bot-id", CoreMessageHandler(), nil, ingestor)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}

	if err := router.HandleMessage(context.Background(), &fakeMessageSender{}, messageCreate("", "channel-id", "user-id", false, "hello")); err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if err := router.HandleMessage(context.Background(), &fakeMessageSender{}, messageCreate("guild-id", "channel-id", "bot-user", true, "hello")); err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if len(ingestor.events) != 0 {
		t.Fatalf("ingest events = %+v, want none", ingestor.events)
	}
}

func TestMessageRouterFailsClosedWhenHandlerErrors(t *testing.T) {
	audit := &fakeAuditSink{}
	router, err := NewMessageRouter("bot-id", MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		return MessageResponse{}, errors.New("boom")
	}), audit)
	if err != nil {
		t.Fatalf("NewMessageRouter returned error: %v", err)
	}
	sender := &fakeMessageSender{}

	err = router.HandleMessage(context.Background(), sender, messageCreate("", "channel-id", "user-id", false, "hello"))
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if sender.content != "Message handling failed." {
		t.Fatalf("sent content = %q, want failure message", sender.content)
	}
	if len(audit.events) != 1 || audit.events[0].Status != AuditStatusFailed {
		t.Fatalf("audit events = %+v, want failed event", audit.events)
	}
}

func TestCoreMessageHandlerAnswersPing(t *testing.T) {
	response, err := CoreMessageHandler().HandleMessage(context.Background(), Message{Text: "ping"})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content != "pong" {
		t.Fatalf("response = %q, want pong", response.Content)
	}
}

func TestCoreMessageHandlerReportsPlaceholderForOtherMessages(t *testing.T) {
	response, err := CoreMessageHandler().HandleMessage(context.Background(), Message{Text: "hello"})
	if err != nil {
		t.Fatalf("HandleMessage returned error: %v", err)
	}
	if response.Content == "" || response.Content == "pong" {
		t.Fatalf("response = %q, want non-ping placeholder", response.Content)
	}
}

func messageCreate(guildID string, channelID string, userID string, bot bool, content string) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID:   guildID,
		ChannelID: channelID,
		Content:   content,
		Author:    &discordgo.User{ID: userID, Bot: bot},
	}}
}

type fakeMessageSender struct {
	channelID string
	content   string
	err       error
}

func (s *fakeMessageSender) ChannelMessageSend(channelID string, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	s.channelID = channelID
	s.content = content
	return &discordgo.Message{ChannelID: channelID, Content: content}, s.err
}

type fakeAuditSink struct {
	events []AuditEvent
	err    error
}

func (s *fakeAuditSink) RecordDiscordEvent(ctx context.Context, event AuditEvent) error {
	s.events = append(s.events, event)
	return s.err
}

type fakeGuildMemoryIngestor struct {
	events      []memory.MessageEvent
	returnValue bool
}

func (i *fakeGuildMemoryIngestor) TryEnqueueMessage(event memory.MessageEvent) bool {
	i.events = append(i.events, event)
	return i.returnValue
}
