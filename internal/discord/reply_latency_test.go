package discord

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestSQLGuildReplyLatencyStoreDefaultsToOffWhenMissing(t *testing.T) {
	db := &fakeReplyLatencyDB{row: fakeReplyLatencyRow{err: sql.ErrNoRows}}
	store := NewSQLGuildReplyLatencyStore(db)

	enabled, err := store.GuildReplyLatencyEnabled(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildReplyLatencyEnabled returned error: %v", err)
	}
	if enabled {
		t.Fatal("enabled = true, want default off")
	}
	if !strings.Contains(db.query, "discord_guild_settings") {
		t.Fatalf("query = %q, want settings table", db.query)
	}
}

func TestSQLGuildReplyLatencyStoreLoadsStoredPreference(t *testing.T) {
	store := NewSQLGuildReplyLatencyStore(&fakeReplyLatencyDB{row: fakeReplyLatencyRow{values: []any{true}}})

	enabled, err := store.GuildReplyLatencyEnabled(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildReplyLatencyEnabled returned error: %v", err)
	}
	if !enabled {
		t.Fatal("enabled = false, want stored true")
	}
}

func TestSQLGuildReplyLatencyStoreUpsertsPreference(t *testing.T) {
	db := &fakeReplyLatencyDB{}
	store := NewSQLGuildReplyLatencyStore(db)

	if err := store.SetGuildReplyLatencyEnabled(context.Background(), "guild-id", true); err != nil {
		t.Fatalf("SetGuildReplyLatencyEnabled returned error: %v", err)
	}
	if !strings.Contains(db.query, "insert into discord_guild_settings") || !strings.Contains(db.query, "on conflict") {
		t.Fatalf("query = %q, want settings upsert", db.query)
	}
	if db.args[0] != "guild-id" || db.args[1] != true {
		t.Fatalf("args = %+v, want guild and enabled", db.args)
	}
}

func TestSQLGuildReplyLatencyStoreRejectsMissingGuild(t *testing.T) {
	db := &fakeReplyLatencyDB{}
	store := NewSQLGuildReplyLatencyStore(db)

	err := store.SetGuildReplyLatencyEnabled(context.Background(), "", true)
	if err == nil || !strings.Contains(err.Error(), "guild ID is required") {
		t.Fatalf("error = %v, want missing guild", err)
	}
	if db.calls != 0 {
		t.Fatalf("exec calls = %d, want none", db.calls)
	}
}

type fakeReplyLatencyDB struct {
	query string
	args  []any
	calls int
	row   fakeReplyLatencyRow
}

func (db *fakeReplyLatencyDB) QueryRowContext(_ context.Context, query string, args ...any) replyLatencyScanner {
	db.query = query
	db.args = args
	return db.row
}

func (db *fakeReplyLatencyDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.calls++
	db.query = query
	db.args = args
	return fakeReplyLatencyResult(1), nil
}

type fakeReplyLatencyRow struct {
	values []any
	err    error
}

func (r fakeReplyLatencyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *bool:
			*target = value.(bool)
		}
	}
	return nil
}

type fakeReplyLatencyResult int64

func (r fakeReplyLatencyResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeReplyLatencyResult) RowsAffected() (int64, error) { return int64(r), nil }
