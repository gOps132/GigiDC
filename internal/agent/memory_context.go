package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gOps132/GigiDC/internal/capability"
	"github.com/gOps132/GigiDC/internal/contextbroker"
	"github.com/gOps132/GigiDC/internal/memory"
)

const defaultMemoryContextLimit = 8
const maxMemoryContextLimit = 20

type MemoryContextProvider struct {
	Store   MemoryStore
	Checker CapabilityChecker
	Limit   int
}

func (p MemoryContextProvider) LoadContextSnippets(ctx context.Context, request Request) ([]contextbroker.Snippet, error) {
	if request.Surface != SurfaceGuildMention || request.GuildID == "" || request.ChannelID == "" || request.ContextScope != "channel" {
		return nil, nil
	}
	if p.Store == nil || p.Checker == nil {
		return nil, nil
	}
	decision, err := p.Checker.Check(ctx, capability.Subject{
		GuildID:          request.GuildID,
		UserID:           request.ActorUserID,
		RoleIDs:          request.RoleIDs,
		HasAdministrator: request.HasAdministrator,
	}, capability.Capability("memory.read.guild"))
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		return nil, nil
	}
	results, err := p.Store.RecentMessages(ctx, memory.RecentRequest{
		GuildID:   request.GuildID,
		ChannelID: request.ChannelID,
		Limit:     p.limit(),
	})
	if err != nil {
		return nil, err
	}
	return memoryResultsContextSnippets(results), nil
}

func (p MemoryContextProvider) limit() int {
	if p.Limit <= 0 {
		return defaultMemoryContextLimit
	}
	if p.Limit > maxMemoryContextLimit {
		return maxMemoryContextLimit
	}
	return p.Limit
}

func memoryResultsContextSnippets(results []memory.SearchResult) []contextbroker.Snippet {
	snippets := make([]contextbroker.Snippet, 0, len(results))
	for index, result := range results {
		text := strings.TrimSpace(result.Text)
		if text == "" {
			continue
		}
		messageID := strings.TrimSpace(result.MessageID)
		if messageID == "" {
			messageID = fmt.Sprintf("memory-%d", index+1)
		}
		createdAt := ""
		if !result.CreatedAt.IsZero() {
			createdAt = result.CreatedAt.UTC().Format(time.RFC3339)
		}
		snippets = append(snippets, contextbroker.Snippet{
			ID:        "message:" + messageID,
			Source:    "discord:channel:" + strings.TrimSpace(result.ChannelID),
			AuthorID:  strings.TrimSpace(result.AuthorUserID),
			Text:      formatMemoryContextText(result.AuthorUserID, text),
			CreatedAt: createdAt,
		})
	}
	return snippets
}

func formatMemoryContextText(authorID string, text string) string {
	authorID = strings.TrimSpace(authorID)
	text = strings.TrimSpace(text)
	if authorID == "" {
		return text
	}
	return "<@" + safeInline(authorID) + ">: " + text
}
