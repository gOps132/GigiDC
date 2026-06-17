package provider

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestSQLPolicyStoreReturnsDefaultOffPolicyWhenMissing(t *testing.T) {
	db := &fakePolicyDB{row: fakeUsageRow{err: sql.ErrNoRows}}
	store := NewSQLPolicyStore(db)

	got, err := store.GuildPolicy(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildPolicy returned error: %v", err)
	}
	if got.GuildID != "guild-id" || got.PersonalKeysMode != PersonalKeysOff {
		t.Fatalf("policy = %+v, want default off", got)
	}
	if !strings.Contains(db.query, "llm_guild_policies") {
		t.Fatalf("query = %q, want policy table lookup", db.query)
	}
}

func TestSQLPolicyStoreLoadsStoredPolicy(t *testing.T) {
	db := &fakePolicyDB{row: fakeUsageRow{scan: func(dest ...any) error {
		*(dest[0].(*PersonalKeysMode)) = PersonalKeysDMOnly
		return nil
	}}}
	store := NewSQLPolicyStore(db)

	got, err := store.GuildPolicy(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildPolicy returned error: %v", err)
	}
	if got.PersonalKeysMode != PersonalKeysDMOnly {
		t.Fatalf("policy = %+v, want dm-only", got)
	}
}

func TestSQLPolicyStoreUpsertsGuildPolicy(t *testing.T) {
	db := &fakePolicyDB{}
	store := NewSQLPolicyStore(db)

	err := store.SetGuildPolicy(context.Background(), GuildPolicyInput{
		GuildID:          "guild-id",
		PersonalKeysMode: PersonalKeysOff,
		ActorUserID:      "actor-id",
	})
	if err != nil {
		t.Fatalf("SetGuildPolicy returned error: %v", err)
	}
	if !strings.Contains(db.query, "insert into llm_guild_policies") || !strings.Contains(db.query, "on conflict") {
		t.Fatalf("query = %q, want policy upsert", db.query)
	}
	if db.args[0] != "guild-id" || db.args[1] != PersonalKeysOff || db.args[2] != "actor-id" {
		t.Fatalf("args = %+v, want guild mode actor", db.args)
	}
}

func TestSQLPolicyStoreRejectsInvalidPolicyInput(t *testing.T) {
	tests := []struct {
		name  string
		input GuildPolicyInput
		want  string
	}{
		{name: "missing guild", input: GuildPolicyInput{PersonalKeysMode: PersonalKeysOff, ActorUserID: "actor-id"}, want: "guild ID is required"},
		{name: "bad mode", input: GuildPolicyInput{GuildID: "guild-id", PersonalKeysMode: "always", ActorUserID: "actor-id"}, want: "unknown personal keys mode"},
		{name: "missing actor", input: GuildPolicyInput{GuildID: "guild-id", PersonalKeysMode: PersonalKeysOff}, want: "actor user ID is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &fakePolicyDB{}
			store := NewSQLPolicyStore(db)
			err := store.SetGuildPolicy(context.Background(), tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if db.calls != 0 {
				t.Fatalf("exec calls = %d, want none", db.calls)
			}
		})
	}
}

type fakePolicyDB struct {
	query string
	args  []any
	calls int
	row   fakeUsageRow
}

func (db *fakePolicyDB) QueryRowContext(_ context.Context, query string, args ...any) usageRow {
	db.query = query
	db.args = args
	return db.row
}

func (db *fakePolicyDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.calls++
	db.query = query
	db.args = args
	return fakeUsageResult(1), nil
}
