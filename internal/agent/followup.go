package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gOps132/GigiDC/internal/contextbroker"
	"time"
)

type RunSnapshot struct {
	RunID        string
	Intent       string
	Results      []ToolResult
	ResponseText string
	ContextState contextbroker.SessionState
	CreatedAt    time.Time
}

func (s RunSnapshot) copy() RunSnapshot {
	copied := RunSnapshot{
		RunID:        strings.TrimSpace(s.RunID),
		Intent:       strings.TrimSpace(s.Intent),
		ResponseText: strings.TrimSpace(s.ResponseText),
		CreatedAt:    s.CreatedAt,
		Results:      make([]ToolResult, 0, len(s.Results)),
		ContextState: copyContextState(s.ContextState),
	}
	for _, result := range s.Results {
		copied.Results = append(copied.Results, copyToolResult(result))
	}
	return copied
}

func copyContextState(state contextbroker.SessionState) contextbroker.SessionState {
	copied := contextbroker.SessionState{Seen: map[string]string{}}
	for key, value := range state.Seen {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			copied.Seen[key] = value
		}
	}
	if len(copied.Seen) == 0 {
		copied.Seen = nil
	}
	return copied
}

func copyContextPack(pack contextbroker.Pack) contextbroker.Pack {
	copied := pack
	copied.Snippets = append([]contextbroker.Snippet(nil), pack.Snippets...)
	copied.Items = append([]contextbroker.ContextItem(nil), pack.Items...)
	copied.Omitted = append([]contextbroker.OmittedContext(nil), pack.Omitted...)
	copied.Invalidations = append([]contextbroker.ContextInvalidation(nil), pack.Invalidations...)
	copied.Citations = append([]contextbroker.Citation(nil), pack.Citations...)
	copied.NextState = copyContextState(pack.NextState)
	return copied
}

type FollowUpStore interface {
	Load(ctx context.Context, request Request) (RunSnapshot, bool, error)
	Save(ctx context.Context, request Request, snapshot RunSnapshot) error
}

type MemoryFollowUpStore struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	clock      func() time.Time
	sequence   uint64
	items      map[string]followUpEntry
}

func NewMemoryFollowUpStore() *MemoryFollowUpStore {
	return &MemoryFollowUpStore{
		ttl:        30 * time.Minute,
		maxEntries: 1000,
		clock:      time.Now,
		items:      map[string]followUpEntry{},
	}
}

func (s *MemoryFollowUpStore) Load(ctx context.Context, request Request) (RunSnapshot, bool, error) {
	key := followUpKey(request)
	if key == "" {
		return RunSnapshot{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.items[key]
	if !ok {
		return RunSnapshot{}, false, nil
	}
	if s.expired(entry, s.now()) {
		delete(s.items, key)
		return RunSnapshot{}, false, nil
	}
	copied := entry.Snapshot.copy()
	return copied, true, nil
}

func (s *MemoryFollowUpStore) Save(ctx context.Context, request Request, snapshot RunSnapshot) error {
	key := followUpKey(request)
	if key == "" {
		return nil
	}
	snapshot = snapshot.copy()
	if snapshot.Intent == "" && len(snapshot.Results) == 0 && snapshot.ResponseText == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = map[string]followUpEntry{}
	}
	now := s.now()
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = now
	}
	s.sequence++
	s.items[key] = followUpEntry{Snapshot: snapshot, SavedAt: now, Sequence: s.sequence}
	s.purgeLocked(now)
	s.trimLocked()
	return nil
}

func (s *MemoryFollowUpStore) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func (s *MemoryFollowUpStore) expired(entry followUpEntry, now time.Time) bool {
	if s.ttl <= 0 {
		return false
	}
	return !entry.SavedAt.IsZero() && now.Sub(entry.SavedAt) > s.ttl
}

func (s *MemoryFollowUpStore) purgeLocked(now time.Time) {
	for key, entry := range s.items {
		if s.expired(entry, now) {
			delete(s.items, key)
		}
	}
}

func (s *MemoryFollowUpStore) trimLocked() {
	if s.maxEntries <= 0 {
		return
	}
	for len(s.items) > s.maxEntries {
		var oldestKey string
		var oldest uint64
		for key, entry := range s.items {
			if oldestKey == "" || entry.Sequence < oldest {
				oldestKey = key
				oldest = entry.Sequence
			}
		}
		if oldestKey == "" {
			return
		}
		delete(s.items, oldestKey)
	}
}

type followUpEntry struct {
	Snapshot RunSnapshot
	SavedAt  time.Time
	Sequence uint64
}

func followUpKey(request Request) string {
	if strings.TrimSpace(request.GuildID) == "" || strings.TrimSpace(request.ChannelID) == "" || strings.TrimSpace(request.ActorUserID) == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", request.GuildID, request.ChannelID, request.ActorUserID)
}

func copyToolResult(result ToolResult) ToolResult {
	copied := ToolResult{
		Name:    strings.TrimSpace(result.Name),
		Summary: strings.TrimSpace(result.Summary),
	}
	if result.Data != nil {
		copied.Data = make(map[string]string, len(result.Data))
		for key, value := range result.Data {
			copied.Data[key] = value
		}
	}
	return copied
}
