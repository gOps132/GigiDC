package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSQLCatalogStoreUpsertsApprovedManifestInTransaction(t *testing.T) {
	db := &fakeCatalogDB{tx: &fakeCatalogTx{}}
	store := NewSQLCatalogStore(db, func() string { return "plugin-version-id" })

	err := store.UpsertApprovedManifest(context.Background(), validManifest(), "actor-id")
	if err != nil {
		t.Fatalf("UpsertApprovedManifest returned error: %v", err)
	}
	if db.beginCalls != 1 || !db.tx.committed || db.tx.rolledBack {
		t.Fatalf("tx state = %+v, begin calls = %d; want committed transaction", db.tx, db.beginCalls)
	}
	if len(db.tx.queries) != 2 {
		t.Fatalf("queries = %d, want plugin and version upserts", len(db.tx.queries))
	}
	if !strings.Contains(db.tx.queries[0], "insert into plugins") || !strings.Contains(db.tx.queries[1], "insert into plugin_versions") {
		t.Fatalf("queries = %+v, want plugin/version inserts", db.tx.queries)
	}
	if db.tx.args[0][0] != "example-tool" || db.tx.args[1][0] != "plugin-version-id" {
		t.Fatalf("args = %+v, want stable plugin ID and generated version ID", db.tx.args)
	}
}

func TestSQLCatalogStoreRollsBackWhenVersionInsertFails(t *testing.T) {
	db := &fakeCatalogDB{tx: &fakeCatalogTx{failOnCall: 2, err: errors.New("db down")}}
	store := NewSQLCatalogStore(db, func() string { return "plugin-version-id" })

	err := store.UpsertApprovedManifest(context.Background(), validManifest(), "actor-id")
	if err == nil {
		t.Fatal("expected insert error")
	}
	if !db.tx.rolledBack || db.tx.committed {
		t.Fatalf("tx state = %+v, want rollback without commit", db.tx)
	}
}

func TestSQLCatalogStoreLoadsEnabledApprovedManifests(t *testing.T) {
	manifest := validManifest()
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	db := &fakeCatalogDB{rows: &fakeCatalogRows{values: [][]byte{manifestJSON}}}
	store := NewSQLCatalogStore(db, nil)

	got, err := store.EnabledForGuild(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("EnabledForGuild returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "example-tool" {
		t.Fatalf("manifests = %+v, want example-tool manifest", got)
	}
	if !strings.Contains(db.query, "guild_plugin_installs") || !strings.Contains(db.query, "approved = true") {
		t.Fatalf("query = %q, want enabled approved install lookup", db.query)
	}
}

type fakeCatalogDB struct {
	tx         *fakeCatalogTx
	rows       catalogRows
	err        error
	query      string
	args       []any
	beginCalls int
	queryCalls int
}

func (db *fakeCatalogDB) BeginTx(_ context.Context, _ *sql.TxOptions) (catalogTx, error) {
	db.beginCalls++
	if db.err != nil {
		return nil, db.err
	}
	return db.tx, nil
}

func (db *fakeCatalogDB) QueryContext(_ context.Context, query string, args ...any) (catalogRows, error) {
	db.queryCalls++
	db.query = query
	db.args = args
	return db.rows, db.err
}

type fakeCatalogTx struct {
	queries    []string
	args       [][]any
	err        error
	failOnCall int
	committed  bool
	rolledBack bool
}

func (tx *fakeCatalogTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.queries = append(tx.queries, query)
	tx.args = append(tx.args, args)
	if tx.failOnCall > 0 && len(tx.queries) == tx.failOnCall {
		return nil, tx.err
	}
	return fakeCatalogResult(1), nil
}

func (tx *fakeCatalogTx) Commit() error {
	tx.committed = true
	return nil
}

func (tx *fakeCatalogTx) Rollback() error {
	tx.rolledBack = true
	return nil
}

type fakeCatalogRows struct {
	values [][]byte
	index  int
}

func (r *fakeCatalogRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeCatalogRows) Scan(dest ...any) error {
	target, ok := dest[0].(*[]byte)
	if !ok {
		return errors.New("expected bytes scan target")
	}
	*target = r.values[r.index]
	r.index++
	return nil
}

func (r *fakeCatalogRows) Err() error {
	return nil
}

func (r *fakeCatalogRows) Close() error {
	return nil
}

type fakeCatalogResult int64

func (r fakeCatalogResult) LastInsertId() (int64, error) { return int64(r), nil }
func (r fakeCatalogResult) RowsAffected() (int64, error) { return int64(r), nil }
