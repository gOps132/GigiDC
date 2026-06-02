package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
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
