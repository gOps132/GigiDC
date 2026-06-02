package storage

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyMigrationsFromDirAppliesSQLInOrder(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "002_second.sql", "create table second ();")
	writeMigration(t, dir, "001_first.sql", "create table first ();")
	writeMigration(t, dir, "notes.txt", "ignored")
	db := &fakeMigrationDB{}

	err := ApplyMigrationsFromDir(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("ApplyMigrationsFromDir returned error: %v", err)
	}
	if len(db.queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(db.queries))
	}
	if !strings.Contains(db.queries[0], "first") || !strings.Contains(db.queries[1], "second") {
		t.Fatalf("queries = %+v, want sorted SQL files", db.queries)
	}
}

func TestApplyMigrationsFromDirRejectsMissingFiles(t *testing.T) {
	err := ApplyMigrationsFromDir(context.Background(), &fakeMigrationDB{}, t.TempDir())
	if err == nil {
		t.Fatal("expected missing migrations error")
	}
}

func TestApplyMigrationsFromDirReturnsExecError(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_first.sql", "create table first ();")
	db := &fakeMigrationDB{err: errors.New("db down")}

	err := ApplyMigrationsFromDir(context.Background(), db, dir)
	if err == nil || !strings.Contains(err.Error(), "apply migration") {
		t.Fatalf("error = %v, want apply migration error", err)
	}
}

func writeMigration(t *testing.T, dir string, name string, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}
}

type fakeMigrationDB struct {
	queries []string
	err     error
}

func (db *fakeMigrationDB) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	db.queries = append(db.queries, query)
	if db.err != nil {
		return nil, db.err
	}
	return fakeMigrationResult(0), nil
}

type fakeMigrationResult int64

func (fakeMigrationResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeMigrationResult) RowsAffected() (int64, error) { return 0, nil }
