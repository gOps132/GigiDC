package discord

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/memory"
)

var memoryCountQuestionPattern = regexp.MustCompile(`(?i)\bhow many times did\s+<@!?([0-9]+)>\s+mention\s+["']?(.+?)["']?\s*\??$`)

type MemoryCounter interface {
	CountMentions(ctx context.Context, req memory.CountRequest) (memory.CountResult, error)
}

func MemoryQuestionHandler(counter MemoryCounter, checker CapabilityChecker, recorder AuditRecorder, fallback MessageHandler) MessageHandler {
	if fallback == nil {
		fallback = CoreMessageHandler()
	}
	return memoryQuestionHandler{
		counter:  counter,
		checker:  checker,
		recorder: recorder,
		fallback: fallback,
	}
}

type memoryQuestionHandler struct {
	counter  MemoryCounter
	checker  CapabilityChecker
	recorder AuditRecorder
	fallback MessageHandler
}

func (h memoryQuestionHandler) HandleMessage(ctx context.Context, message Message) (MessageResponse, error) {
	request, ok := parseMemoryCountQuestion(message)
	if !ok {
		return h.fallback.HandleMessage(ctx, message)
	}
	if message.Surface != MessageSurfaceGuildMention || strings.TrimSpace(message.GuildID) == "" {
		return h.fallback.HandleMessage(ctx, message)
	}
	if h.counter == nil {
		return MessageResponse{Content: "Memory is not configured yet."}, nil
	}
	if h.checker == nil {
		_ = h.record(ctx, message, request, audit.StatusFailed, "memory_checker_missing")
		return MessageResponse{Content: "Permission check failed."}, nil
	}
	decision, err := h.checker.Check(ctx, capability.Subject{
		GuildID:          message.GuildID,
		UserID:           message.UserID,
		RoleIDs:          message.RoleIDs,
		HasAdministrator: message.HasAdministrator,
	}, capability.Capability("memory.read.guild"))
	if err != nil {
		_ = h.record(ctx, message, request, audit.StatusFailed, string(decision.Reason))
		return MessageResponse{Content: "Permission check failed."}, nil
	}
	if !decision.Allowed {
		_ = h.record(ctx, message, request, audit.StatusDenied, string(decision.Reason))
		return MessageResponse{Content: "Permission denied for memory."}, nil
	}
	result, err := h.counter.CountMentions(ctx, memory.CountRequest{
		GuildID:      message.GuildID,
		ChannelID:    message.ChannelID,
		AuthorUserID: request.UserID,
		Text:         request.Text,
	})
	if err != nil {
		_ = h.record(ctx, message, request, audit.StatusFailed, "memory_count_failed")
		return MessageResponse{Content: "Memory lookup failed."}, nil
	}
	if err := h.record(ctx, message, request, audit.StatusSucceeded, ""); err != nil {
		return MessageResponse{}, err
	}
	return MessageResponse{Content: fmt.Sprintf("<@%s> mentioned `%s` %d times in this channel.", safeInline(request.UserID), safeInline(memory.NormalizeText(request.Text)), result.Count)}, nil
}

type memoryCountQuestion struct {
	UserID string
	Text   string
}

func parseMemoryCountQuestion(message Message) (memoryCountQuestion, bool) {
	matches := memoryCountQuestionPattern.FindStringSubmatch(strings.TrimSpace(message.Text))
	if len(matches) != 3 {
		return memoryCountQuestion{}, false
	}
	request := memoryCountQuestion{
		UserID: strings.TrimSpace(matches[1]),
		Text:   strings.Trim(strings.TrimSpace(matches[2]), `"'`),
	}
	if request.UserID == "" || memory.NormalizeText(request.Text) == "" {
		return memoryCountQuestion{}, false
	}
	return request, true
}

func (h memoryQuestionHandler) record(ctx context.Context, message Message, request memoryCountQuestion, status audit.Status, reason string) error {
	if h.recorder == nil || strings.TrimSpace(message.UserID) == "" {
		return nil
	}
	metadata := map[string]string{
		"question":       "count",
		"target_user_id": request.UserID,
		"scope":          "this-channel",
	}
	return h.recorder.Record(ctx, audit.Event{
		Kind:     "discord.memory.count",
		GuildID:  message.GuildID,
		ActorID:  message.UserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
	})
}
