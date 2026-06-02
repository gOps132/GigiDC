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

func TestSQLGrantManagerGrantsRoleCapabilitiesInTransaction(t *testing.T) {
	tx := &fakeGrantTx{}
	db := &fakeExecDB{tx: tx}
	manager := newSQLGrantManagerWithTx(db, func() string { return "grant-id" })

	err := manager.GrantRoleCapabilities(context.Background(), "guild-id", "role-id", []Capability{"relay.dispatch", "relay.receive"}, "actor-id")
	if err != nil {
		t.Fatalf("GrantRoleCapabilities returned error: %v", err)
	}
	if !db.began {
		t.Fatal("transaction not started")
	}
	if !tx.committed || tx.rolledBack {
		t.Fatalf("tx committed=%v rolledBack=%v, want committed without rollback", tx.committed, tx.rolledBack)
	}
	if tx.commitCalls != 1 || tx.rollbackCalls != 0 {
		t.Fatalf("tx commit/rollback calls = %d/%d, want 1/0", tx.commitCalls, tx.rollbackCalls)
	}
	if len(tx.queries) != 3 {
		t.Fatalf("tx queries = %d, want guild upsert plus two grants", len(tx.queries))
	}
}

func TestSQLGrantManagerRollsBackRoleCapabilitiesOnFailure(t *testing.T) {
	tx := &fakeGrantTx{errAt: 3, err: errors.New("insert failed")}
	db := &fakeExecDB{tx: tx}
	manager := newSQLGrantManagerWithTx(db, func() string { return "grant-id" })

	err := manager.GrantRoleCapabilities(context.Background(), "guild-id", "role-id", []Capability{"relay.dispatch", "relay.receive"}, "actor-id")
	if err == nil {
		t.Fatal("expected grant failure")
	}
	if tx.commitCalls != 0 || tx.rollbackCalls != 1 {
		t.Fatalf("tx commit/rollback calls = %d/%d, want 0/1", tx.commitCalls, tx.rollbackCalls)
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
	tx      *fakeGrantTx
	began   bool
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

func (db *fakeExecDB) beginGrantTx(context.Context) (grantTx, error) {
	db.began = true
	return db.tx, nil
}

type fakeGrantTx struct {
	queries       []string
	calls         int
	errAt         int
	err           error
	commitCalls   int
	rollbackCalls int
	committed     bool
	rolledBack    bool
}

func (tx *fakeGrantTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.calls++
	tx.queries = append(tx.queries, query)
	if tx.errAt == tx.calls {
		return nil, tx.err
	}
	return fakeSQLResult(0), nil
}

func (tx *fakeGrantTx) Commit() error {
	tx.commitCalls++
	tx.committed = true
	return nil
}

func (tx *fakeGrantTx) Rollback() error {
	tx.rollbackCalls++
	tx.rolledBack = true
	return nil
}
