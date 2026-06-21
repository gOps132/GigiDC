package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gOps132/GigiDC/internal/capability"
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
		Description: "Count exact text mentions in retained current-channel guild memory. Optional arguments: target_user_id, start_date (YYYY-MM-DD), end_date (YYYY-MM-DD).",
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
	startDate, err := extractStartDate(call.Args)
	if err != nil {
		return ToolResult{}, err
	}
	endDate, err := extractEndDate(call.Args)
	if err != nil {
		return ToolResult{}, err
	}
	result, err := t.Store.CountMentions(ctx, memory.CountRequest{
		GuildID:      request.GuildID,
		ChannelID:    request.ChannelID,
		AuthorUserID: targetUserID,
		Text:         text,
		StartDate:    startDate,
		EndDate:      endDate,
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
		Description: "Search retained current-channel guild memory for exact normalized text. Optional arguments: target_user_id, start_date (YYYY-MM-DD), end_date (YYYY-MM-DD), limit.",
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
	targetUserID := strings.TrimSpace(call.Args["target_user_id"])
	limit := parseLimit(call.Args["limit"], 5, 25)
	startDate, err := extractStartDate(call.Args)
	if err != nil {
		return ToolResult{}, err
	}
	endDate, err := extractEndDate(call.Args)
	if err != nil {
		return ToolResult{}, err
	}
	results, err := t.Store.SearchMessages(ctx, memory.SearchRequest{
		GuildID:      request.GuildID,
		ChannelID:    request.ChannelID,
		AuthorUserID: targetUserID,
		Query:        query,
		Limit:        limit,
		StartDate:    startDate,
		EndDate:      endDate,
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
		Description: "Fetch recent retained current-channel guild memory messages. Optional arguments: target_user_id, start_date (YYYY-MM-DD), end_date (YYYY-MM-DD), limit.",
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
	targetUserID := strings.TrimSpace(call.Args["target_user_id"])
	startDate, err := extractStartDate(call.Args)
	if err != nil {
		return ToolResult{}, err
	}
	endDate, err := extractEndDate(call.Args)
	if err != nil {
		return ToolResult{}, err
	}
	results, err := t.Store.RecentMessages(ctx, memory.RecentRequest{
		GuildID:      request.GuildID,
		ChannelID:    request.ChannelID,
		AuthorUserID: targetUserID,
		Limit:        limit,
		StartDate:    startDate,
		EndDate:      endDate,
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

// parseStartDate parses a memory filter start timestamp.
//
// Accepted formats:
//   - RFC3339 (e.g. "2026-06-22T10:00:00Z") — used verbatim.
//   - YYYY-MM-DD (e.g. "2026-06-22") — midnight UTC, inclusive lower bound.
//
// An empty value yields the zero time, which downstream SQL treats as
// "no lower bound".
func parseStartDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid start date format: %s", value)
}

// parseEndDate parses a memory filter end timestamp.
//
// Accepted formats:
//   - RFC3339 — used verbatim as an exact upper bound.
//   - YYYY-MM-DD — extended by 24h-1s so the whole day is inclusive.
//
// Empty value yields the zero time, which downstream SQL treats as
// "no upper bound".
func parseEndDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Add(24*time.Hour - time.Second), nil
	}
	return time.Time{}, fmt.Errorf("invalid end date format: %s", value)
}

// extractStartDate reads the lower-bound date from the first matching arg key.
// Aliases (start_date, after_date, since) let the LLM use natural phrasing.
func extractStartDate(args map[string]string) (time.Time, error) {
	for _, key := range []string{"start_date", "after_date", "since"} {
		if val, ok := args[key]; ok && strings.TrimSpace(val) != "" {
			return parseStartDate(val)
		}
	}
	return time.Time{}, nil
}

// extractEndDate reads the upper-bound date from the first matching arg key.
// Aliases (end_date, before_date, until) let the LLM use natural phrasing.
func extractEndDate(args map[string]string) (time.Time, error) {
	for _, key := range []string{"end_date", "before_date", "until"} {
		if val, ok := args[key]; ok && strings.TrimSpace(val) != "" {
			return parseEndDate(val)
		}
	}
	return time.Time{}, nil
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
	lines := []string{fmt.Sprintf("%s (%d):", title, len(results))}
	for _, result := range results {
		lines = append(lines, fmt.Sprintf("- <@%s>: %s", safeInline(result.AuthorUserID), safeInlineLimit(result.Text, 140)))
	}
	return strings.Join(lines, "\n")
}

func memoryResultsData(results []memory.SearchResult) map[string]string {
	data := map[string]string{}
	messageIDs := make([]string, 0, len(results))
	authors := make([]string, 0, len(results))
	snippets := make([]string, 0, len(results))
	var newest time.Time
	var oldest time.Time
	for _, result := range results {
		if result.MessageID != "" {
			messageIDs = append(messageIDs, result.MessageID)
		}
		if result.AuthorUserID != "" {
			authors = append(authors, result.AuthorUserID)
		}
		if result.Text != "" {
			snippets = append(snippets, safeInlineLimit(result.Text, 180))
		}
		if !result.CreatedAt.IsZero() {
			if newest.IsZero() || result.CreatedAt.After(newest) {
				newest = result.CreatedAt
			}
			if oldest.IsZero() || result.CreatedAt.Before(oldest) {
				oldest = result.CreatedAt
			}
		}
	}
	if len(messageIDs) > 0 {
		data["message_ids"] = strings.Join(messageIDs, ",")
	}
	if len(authors) > 0 {
		data["author_user_ids"] = strings.Join(authors, ",")
	}
	if len(snippets) > 0 {
		data["snippets"] = strings.Join(snippets, "\n")
	}
	if !newest.IsZero() {
		data["newest_created_at"] = newest.UTC().Format(time.RFC3339)
	}
	if !oldest.IsZero() {
		data["oldest_created_at"] = oldest.UTC().Format(time.RFC3339)
	}
	return data
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
