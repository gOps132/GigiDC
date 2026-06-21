package contextbroker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gOps132/GigiDC/internal/memory"
)

func TestFetchChannelRecentPacksWithinBudget(t *testing.T) {
	created := time.Date(2026, 6, 19, 12, 30, 0, 0, time.UTC)
	store := &fakeRecentStore{results: []memory.SearchResult{
		{MessageID: "m1", ChannelID: "channel-id", AuthorUserID: "alice", Text: "postgres is neat", CreatedAt: created},
		{MessageID: "m2", ChannelID: "channel-id", AuthorUserID: "bob", Text: "redis is not postgres", CreatedAt: created.Add(time.Minute)},
	}}
	fetcher := ChannelRecentFetcher{Store: store}

	pack, err := fetcher.Fetch(context.Background(), ChannelRecentRequest{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		Limit:     10,
		MaxChars:  20,
	})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if store.request.GuildID != "guild-id" || store.request.ChannelID != "channel-id" || store.request.Limit != 10 {
		t.Fatalf("request=%+v, want guild/channel/limit", store.request)
	}
	if len(pack.Snippets) != 2 || !pack.Truncated || pack.Chars > 20 {
		t.Fatalf("pack=%+v, want 2 snippets truncated within budget", pack)
	}
	if got := pack.Snippets[0]; got.ID != "m1" || got.Source != "memory.current_channel" || got.ChannelID != "channel-id" || got.AuthorID != "alice" || got.CreatedAt != created.Format(time.RFC3339) {
		t.Fatalf("snippet=%+v, want mapped memory metadata", got)
	}
}

func TestFetchChannelRecentSkipsEmptyMessages(t *testing.T) {
	store := &fakeRecentStore{results: []memory.SearchResult{
		{MessageID: "empty", Text: "   "},
		{MessageID: "m1", Text: "hello"},
	}}
	fetcher := ChannelRecentFetcher{Store: store}

	pack, err := fetcher.Fetch(context.Background(), ChannelRecentRequest{GuildID: "guild-id", ChannelID: "channel-id"})
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if len(pack.Snippets) != 1 || pack.Snippets[0].ID != "m1" {
		t.Fatalf("pack=%+v, want only non-empty snippet", pack)
	}
}

func TestFetchChannelRecentRequiresGuildAndChannel(t *testing.T) {
	fetcher := ChannelRecentFetcher{Store: &fakeRecentStore{}}

	if _, err := fetcher.Fetch(context.Background(), ChannelRecentRequest{ChannelID: "channel-id"}); err == nil {
		t.Fatalf("Fetch without guild returned nil error")
	}
	if _, err := fetcher.Fetch(context.Background(), ChannelRecentRequest{GuildID: "guild-id"}); err == nil {
		t.Fatalf("Fetch without channel returned nil error")
	}
}

func TestFetchChannelRecentPropagatesStoreError(t *testing.T) {
	fetcher := ChannelRecentFetcher{Store: &fakeRecentStore{err: errors.New("database down")}}

	_, err := fetcher.Fetch(context.Background(), ChannelRecentRequest{GuildID: "guild-id", ChannelID: "channel-id"})
	if err == nil || err.Error() != "database down" {
		t.Fatalf("err=%v, want store error", err)
	}
}

type fakeRecentStore struct {
	request memory.RecentRequest
	results []memory.SearchResult
	err     error
}

func (s *fakeRecentStore) RecentMessages(ctx context.Context, req memory.RecentRequest) ([]memory.SearchResult, error) {
	s.request = req
	if s.err != nil {
		return nil, s.err
	}
	return s.results, nil
}
