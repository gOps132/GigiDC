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

func TestSQLCatalogStorePersistsPublicDispatchApprovalOutsideManifestJSON(t *testing.T) {
	manifest := validManifest()
	manifest.Permissions = nil
	manifest.Dispatch = DispatchModeSendMessage
	manifest.PublicDispatchAllowed = true
	db := &fakeCatalogDB{tx: &fakeCatalogTx{}}
	store := NewSQLCatalogStore(db, func() string { return "plugin-version-id" })

	if err := store.UpsertApprovedManifest(context.Background(), manifest, "actor-id"); err != nil {
		t.Fatalf("UpsertApprovedManifest returned error: %v", err)
	}
	if !strings.Contains(db.tx.queries[1], "public_dispatch_allowed") {
		t.Fatalf("version upsert query = %q, want public dispatch approval column", db.tx.queries[1])
	}
	if len(db.tx.args[1]) < 8 || db.tx.args[1][6] != true {
		t.Fatalf("version upsert args = %+v, want public dispatch approval arg", db.tx.args[1])
	}
	rawManifest, ok := db.tx.args[1][3].(string)
	if !ok {
		t.Fatalf("manifest arg = %T, want string", db.tx.args[1][3])
	}
	if strings.Contains(rawManifest, "public_dispatch_allowed") {
		t.Fatalf("manifest JSON leaked internal approval flag: %s", rawManifest)
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
	db := &fakeCatalogDB{rows: &fakeCatalogRows{values: [][]byte{manifestJSON}, publicDispatchAllowed: []bool{true}}}
	store := NewSQLCatalogStore(db, nil)

	got, err := store.EnabledForGuild(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("EnabledForGuild returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "example-tool" || !got[0].PublicDispatchAllowed {
		t.Fatalf("manifests = %+v, want example-tool manifest with dispatch approval", got)
	}
	if !strings.Contains(db.query, "guild_plugin_installs") || !strings.Contains(db.query, "approved = true") || !strings.Contains(db.query, "public_dispatch_allowed") {
		t.Fatalf("query = %q, want enabled approved install lookup", db.query)
	}
}

func TestSQLCatalogStoreListsApprovedManifests(t *testing.T) {
	manifest := validManifest()
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	db := &fakeCatalogDB{rows: &fakeCatalogRows{values: [][]byte{manifestJSON}, publicDispatchAllowed: []bool{true}}}
	store := NewSQLCatalogStore(db, nil)

	got, err := store.ApprovedManifests(context.Background())
	if err != nil {
		t.Fatalf("ApprovedManifests returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "example-tool" || !got[0].PublicDispatchAllowed {
		t.Fatalf("manifests = %+v, want example-tool manifest with dispatch approval", got)
	}
	if !strings.Contains(db.query, "pv.approved = true") || !strings.Contains(db.query, "public_dispatch_allowed") {
		t.Fatalf("query = %q, want approved manifest lookup", db.query)
	}
}

func TestSQLCatalogStoreEnablesApprovedPluginForGuild(t *testing.T) {
	db := &fakeCatalogDB{tx: &fakeCatalogTx{}}
	store := NewSQLCatalogStore(db, func() string { return "install-id" })

	err := store.EnableForGuild(context.Background(), "guild-id", "example-tool", "1.0.0", "actor-id")
	if err != nil {
		t.Fatalf("EnableForGuild returned error: %v", err)
	}
	if !db.tx.committed || db.tx.rolledBack {
		t.Fatalf("tx state = %+v, want commit", db.tx)
	}
	if len(db.tx.queries) != 1 || !strings.Contains(db.tx.queries[0], "insert into guild_plugin_installs") {
		t.Fatalf("queries = %+v, want guild install upsert", db.tx.queries)
	}
	if db.tx.args[0][0] != "install-id" || db.tx.args[0][1] != "guild-id" || db.tx.args[0][3] != "example-tool" {
		t.Fatalf("args = %+v, want install/guild/plugin identifiers", db.tx.args)
	}
}

func TestSQLCatalogStoreEnableFailsWhenApprovedVersionMissing(t *testing.T) {
	db := &fakeCatalogDB{tx: &fakeCatalogTx{affectedRows: -1}}
	store := NewSQLCatalogStore(db, func() string { return "install-id" })

	err := store.EnableForGuild(context.Background(), "guild-id", "missing", "1.0.0", "actor-id")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want missing approved version", err)
	}
	if !db.tx.rolledBack || db.tx.committed {
		t.Fatalf("tx state = %+v, want rollback", db.tx)
	}
}

func TestSQLCatalogStoreDisablesPluginForGuild(t *testing.T) {
	db := &fakeCatalogDB{tx: &fakeCatalogTx{}}
	store := NewSQLCatalogStore(db, nil)

	err := store.DisableForGuild(context.Background(), "guild-id", "example-tool", "actor-id")
	if err != nil {
		t.Fatalf("DisableForGuild returned error: %v", err)
	}
	if !db.tx.committed || db.tx.rolledBack {
		t.Fatalf("tx state = %+v, want commit", db.tx)
	}
	if len(db.tx.queries) != 1 || !strings.Contains(db.tx.queries[0], "update guild_plugin_installs") {
		t.Fatalf("queries = %+v, want guild install update", db.tx.queries)
	}
	if db.tx.args[0][0] != "guild-id" || db.tx.args[0][1] != "example-tool" {
		t.Fatalf("args = %+v, want guild/plugin identifiers", db.tx.args)
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
	queries      []string
	args         [][]any
	err          error
	failOnCall   int
	affectedRows int64
	committed    bool
	rolledBack   bool
}

func (tx *fakeCatalogTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.queries = append(tx.queries, query)
	tx.args = append(tx.args, args)
	if tx.failOnCall > 0 && len(tx.queries) == tx.failOnCall {
		return nil, tx.err
	}
	affected := tx.affectedRows
	if affected == 0 {
		affected = 1
	}
	if tx.affectedRows == -1 {
		affected = 0
	}
	return fakeCatalogResult(affected), nil
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
	values                [][]byte
	publicDispatchAllowed []bool
	index                 int
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
	if len(dest) > 1 {
		publicDispatchAllowed, ok := dest[1].(*bool)
		if !ok {
			return errors.New("expected bool scan target")
		}
		if r.index < len(r.publicDispatchAllowed) {
			*publicDispatchAllowed = r.publicDispatchAllowed[r.index]
		}
	}
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
