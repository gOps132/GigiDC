package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSQLStoreUpsertsCredentialInTransaction(t *testing.T) {
	db := &fakeLLMDB{tx: &fakeLLMTx{rows: fakeCredentialRows(validCredentialRecord())}}
	store := NewSQLStore(db, func() string { return "credential-id" })

	got, err := store.UpsertCredential(context.Background(), validCredentialInput())
	if err != nil {
		t.Fatalf("UpsertCredential returned error: %v", err)
	}
	if got.ID != "credential-id" || got.Status != CredentialStatusActive || got.LastTestStatus != TestStatusUntested {
		t.Fatalf("record = %+v, want generated active untested credential", got)
	}
	if db.beginCalls != 1 || !db.tx.committed || db.tx.rolledBack {
		t.Fatalf("tx state = %+v, begin calls = %d; want committed transaction", db.tx, db.beginCalls)
	}
	if len(db.tx.queries) != 1 || !strings.Contains(db.tx.queries[0], "insert into llm_credentials") {
		t.Fatalf("queries = %+v, want credential upsert", db.tx.queries)
	}
	if !strings.Contains(db.tx.queries[0], "on conflict") || !strings.Contains(db.tx.queries[0], "llm_credentials_active_owner_label_idx") {
		t.Fatalf("query = %q, want active owner/provider/label conflict target", db.tx.queries[0])
	}
	if !strings.Contains(db.tx.queries[0], "where llm_credentials.provider_id = excluded.provider_id") {
		t.Fatalf("query = %q, want provider mismatch guard on credential label conflict", db.tx.queries[0])
	}
	if !strings.Contains(db.tx.queries[0], "label = excluded.label") {
		t.Fatalf("query = %q, want canonical label update on credential rotation", db.tx.queries[0])
	}
	if containsStringArg(db.tx.args, "sk-should-never-appear") {
		t.Fatalf("args leaked plaintext secret: %+v", db.tx.args)
	}
}

func TestSQLStoreRejectsInvalidCredentialScope(t *testing.T) {
	tests := []struct {
		name  string
		input CredentialInput
		want  string
	}{
		{
			name:  "guild missing guild id",
			input: validCredentialInputWithScope(Scope{OwnerType: OwnerGuild}),
			want:  "guild ID is required",
		},
		{
			name:  "user missing user id",
			input: validCredentialInputWithScope(Scope{OwnerType: OwnerUser}),
			want:  "user ID is required",
		},
		{
			name:  "tenant has guild id",
			input: validCredentialInputWithScope(Scope{OwnerType: OwnerTenant, GuildID: "guild-id"}),
			want:  "tenant credential cannot have guild or user ID",
		},
		{
			name: "missing sealed secret fields",
			input: CredentialInput{
				Owner:           Scope{OwnerType: OwnerGuild, GuildID: "guild-id"},
				ProviderID:      ProviderOpenAI,
				Label:           "main",
				CreatedByUserID: "actor-id",
			},
			want: "credential ciphertext is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSQLStore(&fakeLLMDB{tx: &fakeLLMTx{}}, func() string { return "credential-id" })
			if _, err := store.UpsertCredential(context.Background(), tt.input); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSQLStoreRollsBackCredentialUpsertWhenQueryFails(t *testing.T) {
	db := &fakeLLMDB{tx: &fakeLLMTx{err: errors.New("db down")}}
	store := NewSQLStore(db, func() string { return "credential-id" })

	_, err := store.UpsertCredential(context.Background(), validCredentialInput())
	if err == nil {
		t.Fatal("expected upsert error")
	}
	if !db.tx.rolledBack || db.tx.committed {
		t.Fatalf("tx state = %+v, want rollback without commit", db.tx)
	}
}

func TestSQLStoreRevokesCredentialAndDisablesProfilesInTransaction(t *testing.T) {
	db := &fakeLLMDB{tx: &fakeLLMTx{}}
	store := NewSQLStore(db, nil)

	err := store.RevokeCredential(context.Background(), Scope{OwnerType: OwnerGuild, GuildID: "guild-id"}, "main", "actor-id")
	if err != nil {
		t.Fatalf("RevokeCredential returned error: %v", err)
	}
	if !db.tx.committed || db.tx.rolledBack {
		t.Fatalf("tx state = %+v, want commit", db.tx)
	}
	if len(db.tx.queries) != 2 {
		t.Fatalf("queries = %d, want revoke plus profile disable", len(db.tx.queries))
	}
	if !strings.Contains(db.tx.queries[0], "update llm_credentials") || !strings.Contains(db.tx.queries[0], "credential_ciphertext = '\\x'::bytea") {
		t.Fatalf("revoke query = %q, want secret bytes cleared", db.tx.queries[0])
	}
	if !strings.Contains(db.tx.queries[1], "update llm_model_profiles") || !strings.Contains(db.tx.queries[1], "enabled = false") {
		t.Fatalf("profile query = %q, want model profiles disabled", db.tx.queries[1])
	}
}

func TestSQLStoreRollsBackRevokeWhenProfileDisableFails(t *testing.T) {
	db := &fakeLLMDB{tx: &fakeLLMTx{failExecOnCall: 2, err: errors.New("db down")}}
	store := NewSQLStore(db, nil)

	err := store.RevokeCredential(context.Background(), Scope{OwnerType: OwnerGuild, GuildID: "guild-id"}, "main", "actor-id")
	if err == nil {
		t.Fatal("expected revoke error")
	}
	if !db.tx.rolledBack || db.tx.committed {
		t.Fatalf("tx state = %+v, want rollback without commit", db.tx)
	}
}

func TestSQLStoreListsCredentialMetadataWithoutSecretBytes(t *testing.T) {
	record := validCredentialRecord()
	record.Ciphertext = []byte("encrypted")
	record.Nonce = []byte("nonce")
	db := &fakeLLMDB{rows: fakeCredentialRows(record)}
	store := NewSQLStore(db, nil)

	got, err := store.ListCredentials(context.Background(), Scope{OwnerType: OwnerGuild, GuildID: "guild-id"})
	if err != nil {
		t.Fatalf("ListCredentials returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "credential-id" {
		t.Fatalf("credentials = %+v, want one credential", got)
	}
	if len(got[0].Ciphertext) != 0 || len(got[0].Nonce) != 0 {
		t.Fatalf("credential leaked sealed bytes: %+v", got[0])
	}
	if strings.Contains(db.query, "credential_ciphertext") || strings.Contains(db.query, "credential_nonce") {
		t.Fatalf("query = %q, want metadata-only select", db.query)
	}
}

func TestSQLStoreSelectsModelProfileByDisablingOldProfileFirst(t *testing.T) {
	db := &fakeLLMDB{tx: &fakeLLMTx{}}
	store := NewSQLStore(db, func() string { return "profile-id" })

	err := store.SelectModelProfile(context.Background(), validModelProfileInput())
	if err != nil {
		t.Fatalf("SelectModelProfile returned error: %v", err)
	}
	if !db.tx.committed || db.tx.rolledBack {
		t.Fatalf("tx state = %+v, want commit", db.tx)
	}
	if len(db.tx.queries) != 2 {
		t.Fatalf("queries = %d, want disable then insert", len(db.tx.queries))
	}
	if !strings.Contains(db.tx.queries[0], "update llm_model_profiles") || !strings.Contains(db.tx.queries[0], "enabled = false") {
		t.Fatalf("first query = %q, want old profile disabled first", db.tx.queries[0])
	}
	if !strings.Contains(db.tx.queries[1], "insert into llm_model_profiles") {
		t.Fatalf("second query = %q, want selected profile insert", db.tx.queries[1])
	}
	for _, want := range []string{
		"from llm_credentials",
		"lc.id = $6",
		"lc.provider_id = $7",
		"lc.owner_type = $2",
		"lc.guild_id is not distinct from $3",
		"lc.user_id is not distinct from $4",
		"lc.status = 'active'",
		"lc.revoked_at is null",
	} {
		if !strings.Contains(db.tx.queries[1], want) {
			t.Fatalf("second query = %q, want active same-owner credential guard %q", db.tx.queries[1], want)
		}
	}
	if db.tx.args[1][0] != "profile-id" || db.tx.args[1][5] != "credential-id" || db.tx.args[1][6] != ProviderOpenAI {
		t.Fatalf("insert args = %+v, want profile/credential/provider IDs", db.tx.args[1])
	}
}

func TestSQLStoreActiveModelProfileLoadsEnabledProfile(t *testing.T) {
	db := &fakeLLMDB{rows: fakeModelProfileRows(validModelProfile())}
	store := NewSQLStore(db, nil)

	got, err := store.ActiveModelProfile(context.Background(), Scope{OwnerType: OwnerGuild, GuildID: "guild-id"}, PurposeChat)
	if err != nil {
		t.Fatalf("ActiveModelProfile returned error: %v", err)
	}
	if got.ID != "profile-id" || got.CredentialID != "credential-id" || got.ParamsJSON != `{"temperature":0.2}` {
		t.Fatalf("profile = %+v, want active profile", got)
	}
	if !strings.Contains(db.query, "enabled = true") || !strings.Contains(db.query, "llm_model_profiles") {
		t.Fatalf("query = %q, want enabled profile lookup", db.query)
	}
}

func validCredentialInput() CredentialInput {
	return CredentialInput{
		Owner:           Scope{OwnerType: OwnerGuild, GuildID: "guild-id"},
		ProviderID:      ProviderOpenAI,
		Label:           "main",
		Ciphertext:      []byte("sealed-sk-should-never-appear"),
		Nonce:           []byte("nonce"),
		KeyID:           "key-id",
		Fingerprint:     "fingerprint",
		CreatedByUserID: "actor-id",
		UpdatedByUserID: "actor-id",
	}
}

func validCredentialInputWithScope(scope Scope) CredentialInput {
	input := validCredentialInput()
	input.Owner = scope
	return input
}

func validCredentialRecord() CredentialRecord {
	return CredentialRecord{
		ID:              "credential-id",
		OwnerType:       OwnerGuild,
		GuildID:         "guild-id",
		ProviderID:      ProviderOpenAI,
		Label:           "main",
		KeyID:           "key-id",
		Fingerprint:     "fingerprint",
		Status:          CredentialStatusActive,
		LastTestStatus:  TestStatusUntested,
		CreatedByUserID: "actor-id",
		UpdatedByUserID: "actor-id",
	}
}

func validModelProfileInput() ModelProfileInput {
	return ModelProfileInput{
		Owner:            Scope{OwnerType: OwnerGuild, GuildID: "guild-id"},
		Purpose:          PurposeChat,
		CredentialID:     "credential-id",
		ProviderID:       ProviderOpenAI,
		ModelID:          "gpt-4o-mini",
		ParamsJSON:       `{"temperature":0.2}`,
		SelectedByUserID: "actor-id",
	}
}

func validModelProfile() ModelProfile {
	return ModelProfile{
		ID:               "profile-id",
		OwnerType:        OwnerGuild,
		GuildID:          "guild-id",
		Purpose:          PurposeChat,
		CredentialID:     "credential-id",
		ProviderID:       ProviderOpenAI,
		ModelID:          "gpt-4o-mini",
		ParamsJSON:       `{"temperature":0.2}`,
		Enabled:          true,
		SelectedByUserID: "actor-id",
	}
}

func containsStringArg(args [][]any, needle string) bool {
	for _, argList := range args {
		for _, arg := range argList {
			if s, ok := arg.(string); ok && strings.Contains(s, needle) {
				return true
			}
		}
	}
	return false
}

type fakeLLMDB struct {
	tx         *fakeLLMTx
	rows       llmRows
	err        error
	query      string
	args       []any
	beginCalls int
	queryCalls int
}

func (db *fakeLLMDB) BeginTx(_ context.Context, _ *sql.TxOptions) (llmTx, error) {
	db.beginCalls++
	if db.err != nil {
		return nil, db.err
	}
	return db.tx, nil
}

func (db *fakeLLMDB) QueryContext(_ context.Context, query string, args ...any) (llmRows, error) {
	db.queryCalls++
	db.query = query
	db.args = args
	return db.rows, db.err
}

type fakeLLMTx struct {
	queries        []string
	args           [][]any
	rows           llmRows
	err            error
	failExecOnCall int
	committed      bool
	rolledBack     bool
}

func (tx *fakeLLMTx) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	tx.queries = append(tx.queries, query)
	tx.args = append(tx.args, args)
	if tx.failExecOnCall > 0 && len(tx.queries) == tx.failExecOnCall {
		return nil, tx.err
	}
	return fakeLLMResult(1), nil
}

func (tx *fakeLLMTx) QueryContext(_ context.Context, query string, args ...any) (llmRows, error) {
	tx.queries = append(tx.queries, query)
	tx.args = append(tx.args, args)
	return tx.rows, tx.err
}

func (tx *fakeLLMTx) Commit() error {
	tx.committed = true
	return nil
}

func (tx *fakeLLMTx) Rollback() error {
	tx.rolledBack = true
	return nil
}

type fakeLLMRows struct {
	scans []func(...any) error
	index int
}

func (r *fakeLLMRows) Next() bool {
	return r.index < len(r.scans)
}

func (r *fakeLLMRows) Scan(dest ...any) error {
	if !r.Next() {
		return errors.New("scan past end")
	}
	scan := r.scans[r.index]
	r.index++
	return scan(dest...)
}

func (r *fakeLLMRows) Err() error {
	return nil
}

func (r *fakeLLMRows) Close() error {
	return nil
}

type fakeLLMResult int64

func (r fakeLLMResult) LastInsertId() (int64, error) { return int64(r), nil }
func (r fakeLLMResult) RowsAffected() (int64, error) { return int64(r), nil }

func fakeCredentialRows(records ...CredentialRecord) llmRows {
	rows := &fakeLLMRows{}
	for _, record := range records {
		record := record
		rows.scans = append(rows.scans, func(dest ...any) error {
			if len(dest) != 13 {
				return fmt.Errorf("credential scan targets = %d, want 13", len(dest))
			}
			*(dest[0].(*string)) = record.ID
			*(dest[1].(*OwnerType)) = record.OwnerType
			*(dest[2].(*sql.NullString)) = nullString(record.GuildID)
			*(dest[3].(*sql.NullString)) = nullString(record.UserID)
			*(dest[4].(*ProviderID)) = record.ProviderID
			*(dest[5].(*string)) = record.Label
			*(dest[6].(*string)) = record.KeyID
			*(dest[7].(*string)) = record.Fingerprint
			*(dest[8].(*CredentialStatus)) = record.Status
			*(dest[9].(*sql.NullString)) = nullString(string(record.LastTestStatus))
			*(dest[10].(*sql.NullString)) = nullString(record.LastErrorCode)
			*(dest[11].(*sql.NullString)) = nullString(record.CreatedByUserID)
			*(dest[12].(*sql.NullString)) = nullString(record.UpdatedByUserID)
			return nil
		})
	}
	return rows
}

func fakeModelProfileRows(profiles ...ModelProfile) llmRows {
	rows := &fakeLLMRows{}
	for _, profile := range profiles {
		profile := profile
		rows.scans = append(rows.scans, func(dest ...any) error {
			if len(dest) != 11 {
				return fmt.Errorf("profile scan targets = %d, want 11", len(dest))
			}
			*(dest[0].(*string)) = profile.ID
			*(dest[1].(*OwnerType)) = profile.OwnerType
			*(dest[2].(*sql.NullString)) = nullString(profile.GuildID)
			*(dest[3].(*sql.NullString)) = nullString(profile.UserID)
			*(dest[4].(*Purpose)) = profile.Purpose
			*(dest[5].(*string)) = profile.CredentialID
			*(dest[6].(*ProviderID)) = profile.ProviderID
			*(dest[7].(*string)) = profile.ModelID
			*(dest[8].(*string)) = profile.ParamsJSON
			*(dest[9].(*bool)) = profile.Enabled
			*(dest[10].(*sql.NullString)) = nullString(profile.SelectedByUserID)
			return nil
		})
	}
	return rows
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
