package discord

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

type GuildReplyLatencyStore interface {
	GuildReplyLatencyEnabled(context.Context, string) (bool, error)
	SetGuildReplyLatencyEnabled(context.Context, string, bool) error
}

type replyLatencyExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type replyLatencyQueryRowDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) replyLatencyScanner
}

type sqlReplyLatencyQueryRowDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type replyLatencyScanner interface {
	Scan(dest ...any) error
}

type ReplyLatencyConfig struct {
	Store GuildReplyLatencyStore
	Clock func() time.Time
}

type MemoryGuildReplyLatencyStore struct {
	mu      sync.RWMutex
	enabled map[string]bool
}

func NewMemoryGuildReplyLatencyStore() *MemoryGuildReplyLatencyStore {
	return &MemoryGuildReplyLatencyStore{enabled: make(map[string]bool)}
}

func (s *MemoryGuildReplyLatencyStore) GuildReplyLatencyEnabled(_ context.Context, guildID string) (bool, error) {
	guildID = strings.TrimSpace(guildID)
	if s == nil || guildID == "" {
		return false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled[guildID], nil
}

func (s *MemoryGuildReplyLatencyStore) SetGuildReplyLatencyEnabled(_ context.Context, guildID string, enabled bool) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("guild ID is required")
	}
	if s == nil {
		return fmt.Errorf("guild reply latency store is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled {
		s.enabled[guildID] = true
		return nil
	}
	delete(s.enabled, guildID)
	return nil
}

type SQLGuildReplyLatencyStore struct {
	exec     func(context.Context, string, ...any) (sql.Result, error)
	queryRow func(context.Context, string, ...any) replyLatencyScanner
}

func NewSQLGuildReplyLatencyStore(db any) SQLGuildReplyLatencyStore {
	store := SQLGuildReplyLatencyStore{}
	if execDB, ok := db.(replyLatencyExecDB); ok {
		store.exec = execDB.ExecContext
	}
	if queryDB, ok := db.(replyLatencyQueryRowDB); ok {
		store.queryRow = queryDB.QueryRowContext
	} else if queryDB, ok := db.(sqlReplyLatencyQueryRowDB); ok {
		store.queryRow = func(ctx context.Context, query string, args ...any) replyLatencyScanner {
			return queryDB.QueryRowContext(ctx, query, args...)
		}
	}
	return store
}

func (s SQLGuildReplyLatencyStore) GuildReplyLatencyEnabled(ctx context.Context, guildID string) (bool, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false, nil
	}
	if s.queryRow == nil {
		return false, fmt.Errorf("guild reply latency query database is required")
	}
	var enabled bool
	if err := s.queryRow(ctx, `
select reply_latency_enabled
from discord_guild_settings
where guild_id = $1
`, guildID).Scan(&enabled); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("load guild reply latency setting: %w", err)
	}
	return enabled, nil
}

func (s SQLGuildReplyLatencyStore) SetGuildReplyLatencyEnabled(ctx context.Context, guildID string, enabled bool) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("guild ID is required")
	}
	if s.exec == nil {
		return fmt.Errorf("guild reply latency exec database is required")
	}
	_, err := s.exec(ctx, `
insert into discord_guild_settings (
  guild_id,
  reply_latency_enabled,
  updated_at
)
values ($1, $2, now())
on conflict (guild_id) do update set
  reply_latency_enabled = excluded.reply_latency_enabled,
  updated_at = now()
`, guildID, enabled)
	if err != nil {
		return fmt.Errorf("set guild reply latency setting: %w", err)
	}
	return nil
}

var defaultGuildReplyLatencyStore GuildReplyLatencyStore = NewMemoryGuildReplyLatencyStore()

type replyLatencyConfig struct {
	store GuildReplyLatencyStore
	clock func() time.Time
}

func resolveReplyLatencyConfig(configs ...ReplyLatencyConfig) replyLatencyConfig {
	cfg := replyLatencyConfig{
		store: defaultGuildReplyLatencyStore,
		clock: time.Now,
	}
	if len(configs) == 0 {
		return cfg
	}
	if configs[0].Store != nil {
		cfg.store = configs[0].Store
	}
	if configs[0].Clock != nil {
		cfg.clock = configs[0].Clock
	}
	return cfg
}

func (cfg replyLatencyConfig) enabled(ctx context.Context, guildID string) bool {
	if cfg.store == nil || strings.TrimSpace(guildID) == "" {
		return false
	}
	enabled, err := cfg.store.GuildReplyLatencyEnabled(ctx, guildID)
	return err == nil && enabled
}

func (cfg replyLatencyConfig) now() time.Time {
	if cfg.clock == nil {
		return time.Now()
	}
	return cfg.clock()
}

func appendReplyLatencySuffix(content string, elapsed time.Duration) string {
	content = strings.TrimSpace(content)
	suffix := "`" + formatMillis(durationMillis(elapsed)) + "`"
	if content == "" {
		return suffix
	}
	return content + " " + suffix
}

func durationMillis(elapsed time.Duration) int64 {
	if elapsed <= 0 {
		return 0
	}
	return elapsed.Milliseconds()
}
