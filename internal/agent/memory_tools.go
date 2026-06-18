package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/memory"
)

const (
	ToolMemoryCount  = "memory.count"
	ToolMemorySearch = "memory.search"
)

type MemoryStore interface {
	CountMentions(context.Context, memory.CountRequest) (memory.CountResult, error)
	SearchMessages(context.Context, memory.SearchRequest) ([]memory.SearchResult, error)
}

type MemoryCountTool struct {
	Store   MemoryStore
	Checker CapabilityChecker
}

func (t MemoryCountTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolMemoryCount,
		Description: "Count exact text mentions in retained current-channel guild memory.",
		Kind:        ToolKindRead,
		Capability:  "memory.read.guild",
	}
}

func (t MemoryCountTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if err := checkMemoryToolAccess(ctx, t.Checker, request); err != nil {
		return ToolResult{}, err
	}
	if t.Store == nil {
		return ToolResult{}, fmt.Errorf("memory store is required")
	}
	text := memory.NormalizeText(call.Args["text"])
	if text == "" {
		return ToolResult{}, fmt.Errorf("memory count text is required")
	}
	targetUserID := strings.TrimSpace(call.Args["target_user_id"])
	result, err := t.Store.CountMentions(ctx, memory.CountRequest{
		GuildID:      request.GuildID,
		ChannelID:    request.ChannelID,
		AuthorUserID: targetUserID,
		Text:         text,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Name:    ToolMemoryCount,
		Summary: formatMemoryCountSummary(targetUserID, text, result.Count),
		Data: map[string]string{
			"count": strconv.Itoa(result.Count),
			"scope": "this-channel",
		},
	}, nil
}

type MemorySearchTool struct {
	Store   MemoryStore
	Checker CapabilityChecker
}

func (t MemorySearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolMemorySearch,
		Description: "Search retained current-channel guild memory for exact normalized text.",
		Kind:        ToolKindRead,
		Capability:  "memory.read.guild",
	}
}

func (t MemorySearchTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if err := checkMemoryToolAccess(ctx, t.Checker, request); err != nil {
		return ToolResult{}, err
	}
	if t.Store == nil {
		return ToolResult{}, fmt.Errorf("memory store is required")
	}
	query := memory.NormalizeText(call.Args["query"])
	if query == "" {
		return ToolResult{}, fmt.Errorf("memory search query is required")
	}
	limit := parseLimit(call.Args["limit"], 5, 25)
	results, err := t.Store.SearchMessages(ctx, memory.SearchRequest{
		GuildID:   request.GuildID,
		ChannelID: request.ChannelID,
		Query:     query,
		Limit:     limit,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Name:    ToolMemorySearch,
		Summary: formatMemorySearchSummary(query, len(results)),
		Data: map[string]string{
			"matches": strconv.Itoa(len(results)),
			"scope":   "this-channel",
		},
	}, nil
}

func checkMemoryToolAccess(ctx context.Context, checker CapabilityChecker, request Request) error {
	if request.Surface != SurfaceGuildMention || request.GuildID == "" || request.ChannelID == "" {
		return fmt.Errorf("guild channel memory context is required")
	}
	if checker == nil {
		return fmt.Errorf("capability checker is required")
	}
	decision, err := checker.Check(ctx, capability.Subject{
		GuildID:          request.GuildID,
		UserID:           request.ActorUserID,
		RoleIDs:          request.RoleIDs,
		HasAdministrator: request.HasAdministrator,
	}, capability.Capability("memory.read.guild"))
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return fmt.Errorf("permission denied for memory")
	}
	return nil
}

func parseLimit(value string, fallback int, max int) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || limit < 1 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
}

func formatMemoryCountSummary(targetUserID string, text string, count int) string {
	if strings.TrimSpace(targetUserID) == "" {
		return fmt.Sprintf("Messages mentioned `%s` %d times in this channel.", safeInline(text), count)
	}
	return fmt.Sprintf("<@%s> mentioned `%s` %d times in this channel.", safeInline(targetUserID), safeInline(text), count)
}

func formatMemorySearchSummary(query string, count int) string {
	if count == 0 {
		return fmt.Sprintf("Memory search found no retained full-mode matches for `%s` in this channel.", safeInline(query))
	}
	return fmt.Sprintf("Memory search found %d retained full-mode matches for `%s` in this channel.", count, safeInline(query))
}

func safeInline(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "`", "'")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}
