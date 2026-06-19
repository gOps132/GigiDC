package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gOps132/GigiDC/internal/contextbroker"
)

type RunSnapshot struct {
	Intent       string
	Results      []ToolResult
	ResponseText string
	ContextState contextbroker.SessionState
}

func (s RunSnapshot) copy() RunSnapshot {
	copied := RunSnapshot{
		Intent:       strings.TrimSpace(s.Intent),
		ResponseText: strings.TrimSpace(s.ResponseText),
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
	mu    sync.Mutex
	items map[string]RunSnapshot
}

func NewMemoryFollowUpStore() *MemoryFollowUpStore {
	return &MemoryFollowUpStore{items: map[string]RunSnapshot{}}
}

func (s *MemoryFollowUpStore) Load(ctx context.Context, request Request) (RunSnapshot, bool, error) {
	key := followUpKey(request)
	if key == "" {
		return RunSnapshot{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, ok := s.items[key]
	if !ok {
		return RunSnapshot{}, false, nil
	}
	copied := snapshot.copy()
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
		s.items = map[string]RunSnapshot{}
	}
	s.items[key] = snapshot
	return nil
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
