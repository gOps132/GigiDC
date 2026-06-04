package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type MessageSurface string

const (
	MessageSurfaceDM           MessageSurface = "dm"
	MessageSurfaceGuildMention MessageSurface = "guild_mention"
)

type Message struct {
	Surface          MessageSurface
	GuildID          string
	ChannelID        string
	UserID           string
	RoleIDs          []string
	HasAdministrator bool
	Text             string
	RawContent       string
}

type MessageResponse struct {
	Content string
}

type MessageHandler interface {
	HandleMessage(context.Context, Message) (MessageResponse, error)
}

type MessageHandlerFunc func(context.Context, Message) (MessageResponse, error)

func (f MessageHandlerFunc) HandleMessage(ctx context.Context, message Message) (MessageResponse, error) {
	return f(ctx, message)
}

type AuditStatus string

const (
	AuditStatusSucceeded AuditStatus = "succeeded"
	AuditStatusFailed    AuditStatus = "failed"
)

type AuditEvent struct {
	Kind      string
	Surface   MessageSurface
	GuildID   string
	ChannelID string
	UserID    string
	Status    AuditStatus
	LastError string
}

type AuditSink interface {
	RecordDiscordEvent(context.Context, AuditEvent) error
}

type noopAuditSink struct{}

func (noopAuditSink) RecordDiscordEvent(context.Context, AuditEvent) error {
	return nil
}

type messageSender interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

type MessageRouter struct {
	botUserID string
	handler   MessageHandler
	audit     AuditSink
}

func NewMessageRouter(botUserID string, handler MessageHandler, audit AuditSink) (*MessageRouter, error) {
	botUserID = strings.TrimSpace(botUserID)
	if botUserID == "" {
		return nil, fmt.Errorf("bot user ID is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("message handler is required")
	}
	if audit == nil {
		audit = noopAuditSink{}
	}
	return &MessageRouter{
		botUserID: botUserID,
		handler:   handler,
		audit:     audit,
	}, nil
}

func CoreMessageHandler() MessageHandler {
	return MessageHandlerFunc(func(ctx context.Context, message Message) (MessageResponse, error) {
		text := strings.ToLower(strings.TrimSpace(message.Text))
		if text == "ping" || text == "!ping" {
			return MessageResponse{Content: "pong"}, nil
		}
		return MessageResponse{Content: "Gigi is online. Rich chat is not wired yet."}, nil
	})
}

func (r *MessageRouter) HandleMessage(ctx context.Context, sender messageSender, event *discordgo.MessageCreate) error {
	if event == nil || event.Message == nil || event.Author == nil || event.Author.Bot {
		return nil
	}

	message, ok := r.route(event.Message)
	if !ok {
		return nil
	}

	response, err := r.handler.HandleMessage(ctx, message)
	content := strings.TrimSpace(response.Content)
	audit := AuditEvent{
		Kind:      "discord.message.routed",
		Surface:   message.Surface,
		GuildID:   message.GuildID,
		ChannelID: message.ChannelID,
		UserID:    message.UserID,
		Status:    AuditStatusSucceeded,
	}
	if err != nil {
		content = "Message handling failed."
		audit.Status = AuditStatusFailed
		audit.LastError = err.Error()
	}
	if content == "" {
		content = "ok"
	}

	auditErr := r.audit.RecordDiscordEvent(ctx, audit)
	if _, sendErr := sender.ChannelMessageSend(message.ChannelID, content); sendErr != nil {
		return fmt.Errorf("send discord message: %w", sendErr)
	}
	if auditErr != nil {
		return fmt.Errorf("record discord audit event: %w", auditErr)
	}
	return nil
}

func (r *MessageRouter) route(message *discordgo.Message) (Message, bool) {
	base := Message{
		GuildID:          message.GuildID,
		ChannelID:        message.ChannelID,
		UserID:           message.Author.ID,
		RoleIDs:          messageRoleIDs(message),
		HasAdministrator: messageHasAdministrator(message),
		RawContent:       message.Content,
	}

	if message.GuildID == "" {
		base.Surface = MessageSurfaceDM
		base.Text = strings.TrimSpace(message.Content)
		return base, true
	}

	text, ok := r.stripMention(message.Content)
	if !ok {
		return Message{}, false
	}
	base.Surface = MessageSurfaceGuildMention
	base.Text = text
	return base, true
}

func messageRoleIDs(message *discordgo.Message) []string {
	if message == nil || message.Member == nil {
		return nil
	}
	return append([]string(nil), message.Member.Roles...)
}

func messageHasAdministrator(message *discordgo.Message) bool {
	if message == nil || message.Member == nil {
		return false
	}
	return message.Member.Permissions&discordgo.PermissionAdministrator != 0
}

func (r *MessageRouter) stripMention(content string) (string, bool) {
	plain := "<@" + r.botUserID + ">"
	nick := "<@!" + r.botUserID + ">"
	if !strings.Contains(content, plain) && !strings.Contains(content, nick) {
		return "", false
	}
	replacer := strings.NewReplacer(plain, "", nick, "")
	return strings.TrimSpace(replacer.Replace(content)), true
}
