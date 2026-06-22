package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/memory"
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
	TraceSink        agent.TraceSink
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

type messageEmbedSender interface {
	ChannelMessageSendEmbed(channelID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

type messageEmbedEditor interface {
	ChannelMessageEditEmbed(channelID string, messageID string, embed *discordgo.MessageEmbed, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

type MessageRouter struct {
	botUserID      string
	handler        MessageHandler
	audit          AuditSink
	memoryIngestor GuildMemoryIngestor
	replyLatency   replyLatencyConfig
	liveDebug      AgentLiveDebugReader
}

type GuildMemoryIngestor interface {
	TryEnqueueMessage(event memory.MessageEvent) bool
}

func NewMessageRouter(botUserID string, handler MessageHandler, audit AuditSink, memoryIngestors ...GuildMemoryIngestor) (*MessageRouter, error) {
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
		botUserID:      botUserID,
		handler:        handler,
		audit:          audit,
		memoryIngestor: firstMemoryIngestor(memoryIngestors),
		replyLatency:   resolveReplyLatencyConfig(),
	}, nil
}

func (r *MessageRouter) SetReplyLatencyConfig(config ReplyLatencyConfig) {
	if r == nil {
		return
	}
	r.replyLatency = resolveReplyLatencyConfig(config)
}

func (r *MessageRouter) SetAgentLiveDebugStore(store AgentLiveDebugReader) {
	if r == nil {
		return
	}
	r.liveDebug = store
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
	r.enqueueMemory(event.Message)

	message, ok := r.route(event.Message)
	if !ok {
		return nil
	}

	liveSink := r.startLiveDebug(ctx, sender, message)
	message.TraceSink = liveSink
	startedAt := r.replyLatency.now()
	response, err := r.handler.HandleMessage(ctx, message)
	elapsed := r.replyLatency.now().Sub(startedAt)
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
	if message.Surface == MessageSurfaceGuildMention && r.replyLatency.enabled(ctx, message.GuildID) {
		content = appendReplyLatencySuffix(content, elapsed)
	}
	if liveSink != nil {
		liveSink.finish(ctx, content)
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

func (r *MessageRouter) startLiveDebug(ctx context.Context, sender messageSender, message Message) *channelAgentTraceSink {
	if r == nil || r.liveDebug == nil || message.Surface != MessageSurfaceGuildMention || strings.TrimSpace(message.GuildID) == "" {
		return nil
	}
	enabled, err := r.liveDebug.LiveDebugEnabled(ctx, message.GuildID, message.UserID)
	if err != nil || !enabled {
		return nil
	}
	embedSender, ok := sender.(messageEmbedSender)
	if !ok {
		return nil
	}
	embedEditor, ok := sender.(messageEmbedEditor)
	if !ok {
		return nil
	}
	sink := &channelAgentTraceSink{editor: embedEditor, channelID: message.ChannelID}
	sent, err := embedSender.ChannelMessageSendEmbed(message.ChannelID, formatAgentTraceEmbed(agent.TraceRun{Status: "running"}, "debug"))
	if err != nil || sent == nil {
		return nil
	}
	sink.messageID = sent.ID
	return sink
}

type channelAgentTraceSink struct {
	mu        sync.Mutex
	editor    messageEmbedEditor
	channelID string
	messageID string
	run       agent.TraceRun
}

func (s *channelAgentTraceSink) RecordTraceEvent(ctx context.Context, request agent.Request, event agent.TraceEvent) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.run.RunID == "" {
		s.run.RunID = event.RunID
	}
	s.run.GuildID = request.GuildID
	s.run.ChannelID = request.ChannelID
	s.run.ActorUserID = request.ActorUserID
	s.run.Surface = string(request.Surface)
	s.run.Status = event.Status
	s.run.Events = append(s.run.Events, event)
	s.mu.Unlock()
	s.update(ctx, "")
	return nil
}

func (s *channelAgentTraceSink) finish(ctx context.Context, answer string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.run.Status == "" || s.run.Status == "running" {
		s.run.Status = "succeeded"
	}
	s.mu.Unlock()
	s.update(ctx, answer)
}

func (s *channelAgentTraceSink) update(ctx context.Context, answer string) {
	if s == nil || s.editor == nil || strings.TrimSpace(s.channelID) == "" || strings.TrimSpace(s.messageID) == "" {
		return
	}
	s.mu.Lock()
	run := s.run
	s.mu.Unlock()
	embed := formatAgentTraceEmbed(run, "debug")
	if strings.TrimSpace(answer) != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Final answer", Value: boundEmbedValue(answer)})
	}
	_, _ = s.editor.ChannelMessageEditEmbed(s.channelID, s.messageID, embed)
}

func (r *MessageRouter) HandleMessageDelete(ctx context.Context, event *discordgo.MessageDelete) error {
	if event == nil || event.Message == nil {
		return nil
	}
	r.enqueueMemoryDelete(event.Message)
	return nil
}

func (r *MessageRouter) HandleMessageDeleteBulk(ctx context.Context, event *discordgo.MessageDeleteBulk) error {
	if event == nil || strings.TrimSpace(event.GuildID) == "" {
		return nil
	}
	for _, messageID := range event.Messages {
		r.enqueueMemoryDelete(&discordgo.Message{
			ID:        messageID,
			GuildID:   event.GuildID,
			ChannelID: event.ChannelID,
		})
	}
	return nil
}

func (r *MessageRouter) enqueueMemory(message *discordgo.Message) {
	if r == nil || r.memoryIngestor == nil || message == nil || message.Author == nil || strings.TrimSpace(message.GuildID) == "" {
		return
	}
	r.memoryIngestor.TryEnqueueMessage(memory.MessageEvent{
		MessageID:    message.ID,
		GuildID:      message.GuildID,
		ChannelID:    message.ChannelID,
		AuthorUserID: message.Author.ID,
		Content:      message.Content,
		CreatedAt:    message.Timestamp,
	})
}

func (r *MessageRouter) enqueueMemoryDelete(message *discordgo.Message) {
	if r == nil || r.memoryIngestor == nil || message == nil || strings.TrimSpace(message.GuildID) == "" || strings.TrimSpace(message.ID) == "" {
		return
	}
	r.memoryIngestor.TryEnqueueMessage(memory.MessageEvent{
		MessageID: message.ID,
		GuildID:   message.GuildID,
		ChannelID: message.ChannelID,
		Deleted:   true,
	})
}

func firstMemoryIngestor(ingestors []GuildMemoryIngestor) GuildMemoryIngestor {
	if len(ingestors) == 0 {
		return nil
	}
	return ingestors[0]
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
