package memory

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSQLStoreGuildPolicyReturnsDefaultWhenMissing(t *testing.T) {
	db := &fakeMemoryDB{rows: &fakeMemoryRows{}}
	store := NewSQLStore(db)

	got, err := store.GuildPolicy(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildPolicy returned error: %v", err)
	}
	if got.GuildID != "guild-id" || got.DefaultRetentionDays != DefaultRetentionDays || got.RawStorageMode != ModeMetadata || got.EmbeddingsEnabled {
		t.Fatalf("policy = %+v, want default metadata policy", got)
	}
	if !strings.Contains(db.query, "from guild_memory_policies") {
		t.Fatalf("query = %q, want policy lookup", db.query)
	}
}

func TestSQLStoreUpsertsChannelPolicy(t *testing.T) {
	db := &fakeMemoryDB{}
	store := NewSQLStore(db)

	got, err := store.UpsertChannelPolicy(context.Background(), UpsertChannelPolicyRequest{
		GuildID:       " guild-id ",
		ChannelID:     " channel-id ",
		Mode:          ModeFull,
		RetentionDays: 30,
		ActorID:       " actor-id ",
	})
	if err != nil {
		t.Fatalf("UpsertChannelPolicy returned error: %v", err)
	}
	if got.GuildID != "guild-id" || got.ChannelID != "channel-id" || got.Mode != ModeFull || got.RetentionDays != 30 || got.UpdatedByUserID != "actor-id" {
		t.Fatalf("policy = %+v, want normalized channel policy", got)
	}
	if !strings.Contains(db.exec, "insert into guild_memory_channels") || !strings.Contains(db.exec, "on conflict (guild_id, channel_id)") {
		t.Fatalf("exec = %q, want channel policy upsert", db.exec)
	}
	if len(db.args) != 5 || db.args[0] != "guild-id" || db.args[1] != "channel-id" || db.args[2] != ModeFull || db.args[3] != 30 || db.args[4] != "actor-id" {
		t.Fatalf("args = %+v, want normalized upsert args", db.args)
	}
}

func TestSQLStoreRejectsInvalidChannelPolicy(t *testing.T) {
	tests := []struct {
		name string
		req  UpsertChannelPolicyRequest
		want string
	}{
		{name: "missing guild", req: UpsertChannelPolicyRequest{ChannelID: "channel-id", Mode: ModeMetadata, ActorID: "actor-id"}, want: "guild ID is required"},
		{name: "missing channel", req: UpsertChannelPolicyRequest{GuildID: "guild-id", Mode: ModeMetadata, ActorID: "actor-id"}, want: "channel ID is required"},
		{name: "missing actor", req: UpsertChannelPolicyRequest{GuildID: "guild-id", ChannelID: "channel-id", Mode: ModeMetadata}, want: "actor ID is required"},
		{name: "bad mode", req: UpsertChannelPolicyRequest{GuildID: "guild-id", ChannelID: "channel-id", Mode: "raw", ActorID: "actor-id"}, want: "Unsupported memory mode."},
		{name: "bad retention", req: UpsertChannelPolicyRequest{GuildID: "guild-id", ChannelID: "channel-id", Mode: ModeMetadata, RetentionDays: 366, ActorID: "actor-id"}, want: "retention days must be between 1 and 365"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSQLStore(&fakeMemoryDB{})
			_, err := store.UpsertChannelPolicy(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSQLStoreGuildStatusLoadsPolicyAndChannels(t *testing.T) {
	db := &fakeMemoryDB{rowsByQuery: []*fakeMemoryRows{
		{values: [][]any{{"guild-id", 120, ModeMetadata, false, "actor-id"}}},
		{values: [][]any{{"guild-id", "channel-b", ModeOff, 0, "actor-id"}, {"guild-id", "channel-a", ModeFull, 14, "actor-id"}}},
	}}
	store := NewSQLStore(db)

	got, err := store.GuildStatus(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildStatus returned error: %v", err)
	}
	if got.Policy.DefaultRetentionDays != 120 || len(got.Channels) != 2 || got.Channels[0].ChannelID != "channel-a" || got.Channels[1].ChannelID != "channel-b" {
		t.Fatalf("status = %+v, want policy plus sorted channels", got)
	}
}

func TestSQLStoreReturnsQueryErrors(t *testing.T) {
	store := NewSQLStore(&fakeMemoryDB{err: errors.New("db down")})

	_, err := store.GuildPolicy(context.Background(), "guild-id")
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("error = %v, want db down", err)
	}
}

func TestSQLStoreRecordsMessageWithoutRawTextWhenMetadataOnly(t *testing.T) {
	db := &fakeMemoryDB{}
	store := NewSQLStore(db)
	createdAt := time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC)

	err := store.RecordMessage(context.Background(), MessageRecord{
		MessageID:      "message-id",
		GuildID:        "guild-id",
		ChannelID:      "channel-id",
		AuthorUserID:   "user-id",
		NormalizedText: "",
		ContentHash:    HashText("hello postgres"),
		CreatedAt:      createdAt,
		RetentionUntil: createdAt.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("RecordMessage returned error: %v", err)
	}
	if !strings.Contains(db.exec, "insert into guild_memory_messages") || !strings.Contains(db.exec, "on conflict (message_id)") {
		t.Fatalf("exec = %q, want message upsert", db.exec)
	}
	if len(db.args) != 8 || db.args[4] != "" || db.args[5] == "" {
		t.Fatalf("args = %+v, want no raw normalized text and hash present", db.args)
	}
}

func TestSQLStoreCountMentionsUsesNormalizedText(t *testing.T) {
	db := &fakeMemoryDB{rows: &fakeMemoryRows{values: [][]any{{3}}}}
	store := NewSQLStore(db)

	got, err := store.CountMentions(context.Background(), CountRequest{
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		AuthorUserID: "user-id",
		Text:         "Postgres",
	})
	if err != nil {
		t.Fatalf("CountMentions returned error: %v", err)
	}
	if got.Count != 3 {
		t.Fatalf("count = %d, want 3", got.Count)
	}
	if !strings.Contains(db.query, "normalized_text") || !strings.Contains(db.query, "retention_until > now()") {
		t.Fatalf("query = %q, want exact retained text count", db.query)
	}
}

func TestLiveIngestorStoresOnlyFullModeText(t *testing.T) {
	store := &fakeIngestStore{
		policy:  DefaultPolicy("guild-id"),
		channel: ChannelPolicy{GuildID: "guild-id", ChannelID: "channel-id", Mode: ModeFull, RetentionDays: 7},
		ok:      true,
	}
	ingestor := &LiveIngestor{store: store, now: func() time.Time { return time.Date(2026, 6, 18, 1, 0, 0, 0, time.UTC) }}

	err := ingestor.ingest(context.Background(), MessageEvent{
		MessageID:    "message-id",
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		AuthorUserID: "user-id",
		Content:      "Hello Postgres",
	})
	if err != nil {
		t.Fatalf("ingest returned error: %v", err)
	}
	if store.record.NormalizedText != "hello postgres" || store.record.RetentionUntil.Sub(store.record.CreatedAt) != 7*24*time.Hour {
		t.Fatalf("record = %+v, want full text and channel retention", store.record)
	}
}

func TestLiveIngestorSkipsOffChannels(t *testing.T) {
	store := &fakeIngestStore{
		policy:  DefaultPolicy("guild-id"),
		channel: ChannelPolicy{GuildID: "guild-id", ChannelID: "channel-id", Mode: ModeOff},
		ok:      true,
	}
	ingestor := &LiveIngestor{store: store}

	err := ingestor.ingest(context.Background(), MessageEvent{
		MessageID:    "message-id",
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		AuthorUserID: "user-id",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("ingest returned error: %v", err)
	}
	if store.recorded {
		t.Fatalf("recorded = true, want off channel skipped")
	}
}

type fakeMemoryDB struct {
	query       string
	exec        string
	args        []any
	rows        *fakeMemoryRows
	rowsByQuery []*fakeMemoryRows
	err         error
}

func (db *fakeMemoryDB) QueryContext(ctx context.Context, query string, args ...any) (memoryRows, error) {
	db.query = query
	if db.err != nil {
		return nil, db.err
	}
	if len(db.rowsByQuery) > 0 {
		rows := db.rowsByQuery[0]
		db.rowsByQuery = db.rowsByQuery[1:]
		return rows, nil
	}
	if db.rows != nil {
		return db.rows, nil
	}
	return &fakeMemoryRows{}, nil
}

func (db *fakeMemoryDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	db.exec = query
	db.args = append([]any(nil), args...)
	return fakeMemoryResult(1), db.err
}

type fakeMemoryRows struct {
	values [][]any
	index  int
	err    error
}

func (r *fakeMemoryRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeMemoryRows) Scan(dest ...any) error {
	if r.index >= len(r.values) {
		return sql.ErrNoRows
	}
	row := r.values[r.index]
	r.index++
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *int:
			*d = row[i].(int)
		case *Mode:
			*d = row[i].(Mode)
		case *bool:
			*d = row[i].(bool)
		default:
			return errors.New("unsupported scan dest")
		}
	}
	return nil
}

func (r *fakeMemoryRows) Err() error {
	return r.err
}

func (r *fakeMemoryRows) Close() error {
	return nil
}

type fakeMemoryResult int64

func (r fakeMemoryResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeMemoryResult) RowsAffected() (int64, error) { return int64(r), nil }

type fakeIngestStore struct {
	policy   Policy
	channel  ChannelPolicy
	ok       bool
	record   MessageRecord
	recorded bool
	err      error
}

func (s *fakeIngestStore) GuildPolicy(ctx context.Context, guildID string) (Policy, error) {
	return s.policy, s.err
}

func (s *fakeIngestStore) ChannelPolicy(ctx context.Context, guildID string, channelID string) (ChannelPolicy, bool, error) {
	return s.channel, s.ok, s.err
}

func (s *fakeIngestStore) RecordMessage(ctx context.Context, record MessageRecord) error {
	s.record = record
	s.recorded = true
	return s.err
}
