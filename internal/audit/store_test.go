package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStoreRecordsValidEvent(t *testing.T) {
	db := &fakeExecDB{}
	store := NewStore(db, func() string { return "audit-id" })

	err := store.Record(context.Background(), Event{
		Kind:     "discord.permission.check",
		GuildID:  "guild-id",
		ActorID:  "user-id",
		Status:   StatusDenied,
		Reason:   "missing_capability",
		Metadata: map[string]string{"capability": "job.admin"},
	})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("exec calls = %d, want 1", db.calls)
	}
	if !strings.Contains(db.query, "insert into audit_logs") {
		t.Fatalf("query = %q, want audit_logs insert", db.query)
	}
	if db.args[0] != "audit-id" || db.args[1] != "discord.permission.check" {
		t.Fatalf("args = %+v, want generated ID and kind", db.args)
	}
}

func TestStoreSanitizesSensitiveMetadataBeforeInsert(t *testing.T) {
	db := &fakeExecDB{}
	store := NewStore(db, func() string { return "audit-id" })

	err := store.Record(context.Background(), Event{
		Kind:    "llm.provider.update",
		ActorID: "user-id",
		Status:  StatusAllowed,
		Metadata: map[string]string{
			"capability":    "provider.write",
			"source":        "discord",
			"run_id":        "run-123",
			"api_key":       "raw",
			"authorization": "Bearer raw-token",
			"fingerprint":   "sk-live-secret",
		},
	})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("exec calls = %d, want 1", db.calls)
	}
	var metadata map[string]string
	if err := json.Unmarshal([]byte(db.args[6].(string)), &metadata); err != nil {
		t.Fatalf("metadata JSON = %q: %v", db.args[6], err)
	}
	for key, want := range map[string]string{
		"capability": "provider.write",
		"source":     "discord",
		"run_id":     "run-123",
	} {
		if metadata[key] != want {
			t.Fatalf("metadata[%q] = %q, want %q", key, metadata[key], want)
		}
	}
	for _, key := range []string{"api_key", "authorization"} {
		if _, ok := metadata[key]; ok {
			t.Fatalf("metadata contains sensitive key %q: %+v", key, metadata)
		}
	}
	for key, value := range metadata {
		if strings.Contains(strings.ToLower(value), "bearer") || strings.Contains(value, "sk-live-secret") {
			t.Fatalf("metadata[%q] contains sensitive value %q", key, value)
		}
	}
}

func TestStoreRejectsUnknownStatusBeforeInsert(t *testing.T) {
	db := &fakeExecDB{}
	store := NewStore(db, func() string { return "audit-id" })

	err := store.Record(context.Background(), Event{
		Kind:    "discord.agent",
		ActorID: "user-id",
		Status:  Status("weird"),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if db.calls != 0 {
		t.Fatalf("exec calls = %d, want 0", db.calls)
	}
}

func TestStoreReturnsDatabaseError(t *testing.T) {
	db := &fakeExecDB{err: errors.New("db down")}
	store := NewStore(db, func() string { return "audit-id" })

	err := store.Record(context.Background(), Event{
		Kind:    "discord.permission.check",
		ActorID: "user-id",
		Status:  StatusAllowed,
	})
	if err == nil {
		t.Fatal("expected database error")
	}
	if !strings.Contains(err.Error(), "insert audit log") {
		t.Fatalf("error = %v, want insert audit log", err)
	}
}

type fakeExecDB struct {
	query string
	args  []any
	err   error
	calls int
}

func (db *fakeExecDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	db.calls++
	db.query = query
	db.args = args
	return fakeResult(1), db.err
}

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r fakeResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
