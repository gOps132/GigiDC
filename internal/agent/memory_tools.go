package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/contextbroker"
	"github.com/gOps132/GigiDC/internal/memory"
)

const (
	ToolMemoryCount  = "memory.count"
	ToolMemorySearch = "memory.search"
	ToolMemoryRecent = "memory.recent"
)

type MemoryStore interface {
	CountMentions(context.Context, memory.CountRequest) (memory.CountResult, error)
	SearchMessages(context.Context, memory.SearchRequest) ([]memory.SearchResult, error)
	RecentMessages(context.Context, memory.RecentRequest) ([]memory.SearchResult, error)
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
		Summary: formatMemoryResultsSummary(fmt.Sprintf("Memory search for `%s`", safeInline(query)), results),
		Data: mergeToolData(map[string]string{
			"matches": strconv.Itoa(len(results)),
			"scope":   "this-channel",
		}, memoryResultsData(results)),
	}, nil
}

type MemoryRecentTool struct {
	Store   MemoryStore
	Checker CapabilityChecker
}

func (t MemoryRecentTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolMemoryRecent,
		Description: "Fetch recent retained current-channel guild memory messages.",
		Kind:        ToolKindRead,
		Capability:  "memory.read.guild",
	}
}

func (t MemoryRecentTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if err := checkMemoryToolAccess(ctx, t.Checker, request); err != nil {
		return ToolResult{}, err
	}
	if t.Store == nil {
		return ToolResult{}, fmt.Errorf("memory store is required")
	}
	limit := parseLimit(call.Args["limit"], 5, 25)
	results, err := t.Store.RecentMessages(ctx, memory.RecentRequest{
		GuildID:      request.GuildID,
		ChannelID:    request.ChannelID,
		AuthorUserID: strings.TrimSpace(call.Args["target_user_id"]),
		Limit:        limit,
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		Name:    ToolMemoryRecent,
		Summary: formatMemoryResultsSummary("Recent retained full-mode messages in this channel", results),
		Data: mergeToolData(map[string]string{
			"messages": strconv.Itoa(len(results)),
			"scope":    "this-channel",
		}, memoryResultsData(results)),
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

func formatMemoryResultsSummary(title string, results []memory.SearchResult) string {
	if len(results) == 0 {
		return title + ": no retained full-mode matches."
	}
	pack := memoryEvidencePack(results)
	lines := []string{fmt.Sprintf("%s (%d):", title, len(pack.Items))}
	for _, item := range pack.Items {
		lines = append(lines, fmt.Sprintf("- [%s] %s", item.Citation.Label, safeInlineLimit(item.Snippet.Text, 180)))
	}
	return strings.Join(lines, "\n")
}

func memoryResultsData(results []memory.SearchResult) map[string]string {
	data := map[string]string{}
	messageIDs := make([]string, 0, len(results))
	authors := make([]string, 0, len(results))
	sourceIDs := make([]string, 0, len(results))
	citationLabels := make([]string, 0, len(results))
	restoreHandles := make([]string, 0, len(results))
	retentionUntils := make([]string, 0, len(results))
	retrievedAts := make([]string, 0, len(results))
	var newest time.Time
	var oldest time.Time
	pack := memoryEvidencePack(results)
	for _, item := range pack.Items {
		if item.Citation.Label != "" {
			citationLabels = append(citationLabels, item.Citation.Label)
		}
		if item.SourceID != "" {
			sourceIDs = append(sourceIDs, item.SourceID)
		}
		if item.RestoreHandle != "" {
			restoreHandles = append(restoreHandles, item.RestoreHandle)
		}
	}
	for _, result := range results {
		if result.MessageID != "" {
			messageIDs = append(messageIDs, result.MessageID)
		}
		if result.AuthorUserID != "" {
			authors = append(authors, result.AuthorUserID)
		}
		if !result.CreatedAt.IsZero() {
			if newest.IsZero() || result.CreatedAt.After(newest) {
				newest = result.CreatedAt
			}
			if oldest.IsZero() || result.CreatedAt.Before(oldest) {
				oldest = result.CreatedAt
			}
		}
		if !result.RetentionUntil.IsZero() {
			retentionUntils = append(retentionUntils, result.RetentionUntil.UTC().Format(time.RFC3339))
		}
		if !result.RetrievedAt.IsZero() {
			retrievedAts = append(retrievedAts, result.RetrievedAt.UTC().Format(time.RFC3339))
		}
	}
	if len(messageIDs) > 0 {
		data["message_ids"] = strings.Join(messageIDs, ",")
	}
	if len(authors) > 0 {
		data["author_user_ids"] = strings.Join(authors, ",")
	}
	if len(citationLabels) > 0 {
		data["citation_labels"] = strings.Join(citationLabels, ",")
	}
	if len(sourceIDs) > 0 {
		data["source_ids"] = strings.Join(sourceIDs, ",")
	}
	if len(restoreHandles) > 0 {
		data["restore_handles"] = strings.Join(restoreHandles, ",")
	}
	if len(retentionUntils) > 0 {
		data["retention_untils"] = strings.Join(retentionUntils, ",")
	}
	if len(retrievedAts) > 0 {
		data["retrieved_at"] = strings.Join(retrievedAts, ",")
	}
	if !newest.IsZero() {
		data["newest_created_at"] = newest.UTC().Format(time.RFC3339)
	}
	if !oldest.IsZero() {
		data["oldest_created_at"] = oldest.UTC().Format(time.RFC3339)
	}
	return data
}

func memoryEvidencePack(results []memory.SearchResult) contextbroker.Pack {
	return contextbroker.BuildPack(contextbroker.BuildRequest{
		Snippets: memoryResultsContextSnippets(results),
		MaxChars: 2400,
	})
}

func mergeToolData(base map[string]string, extra map[string]string) map[string]string {
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func safeInline(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "`", "'")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}

func safeInlineLimit(value string, limit int) string {
	value = safeInline(value)
	if limit > 0 && len(value) > limit {
		return value[:limit] + "..."
	}
	return value
}
