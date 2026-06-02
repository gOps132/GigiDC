package capability

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSQLGrantStoreLoadsUserCapabilities(t *testing.T) {
	db := &fakeQueryDB{rows: &fakeRows{values: []Capability{"plugin.install", "job.admin"}}}
	store := newSQLGrantStoreWithRows(db)

	got, err := store.UserCapabilities(context.Background(), "guild-id", "user-id")
	if err != nil {
		t.Fatalf("UserCapabilities returned error: %v", err)
	}
	if len(got) != 2 || got[0] != "plugin.install" || got[1] != "job.admin" {
		t.Fatalf("capabilities = %+v, want plugin.install/job.admin", got)
	}
	if !strings.Contains(db.query, "user_capability_grants") || !strings.Contains(db.query, "user_id") {
		t.Fatalf("query = %q, want user grant lookup", db.query)
	}
}

func TestSQLGrantStoreLoadsRoleCapabilitiesByRoleID(t *testing.T) {
	db := &fakeQueryDB{rows: &fakeRows{values: []Capability{"plugin.run.music"}}}
	store := newSQLGrantStoreWithRows(db)

	got, err := store.RoleCapabilities(context.Background(), "guild-id", []string{"role-1", "role-2"})
	if err != nil {
		t.Fatalf("RoleCapabilities returned error: %v", err)
	}
	if len(got) != 1 || got[0] != "plugin.run.music" {
		t.Fatalf("capabilities = %+v, want plugin.run.music", got)
	}
	if !strings.Contains(db.query, "role_capability_grants") || !strings.Contains(db.query, "role_id") || strings.Contains(db.query, "role_name") {
		t.Fatalf("query = %q, want role ID lookup only", db.query)
	}
}

func TestSQLGrantStoreSkipsRoleLookupWithoutRoles(t *testing.T) {
	db := &fakeQueryDB{}
	store := newSQLGrantStoreWithRows(db)

	got, err := store.RoleCapabilities(context.Background(), "guild-id", nil)
	if err != nil {
		t.Fatalf("RoleCapabilities returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("capabilities = %+v, want none", got)
	}
	if db.calls != 0 {
		t.Fatalf("query calls = %d, want 0", db.calls)
	}
}

func TestSQLGrantStoreReturnsQueryErrors(t *testing.T) {
	db := &fakeQueryDB{err: errors.New("db down")}
	store := newSQLGrantStoreWithRows(db)

	_, err := store.UserCapabilities(context.Background(), "guild-id", "user-id")
	if err == nil {
		t.Fatal("expected query error")
	}
}

type fakeQueryDB struct {
	query string
	args  []any
	rows  grantRows
	err   error
	calls int
}

func (db *fakeQueryDB) QueryContext(ctx context.Context, query string, args ...any) (grantRows, error) {
	db.calls++
	db.query = query
	db.args = args
	return db.rows, db.err
}

type fakeRows struct {
	values []Capability
	index  int
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	target, ok := dest[0].(*string)
	if !ok {
		return errors.New("expected string scan target")
	}
	*target = string(r.values[r.index])
	r.index++
	return nil
}

func (r *fakeRows) Err() error {
	return nil
}

func (r *fakeRows) Close() error {
	return nil
}
