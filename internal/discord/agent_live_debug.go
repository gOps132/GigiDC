package discord

import (
	"context"
	"strings"
	"sync"
)

type AgentLiveDebugReader interface {
	LiveDebugEnabled(context.Context, string, string) (bool, error)
}

type AgentLiveDebugStore interface {
	AgentLiveDebugReader
	SetLiveDebugEnabled(context.Context, string, string, bool) error
}

type MemoryAgentLiveDebugStore struct {
	mu      sync.Mutex
	enabled map[string]bool
}

func NewMemoryAgentLiveDebugStore() *MemoryAgentLiveDebugStore {
	return &MemoryAgentLiveDebugStore{enabled: map[string]bool{}}
}

func (s *MemoryAgentLiveDebugStore) LiveDebugEnabled(ctx context.Context, guildID string, userID string) (bool, error) {
	if s == nil {
		return false, nil
	}
	key := agentLiveDebugKey(guildID, userID)
	if key == "" {
		return false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled[key], nil
}

func (s *MemoryAgentLiveDebugStore) SetLiveDebugEnabled(ctx context.Context, guildID string, userID string, enabled bool) error {
	if s == nil {
		return nil
	}
	key := agentLiveDebugKey(guildID, userID)
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled == nil {
		s.enabled = map[string]bool{}
	}
	if enabled {
		s.enabled[key] = true
		return nil
	}
	delete(s.enabled, key)
	return nil
}

func agentLiveDebugKey(guildID string, userID string) string {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return ""
	}
	return guildID + ":" + userID
}
