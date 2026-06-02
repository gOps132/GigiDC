package capability

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeCapability(t *testing.T) {
	capabilityName, err := Normalize(" plugin.install ")
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if capabilityName != "plugin.install" {
		t.Fatalf("capability = %q, want plugin.install", capabilityName)
	}

	for _, value := range []string{"", "Plugin.Install", "plugin install"} {
		if _, err := Normalize(value); err == nil {
			t.Fatalf("Normalize(%q) returned nil error", value)
		}
	}
}

func TestSQLGrantManagerGrantsRole(t *testing.T) {
	db := &fakeExecDB{}
	manager := NewSQLGrantManager(db, func() string { return "grant-id" })

	err := manager.GrantRole(context.Background(), "guild-id", "role-id", "plugin.install", "actor-id")
	if err != nil {
		t.Fatalf("GrantRole returned error: %v", err)
	}
	if len(db.queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(db.queries))
	}
	if !strings.Contains(db.queries[0], "insert into guilds") {
		t.Fatalf("first query = %q, want guild upsert", db.queries[0])
	}
	if !strings.Contains(db.queries[1], "insert into role_capability_grants") || !strings.Contains(db.queries[1], "role_id") {
		t.Fatalf("grant query = %q, want role grant insert", db.queries[1])
	}
	if got := db.args[1][0]; got != "grant-id" {
		t.Fatalf("grant id arg = %v, want grant-id", got)
	}
	if got := db.args[1][4]; got != "actor-id" {
		t.Fatalf("actor arg = %v, want actor-id", got)
	}
}

func TestSQLGrantManagerRevokesUser(t *testing.T) {
	db := &fakeExecDB{}
	manager := NewSQLGrantManager(db, func() string { return "unused" })

	err := manager.RevokeUser(context.Background(), "guild-id", "user-id", "job.admin")
	if err != nil {
		t.Fatalf("RevokeUser returned error: %v", err)
	}
	if len(db.queries) != 1 {
		t.Fatalf("queries = %d, want 1", len(db.queries))
	}
	if !strings.Contains(db.queries[0], "delete from user_capability_grants") || !strings.Contains(db.queries[0], "user_id") {
		t.Fatalf("revoke query = %q, want user grant delete", db.queries[0])
	}
}

func TestSQLGrantManagerValidatesInputs(t *testing.T) {
	db := &fakeExecDB{}
	manager := NewSQLGrantManager(db, func() string { return "grant-id" })

	err := manager.GrantUser(context.Background(), "", "user-id", "job.admin", "actor-id")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(db.queries) != 0 {
		t.Fatalf("queries = %d, want 0", len(db.queries))
	}
}

func TestSQLGrantManagerReturnsDBErrors(t *testing.T) {
	db := &fakeExecDB{err: errors.New("db down")}
	manager := NewSQLGrantManager(db, func() string { return "grant-id" })

	err := manager.GrantRole(context.Background(), "guild-id", "role-id", "plugin.install", "actor-id")
	if err == nil || !strings.Contains(err.Error(), "ensure guild") {
		t.Fatalf("error = %v, want ensure guild error", err)
	}
}

type fakeExecDB struct {
	queries []string
	args    [][]any
	err     error
}

func (db *fakeExecDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.queries = append(db.queries, query)
	db.args = append(db.args, args)
	if db.err != nil {
		return nil, db.err
	}
	return fakeSQLResult(0), nil
}

type fakeSQLResult int64

func (fakeSQLResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeSQLResult) RowsAffected() (int64, error) { return 0, nil }
