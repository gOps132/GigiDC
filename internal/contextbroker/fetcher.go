package contextbroker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gOps132/GigiDC/internal/memory"
)

const SourceMemoryCurrentChannel = "memory.current_channel"

type RecentMemoryStore interface {
	RecentMessages(context.Context, memory.RecentRequest) ([]memory.SearchResult, error)
}

type ChannelRecentRequest struct {
	GuildID      string
	ChannelID    string
	AuthorUserID string
	Limit        int
	MaxChars     int
}

type ChannelRecentFetcher struct {
	Store RecentMemoryStore
}

func (f ChannelRecentFetcher) Fetch(ctx context.Context, request ChannelRecentRequest) (Pack, error) {
	request = normalizeChannelRecentRequest(request)
	if request.GuildID == "" {
		return Pack{}, fmt.Errorf("guild ID is required")
	}
	if request.ChannelID == "" {
		return Pack{}, fmt.Errorf("channel ID is required")
	}
	if f.Store == nil {
		return Pack{}, fmt.Errorf("memory store is required")
	}
	results, err := f.Store.RecentMessages(ctx, memory.RecentRequest{
		GuildID:      request.GuildID,
		ChannelID:    request.ChannelID,
		AuthorUserID: request.AuthorUserID,
		Limit:        request.Limit,
	})
	if err != nil {
		return Pack{}, err
	}
	snippets := make([]Snippet, 0, len(results))
	for _, result := range results {
		snippets = append(snippets, Snippet{
			ID:        result.MessageID,
			Source:    SourceMemoryCurrentChannel,
			ChannelID: result.ChannelID,
			AuthorID:  result.AuthorUserID,
			Text:      result.Text,
			CreatedAt: formatCreatedAt(result.CreatedAt),
		})
	}
	return PackSnippets(PackRequest{Snippets: snippets, MaxChars: request.MaxChars}), nil
}

func normalizeChannelRecentRequest(request ChannelRecentRequest) ChannelRecentRequest {
	request.GuildID = strings.TrimSpace(request.GuildID)
	request.ChannelID = strings.TrimSpace(request.ChannelID)
	request.AuthorUserID = strings.TrimSpace(request.AuthorUserID)
	if request.Limit <= 0 {
		request.Limit = 10
	}
	if request.Limit > 25 {
		request.Limit = 25
	}
	return request
}

func formatCreatedAt(createdAt time.Time) string {
	if createdAt.IsZero() {
		return ""
	}
	return createdAt.UTC().Format(time.RFC3339)
}
