package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
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

type MessageEvent struct {
	MessageID    string
	GuildID      string
	ChannelID    string
	AuthorUserID string
	Content      string
	CreatedAt    time.Time
}

type MessageRecord struct {
	MessageID      string
	GuildID        string
	ChannelID      string
	AuthorUserID   string
	NormalizedText string
	ContentHash    string
	CreatedAt      time.Time
	RetentionUntil time.Time
}

type CountRequest struct {
	GuildID      string
	ChannelID    string
	AuthorUserID string
	Text         string
}

type CountResult struct {
	Count int
}

type SearchRequest struct {
	GuildID      string
	ChannelID    string
	AuthorUserID string
	Query        string
	Limit        int
}

type SearchResult struct {
	MessageID    string
	ChannelID    string
	AuthorUserID string
	Text         string
	CreatedAt    time.Time
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

func (s SQLStore) ChannelPolicy(ctx context.Context, guildID string, channelID string) (ChannelPolicy, bool, error) {
	guildID = strings.TrimSpace(guildID)
	channelID = strings.TrimSpace(channelID)
	if guildID == "" {
		return ChannelPolicy{}, false, fmt.Errorf("guild ID is required")
	}
	if channelID == "" {
		return ChannelPolicy{}, false, fmt.Errorf("channel ID is required")
	}
	if s.query == nil {
		return ChannelPolicy{}, false, fmt.Errorf("memory query database is required")
	}
	rows, err := s.query(ctx, `
select guild_id, channel_id, mode, coalesce(retention_days, 0), coalesce(updated_by_user_id, '')
from guild_memory_channels
where guild_id = $1 and channel_id = $2
`, guildID, channelID)
	if err != nil {
		return ChannelPolicy{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return ChannelPolicy{}, false, err
		}
		return ChannelPolicy{}, false, nil
	}
	var channel ChannelPolicy
	if err := rows.Scan(&channel.GuildID, &channel.ChannelID, &channel.Mode, &channel.RetentionDays, &channel.UpdatedByUserID); err != nil {
		return ChannelPolicy{}, false, err
	}
	if err := rows.Err(); err != nil {
		return ChannelPolicy{}, false, err
	}
	return channel, true, nil
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

func (s SQLStore) RecordMessage(ctx context.Context, record MessageRecord) error {
	record, err := normalizeMessageRecord(record)
	if err != nil {
		return err
	}
	if s.exec == nil {
		return fmt.Errorf("memory exec database is required")
	}
	_, err = s.exec(ctx, `
insert into guild_memory_messages (
  message_id,
  guild_id,
  channel_id,
  author_user_id,
  normalized_text,
  content_hash,
  created_at,
  retention_until,
  indexed_at
)
values ($1, $2, $3, $4, nullif($5, ''), $6, $7, $8, now())
on conflict (message_id) do update set
  guild_id = excluded.guild_id,
  channel_id = excluded.channel_id,
  author_user_id = excluded.author_user_id,
  normalized_text = excluded.normalized_text,
  content_hash = excluded.content_hash,
  created_at = excluded.created_at,
  retention_until = excluded.retention_until,
  indexed_at = now()
where guild_memory_messages.deleted_at is null
`, record.MessageID, record.GuildID, record.ChannelID, record.AuthorUserID, record.NormalizedText, record.ContentHash, record.CreatedAt, record.RetentionUntil)
	if err != nil {
		return fmt.Errorf("record memory message: %w", err)
	}
	return nil
}

func (s SQLStore) CountMentions(ctx context.Context, req CountRequest) (CountResult, error) {
	req = normalizeCountRequest(req)
	if req.GuildID == "" {
		return CountResult{}, fmt.Errorf("guild ID is required")
	}
	if req.Text == "" {
		return CountResult{}, fmt.Errorf("Text is required.")
	}
	if s.query == nil {
		return CountResult{}, fmt.Errorf("memory query database is required")
	}
	whereChannel := "and ($2 = '' or channel_id = $2)"
	rows, err := s.query(ctx, `
select coalesce(sum((length(normalized_text) - length(replace(normalized_text, $4, ''))) / length($4)), 0)
from guild_memory_messages
where guild_id = $1
  `+whereChannel+`
  and ($3 = '' or author_user_id = $3)
  and normalized_text is not null
  and deleted_at is null
  and retention_until > now()
  and position($4 in normalized_text) > 0
`, req.GuildID, req.ChannelID, req.AuthorUserID, req.Text)
	if err != nil {
		return CountResult{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return CountResult{}, err
		}
		return CountResult{}, nil
	}
	var result CountResult
	if err := rows.Scan(&result.Count); err != nil {
		return CountResult{}, err
	}
	if err := rows.Err(); err != nil {
		return CountResult{}, err
	}
	return result, nil
}

func (s SQLStore) SearchMessages(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	req = normalizeSearchRequest(req)
	if req.GuildID == "" {
		return nil, fmt.Errorf("guild ID is required")
	}
	if req.ChannelID == "" {
		return nil, fmt.Errorf("channel ID is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("Query is required.")
	}
	if s.query == nil {
		return nil, fmt.Errorf("memory query database is required")
	}
	rows, err := s.query(ctx, `
select message_id, channel_id, author_user_id, normalized_text, created_at
from guild_memory_messages
where guild_id = $1
  and channel_id = $2
  and ($3 = '' or author_user_id = $3)
  and normalized_text is not null
  and deleted_at is null
  and retention_until > now()
  and position($4 in normalized_text) > 0
order by created_at desc
limit $5
`, req.GuildID, req.ChannelID, req.AuthorUserID, req.Query, req.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(&result.MessageID, &result.ChannelID, &result.AuthorUserID, &result.Text, &result.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
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

func normalizeMessageRecord(record MessageRecord) (MessageRecord, error) {
	record.MessageID = strings.TrimSpace(record.MessageID)
	record.GuildID = strings.TrimSpace(record.GuildID)
	record.ChannelID = strings.TrimSpace(record.ChannelID)
	record.AuthorUserID = strings.TrimSpace(record.AuthorUserID)
	record.NormalizedText = NormalizeText(record.NormalizedText)
	record.ContentHash = strings.TrimSpace(record.ContentHash)
	if record.MessageID == "" {
		return record, fmt.Errorf("message ID is required")
	}
	if record.GuildID == "" {
		return record, fmt.Errorf("guild ID is required")
	}
	if record.ChannelID == "" {
		return record, fmt.Errorf("channel ID is required")
	}
	if record.AuthorUserID == "" {
		return record, fmt.Errorf("author user ID is required")
	}
	if record.CreatedAt.IsZero() {
		return record, fmt.Errorf("message created time is required")
	}
	if record.RetentionUntil.IsZero() || !record.RetentionUntil.After(record.CreatedAt) {
		return record, fmt.Errorf("retention time must be after message creation")
	}
	if record.ContentHash == "" {
		record.ContentHash = HashText(record.NormalizedText)
	}
	return record, nil
}

func normalizeCountRequest(req CountRequest) CountRequest {
	req.GuildID = strings.TrimSpace(req.GuildID)
	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.AuthorUserID = strings.TrimSpace(req.AuthorUserID)
	req.Text = NormalizeText(req.Text)
	return req
}

func normalizeSearchRequest(req SearchRequest) SearchRequest {
	req.GuildID = strings.TrimSpace(req.GuildID)
	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.AuthorUserID = strings.TrimSpace(req.AuthorUserID)
	req.Query = NormalizeText(req.Query)
	if req.Limit == 0 {
		req.Limit = 5
	}
	if req.Limit < 1 {
		req.Limit = 1
	}
	if req.Limit > 25 {
		req.Limit = 25
	}
	return req
}

func NormalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func HashText(value string) string {
	sum := sha256.Sum256([]byte(NormalizeText(value)))
	return hex.EncodeToString(sum[:])
}
