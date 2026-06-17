package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type Mode string

const (
	ModeOff      Mode = "off"
	ModeMetadata Mode = "metadata"
	ModeFull     Mode = "full"
)

const DefaultRetentionDays = 90

type Policy struct {
	GuildID              string
	DefaultRetentionDays int
	RawStorageMode       Mode
	EmbeddingsEnabled    bool
	UpdatedByUserID      string
}

type ChannelPolicy struct {
	GuildID         string
	ChannelID       string
	Mode            Mode
	RetentionDays   int
	UpdatedByUserID string
}

type Status struct {
	Policy   Policy
	Channels []ChannelPolicy
}

type UpsertChannelPolicyRequest struct {
	GuildID       string
	ChannelID     string
	Mode          Mode
	RetentionDays int
	ActorID       string
}

type memoryRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type memoryQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (memoryRows, error)
}

type memoryExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type SQLStore struct {
	query func(context.Context, string, ...any) (memoryRows, error)
	exec  func(context.Context, string, ...any) (sql.Result, error)
}

func NewSQLStore(db any) SQLStore {
	store := SQLStore{}
	if queryDB, ok := db.(memoryQueryDB); ok {
		store.query = queryDB.QueryContext
	} else if queryDB, ok := db.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}); ok {
		store.query = func(ctx context.Context, query string, args ...any) (memoryRows, error) {
			return queryDB.QueryContext(ctx, query, args...)
		}
	}
	if execDB, ok := db.(memoryExecDB); ok {
		store.exec = execDB.ExecContext
	} else if execDB, ok := db.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}); ok {
		store.exec = execDB.ExecContext
	}
	return store
}

func (s SQLStore) GuildStatus(ctx context.Context, guildID string) (Status, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return Status{}, fmt.Errorf("guild ID is required")
	}
	policy, err := s.GuildPolicy(ctx, guildID)
	if err != nil {
		return Status{}, err
	}
	channels, err := s.ChannelPolicies(ctx, guildID)
	if err != nil {
		return Status{}, err
	}
	return Status{Policy: policy, Channels: channels}, nil
}

func (s SQLStore) GuildPolicy(ctx context.Context, guildID string) (Policy, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return Policy{}, fmt.Errorf("guild ID is required")
	}
	if s.query == nil {
		return Policy{}, fmt.Errorf("memory query database is required")
	}
	rows, err := s.query(ctx, `
select guild_id, default_retention_days, raw_storage_mode, embeddings_enabled, coalesce(updated_by_user_id, '')
from guild_memory_policies
where guild_id = $1
`, guildID)
	if err != nil {
		return Policy{}, err
	}
	defer rows.Close()

	if rows.Next() {
		var policy Policy
		if err := rows.Scan(&policy.GuildID, &policy.DefaultRetentionDays, &policy.RawStorageMode, &policy.EmbeddingsEnabled, &policy.UpdatedByUserID); err != nil {
			return Policy{}, err
		}
		if err := rows.Err(); err != nil {
			return Policy{}, err
		}
		return policy, nil
	}
	if err := rows.Err(); err != nil {
		return Policy{}, err
	}
	return DefaultPolicy(guildID), nil
}

func (s SQLStore) ChannelPolicies(ctx context.Context, guildID string) ([]ChannelPolicy, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, fmt.Errorf("guild ID is required")
	}
	if s.query == nil {
		return nil, fmt.Errorf("memory query database is required")
	}
	rows, err := s.query(ctx, `
select guild_id, channel_id, mode, coalesce(retention_days, 0), coalesce(updated_by_user_id, '')
from guild_memory_channels
where guild_id = $1
order by channel_id
`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []ChannelPolicy
	for rows.Next() {
		var channel ChannelPolicy
		if err := rows.Scan(&channel.GuildID, &channel.ChannelID, &channel.Mode, &channel.RetentionDays, &channel.UpdatedByUserID); err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].ChannelID < channels[j].ChannelID })
	return channels, nil
}

func (s SQLStore) UpsertChannelPolicy(ctx context.Context, req UpsertChannelPolicyRequest) (ChannelPolicy, error) {
	req, err := normalizeChannelPolicyRequest(req)
	if err != nil {
		return ChannelPolicy{}, err
	}
	if s.exec == nil {
		return ChannelPolicy{}, fmt.Errorf("memory exec database is required")
	}
	if _, err := s.exec(ctx, `
insert into guild_memory_channels (
  guild_id,
  channel_id,
  mode,
  retention_days,
  updated_by_user_id,
  updated_at
)
values ($1, $2, $3, nullif($4, 0), $5, now())
on conflict (guild_id, channel_id) do update set
  mode = excluded.mode,
  retention_days = excluded.retention_days,
  updated_by_user_id = excluded.updated_by_user_id,
  updated_at = now()
`, req.GuildID, req.ChannelID, req.Mode, req.RetentionDays, req.ActorID); err != nil {
		return ChannelPolicy{}, err
	}
	return ChannelPolicy{
		GuildID:         req.GuildID,
		ChannelID:       req.ChannelID,
		Mode:            req.Mode,
		RetentionDays:   req.RetentionDays,
		UpdatedByUserID: req.ActorID,
	}, nil
}

func DefaultPolicy(guildID string) Policy {
	return Policy{
		GuildID:              strings.TrimSpace(guildID),
		DefaultRetentionDays: DefaultRetentionDays,
		RawStorageMode:       ModeMetadata,
		EmbeddingsEnabled:    false,
	}
}

func normalizeChannelPolicyRequest(req UpsertChannelPolicyRequest) (UpsertChannelPolicyRequest, error) {
	req.GuildID = strings.TrimSpace(req.GuildID)
	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.ActorID = strings.TrimSpace(req.ActorID)
	req.Mode = Mode(strings.TrimSpace(string(req.Mode)))
	if req.GuildID == "" {
		return req, fmt.Errorf("guild ID is required")
	}
	if req.ChannelID == "" {
		return req, fmt.Errorf("channel ID is required")
	}
	if req.ActorID == "" {
		return req, fmt.Errorf("actor ID is required")
	}
	if err := ValidateMode(req.Mode); err != nil {
		return req, err
	}
	if req.RetentionDays < 0 || req.RetentionDays > 365 {
		return req, fmt.Errorf("retention days must be between 1 and 365")
	}
	if req.RetentionDays == 0 {
		return req, nil
	}
	if req.RetentionDays < 1 {
		return req, fmt.Errorf("retention days must be between 1 and 365")
	}
	return req, nil
}

func ValidateMode(mode Mode) error {
	switch mode {
	case ModeOff, ModeMetadata, ModeFull:
		return nil
	default:
		return fmt.Errorf("Unsupported memory mode.")
	}
}
