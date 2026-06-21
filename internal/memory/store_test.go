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

func TestSQLStoreRecordMessageDoesNotReviveTombstonedRows(t *testing.T) {
	db := &fakeMemoryDB{}
	store := NewSQLStore(db)

	err := store.RecordMessage(context.Background(), MessageRecord{
		MessageID:      "message-id",
		GuildID:        "guild-id",
		ChannelID:      "channel-id",
		AuthorUserID:   "user-id",
		NormalizedText: "hello postgres",
		ContentHash:    HashText("hello postgres"),
		CreatedAt:      time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC),
		RetentionUntil: time.Date(2026, 6, 25, 1, 2, 3, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordMessage returned error: %v", err)
	}
	if !strings.Contains(db.exec, "where guild_memory_messages.deleted_at is null") {
		t.Fatalf("exec = %q, want tombstoned rows protected from revive", db.exec)
	}
}

func TestSQLStoreDeleteMessageTombstonesAndClearsText(t *testing.T) {
	db := &fakeMemoryDB{}
	store := NewSQLStore(db)
	deletedAt := time.Date(2026, 6, 18, 2, 0, 0, 0, time.UTC)

	if err := store.DeleteMessage(context.Background(), " guild-id ", " message-id ", deletedAt); err != nil {
		t.Fatalf("DeleteMessage returned error: %v", err)
	}
	for _, want := range []string{"delete from guild_memory_segments", "update guild_memory_messages", "deleted_at = $3", "normalized_text = null", "content_ciphertext = null", "content_hash = ''"} {
		if !strings.Contains(db.exec, want) {
			t.Fatalf("exec = %q, want %q", db.exec, want)
		}
	}
	if len(db.args) != 3 || db.args[0] != "guild-id" || db.args[1] != "message-id" || db.args[2] != deletedAt {
		t.Fatalf("args = %+v, want normalized delete args", db.args)
	}
}

func TestSQLStorePurgeExpiredMessagesDeletesRetentionRows(t *testing.T) {
	db := &fakeMemoryDB{}
	store := NewSQLStore(db)
	now := time.Date(2026, 6, 18, 2, 0, 0, 0, time.UTC)

	deleted, err := store.PurgeExpiredMessages(context.Background(), now)
	if err != nil {
		t.Fatalf("PurgeExpiredMessages returned error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if !strings.Contains(db.exec, "delete from guild_memory_messages") || !strings.Contains(db.exec, "retention_until <= $1") {
		t.Fatalf("exec = %q, want retention delete", db.exec)
	}
	if len(db.args) != 1 || db.args[0] != now {
		t.Fatalf("args = %+v, want purge time", db.args)
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

func TestSQLStoreSearchMessagesUsesCurrentChannelAndLimit(t *testing.T) {
	createdAt := time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC)
	retentionUntil := time.Date(2026, 7, 18, 1, 2, 3, 0, time.UTC)
	retrievedAt := time.Date(2026, 6, 19, 1, 2, 3, 0, time.UTC)
	db := &fakeMemoryDB{rows: &fakeMemoryRows{values: [][]any{{"message-id", "guild-id", "channel-id", "user-id", "hello postgres", createdAt, retentionUntil, retrievedAt}}}}
	store := NewSQLStore(db)

	got, err := store.SearchMessages(context.Background(), SearchRequest{
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		AuthorUserID: "user-id",
		Query:        "Postgres",
		Limit:        10,
	})
	if err != nil {
		t.Fatalf("SearchMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].MessageID != "message-id" || got[0].GuildID != "guild-id" || got[0].Text != "hello postgres" || !got[0].RetentionUntil.Equal(retentionUntil) || !got[0].RetrievedAt.Equal(retrievedAt) {
		t.Fatalf("results = %+v, want search result", got)
	}
	for _, want := range []string{"channel_id = $2", "deleted_at is null", "retention_until > now()", "position($4 in normalized_text) > 0", "retention_until", "now() as retrieved_at", "limit $7"} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("query = %q, want %q", db.query, want)
		}
	}
	if db.args[6] != 10 {
		t.Fatalf("args = %+v, want limit 10", db.args)
	}
}

func TestSQLStoreRecentMessagesReturnsProvenanceAndRetentionProof(t *testing.T) {
	createdAt := time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC)
	retentionUntil := time.Date(2026, 7, 18, 1, 2, 3, 0, time.UTC)
	retrievedAt := time.Date(2026, 6, 19, 1, 2, 3, 0, time.UTC)
	db := &fakeMemoryDB{rows: &fakeMemoryRows{values: [][]any{{"message-id", "guild-id", "channel-id", "user-id", "hello postgres", createdAt, retentionUntil, retrievedAt}}}}
	store := NewSQLStore(db)

	got, err := store.RecentMessages(context.Background(), RecentRequest{
		GuildID:   "guild-id",
		ChannelID: "channel-id",
		Limit:     3,
	})
	if err != nil {
		t.Fatalf("RecentMessages returned error: %v", err)
	}
	if len(got) != 1 || got[0].GuildID != "guild-id" || got[0].MessageID != "message-id" || !got[0].RetentionUntil.Equal(retentionUntil) || !got[0].RetrievedAt.Equal(retrievedAt) {
		t.Fatalf("results = %+v, want recent result with provenance", got)
	}
	for _, want := range []string{"guild_id = $1", "channel_id = $2", "deleted_at is null", "retention_until > now()", "retention_until", "now() as retrieved_at", "limit $6"} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("query = %q, want %q", db.query, want)
		}
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

func TestLiveIngestorTombstonesDeleteEvents(t *testing.T) {
	deletedAt := time.Date(2026, 6, 18, 2, 0, 0, 0, time.UTC)
	store := &fakeIngestStore{}
	ingestor := &LiveIngestor{store: store, now: func() time.Time { return deletedAt }}

	err := ingestor.ingest(context.Background(), MessageEvent{
		MessageID: "message-id",
		GuildID:   "guild-id",
		Deleted:   true,
	})
	if err != nil {
		t.Fatalf("ingest returned error: %v", err)
	}
	if !store.deleted || store.deletedGuildID != "guild-id" || store.deletedMessageID != "message-id" || !store.deletedAt.Equal(deletedAt) {
		t.Fatalf("delete = %v %s %s %s, want tombstone event", store.deleted, store.deletedGuildID, store.deletedMessageID, store.deletedAt)
	}
	if store.recorded {
		t.Fatalf("recorded = true, want delete path only")
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
	db.args = append([]any(nil), args...)
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
		case *time.Time:
			*d = row[i].(time.Time)
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
	policy           Policy
	channel          ChannelPolicy
	ok               bool
	record           MessageRecord
	recorded         bool
	deleted          bool
	deletedGuildID   string
	deletedMessageID string
	deletedAt        time.Time
	err              error
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

func (s *fakeIngestStore) DeleteMessage(ctx context.Context, guildID string, messageID string, deletedAt time.Time) error {
	s.deleted = true
	s.deletedGuildID = guildID
	s.deletedMessageID = messageID
	s.deletedAt = deletedAt
	return s.err
}
