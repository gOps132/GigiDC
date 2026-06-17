package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type CredentialStatus string

const (
	CredentialStatusActive  CredentialStatus = "active"
	CredentialStatusInvalid CredentialStatus = "invalid"
	CredentialStatusRevoked CredentialStatus = "revoked"
)

type TestStatus string

const (
	TestStatusUntested  TestStatus = "untested"
	TestStatusSucceeded TestStatus = "succeeded"
	TestStatusFailed    TestStatus = "failed"
)

type Scope struct {
	OwnerType OwnerType
	GuildID   string
	UserID    string
}

type CredentialRecord struct {
	ID              string
	OwnerType       OwnerType
	GuildID         string
	UserID          string
	ProviderID      ProviderID
	Label           string
	Ciphertext      []byte
	Nonce           []byte
	KeyID           string
	Fingerprint     string
	Status          CredentialStatus
	LastTestStatus  TestStatus
	LastErrorCode   string
	CreatedByUserID string
	UpdatedByUserID string
}

type CredentialInput struct {
	Owner           Scope
	ProviderID      ProviderID
	Label           string
	Ciphertext      []byte
	Nonce           []byte
	KeyID           string
	Fingerprint     string
	CreatedByUserID string
	UpdatedByUserID string
}

type ModelProfile struct {
	ID               string
	OwnerType        OwnerType
	GuildID          string
	UserID           string
	Purpose          Purpose
	CredentialID     string
	ProviderID       ProviderID
	ModelID          string
	ParamsJSON       string
	Enabled          bool
	SelectedByUserID string
}

type ModelProfileInput struct {
	Owner            Scope
	Purpose          Purpose
	CredentialID     string
	ProviderID       ProviderID
	ModelID          string
	ParamsJSON       string
	SelectedByUserID string
}

type llmRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type llmTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (llmRows, error)
	Commit() error
	Rollback() error
}

type llmTxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (llmTx, error)
}

type llmQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (llmRows, error)
}

type SQLStore struct {
	beginTx func(context.Context) (llmTx, error)
	query   func(context.Context, string, ...any) (llmRows, error)
	newID   func() string
}

func NewSQLStore(db any, newID func() string) SQLStore {
	store := SQLStore{newID: newID}
	if beginner, ok := db.(llmTxBeginner); ok {
		store.beginTx = func(ctx context.Context) (llmTx, error) {
			return beginner.BeginTx(ctx, nil)
		}
	} else if beginner, ok := db.(interface {
		BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
	}); ok {
		store.beginTx = func(ctx context.Context) (llmTx, error) {
			tx, err := beginner.BeginTx(ctx, nil)
			if err != nil {
				return nil, err
			}
			return sqlTx{tx: tx}, nil
		}
	}

	if queryDB, ok := db.(llmQueryDB); ok {
		store.query = queryDB.QueryContext
	} else if queryDB, ok := db.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}); ok {
		store.query = func(ctx context.Context, query string, args ...any) (llmRows, error) {
			return queryDB.QueryContext(ctx, query, args...)
		}
	}
	return store
}

func (s SQLStore) UpsertCredential(ctx context.Context, input CredentialInput) (CredentialRecord, error) {
	input, err := normalizeCredentialInput(input)
	if err != nil {
		return CredentialRecord{}, err
	}
	if s.beginTx == nil {
		return CredentialRecord{}, fmt.Errorf("llm provider transaction is required")
	}
	if s.newID == nil {
		return CredentialRecord{}, fmt.Errorf("credential ID generator is required")
	}
	credentialID := strings.TrimSpace(s.newID())
	if credentialID == "" {
		return CredentialRecord{}, fmt.Errorf("credential ID is required")
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return CredentialRecord{}, fmt.Errorf("begin llm provider transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	args := []any{
		credentialID,
		input.Owner.OwnerType,
		nullArg(input.Owner.GuildID),
		nullArg(input.Owner.UserID),
		input.ProviderID,
		input.Label,
		input.Ciphertext,
		input.Nonce,
		input.KeyID,
		input.Fingerprint,
		input.CreatedByUserID,
		input.UpdatedByUserID,
	}
	rows, err := tx.QueryContext(ctx, `
insert into llm_credentials (
  id,
  owner_type,
  guild_id,
  user_id,
  provider_id,
  label,
  credential_ciphertext,
  credential_nonce,
  credential_key_id,
  credential_fingerprint,
  status,
  last_test_status,
  last_error_code,
  created_by_user_id,
  updated_by_user_id,
  revoked_at,
  updated_at
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'active', 'untested', null, $11, $12, null, now())
-- llm_credentials_active_owner_label_idx
on conflict (
  owner_type,
  (coalesce(guild_id, '')),
  (coalesce(user_id, '')),
  (lower(label))
)
where revoked_at is null
do update set
  label = excluded.label,
  credential_ciphertext = excluded.credential_ciphertext,
  credential_nonce = excluded.credential_nonce,
  credential_key_id = excluded.credential_key_id,
  credential_fingerprint = excluded.credential_fingerprint,
  status = 'active',
  last_test_status = 'untested',
  last_error_code = null,
  updated_by_user_id = excluded.updated_by_user_id,
  revoked_at = null,
  updated_at = now()
where llm_credentials.provider_id = excluded.provider_id
returning
  id,
  owner_type,
  guild_id,
  user_id,
  provider_id,
  label,
  credential_key_id,
  credential_fingerprint,
  status,
  last_test_status,
  last_error_code,
  created_by_user_id,
  updated_by_user_id
`, args...)
	if err != nil {
		return CredentialRecord{}, fmt.Errorf("upsert llm credential: %w", err)
	}
	defer rows.Close()

	record, ok, err := scanCredentialRecord(rows)
	if err != nil {
		return CredentialRecord{}, err
	}
	if !ok {
		return CredentialRecord{}, fmt.Errorf("upsert llm credential returned no row")
	}
	if err := rows.Err(); err != nil {
		return CredentialRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return CredentialRecord{}, fmt.Errorf("commit llm provider transaction: %w", err)
	}
	committed = true
	return record, nil
}

func (s SQLStore) RevokeCredential(ctx context.Context, owner Scope, label string, actorID string) error {
	owner, err := normalizeScope(owner)
	if err != nil {
		return err
	}
	label = strings.TrimSpace(label)
	actorID = strings.TrimSpace(actorID)
	if label == "" {
		return fmt.Errorf("credential label is required")
	}
	if actorID == "" {
		return fmt.Errorf("actor user ID is required")
	}
	if s.beginTx == nil {
		return fmt.Errorf("llm provider transaction is required")
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin llm provider transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	args := []any{owner.OwnerType, nullArg(owner.GuildID), nullArg(owner.UserID), label, actorID}
	result, err := tx.ExecContext(ctx, `
update llm_credentials
set
  revoked_at = now(),
  status = 'revoked',
  updated_by_user_id = $5,
  credential_ciphertext = '\x'::bytea,
  credential_nonce = '\x'::bytea,
  updated_at = now()
where owner_type = $1
  and guild_id is not distinct from $2
  and user_id is not distinct from $3
  and lower(label) = lower($4)
  and revoked_at is null
`, args...)
	if err != nil {
		return fmt.Errorf("revoke llm credential: %w", err)
	}
	if err := requireLLMRowsAffected(result, "active credential was not found"); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
update llm_model_profiles
set
  enabled = false,
  selected_by_user_id = $5,
  updated_at = now()
where enabled = true
  and credential_id in (
    select id
    from llm_credentials
    where owner_type = $1
      and guild_id is not distinct from $2
      and user_id is not distinct from $3
      and lower(label) = lower($4)
      and status = 'revoked'
  )
`, args...); err != nil {
		return fmt.Errorf("disable revoked credential model profiles: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit llm provider transaction: %w", err)
	}
	committed = true
	return nil
}

func (s SQLStore) ListCredentials(ctx context.Context, owner Scope) ([]CredentialRecord, error) {
	owner, err := normalizeScope(owner)
	if err != nil {
		return nil, err
	}
	if s.query == nil {
		return nil, fmt.Errorf("llm provider query database is required")
	}
	rows, err := s.query(ctx, `
select
  id,
  owner_type,
  guild_id,
  user_id,
  provider_id,
  label,
  credential_key_id,
  credential_fingerprint,
  status,
  last_test_status,
  last_error_code,
  created_by_user_id,
  updated_by_user_id
from llm_credentials
where owner_type = $1
  and guild_id is not distinct from $2
  and user_id is not distinct from $3
  and revoked_at is null
order by provider_id, lower(label)
`, owner.OwnerType, nullArg(owner.GuildID), nullArg(owner.UserID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CredentialRecord
	for {
		record, ok, err := scanCredentialRecord(rows)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s SQLStore) SelectModelProfile(ctx context.Context, input ModelProfileInput) error {
	input, err := normalizeModelProfileInput(input)
	if err != nil {
		return err
	}
	if s.beginTx == nil {
		return fmt.Errorf("llm provider transaction is required")
	}
	if s.newID == nil {
		return fmt.Errorf("model profile ID generator is required")
	}
	profileID := strings.TrimSpace(s.newID())
	if profileID == "" {
		return fmt.Errorf("model profile ID is required")
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin llm provider transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
update llm_model_profiles
set
  enabled = false,
  selected_by_user_id = $5,
  updated_at = now()
where owner_type = $1
  and guild_id is not distinct from $2
  and user_id is not distinct from $3
  and purpose = $4
  and enabled = true
`, input.Owner.OwnerType, nullArg(input.Owner.GuildID), nullArg(input.Owner.UserID), input.Purpose, input.SelectedByUserID); err != nil {
		return fmt.Errorf("disable old model profile: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
insert into llm_model_profiles (
  id,
  owner_type,
  guild_id,
  user_id,
  purpose,
  credential_id,
  provider_id,
  model_id,
  params,
  enabled,
  selected_by_user_id,
  updated_at
)
select
  $1,
  $2,
  $3,
  $4,
  $5,
  lc.id,
  lc.provider_id,
  $8,
  $9::jsonb,
  true,
  $10,
  now()
from llm_credentials lc
where lc.id = $6
  and lc.provider_id = $7
  and lc.owner_type = $2
  and lc.guild_id is not distinct from $3
  and lc.user_id is not distinct from $4
	  and lc.status = 'active'
	  and lc.revoked_at is null
`, profileID, input.Owner.OwnerType, nullArg(input.Owner.GuildID), nullArg(input.Owner.UserID), input.Purpose, input.CredentialID, input.ProviderID, input.ModelID, input.ParamsJSON, input.SelectedByUserID)
	if err != nil {
		return fmt.Errorf("select model profile: %w", err)
	}
	if err := requireLLMRowsAffected(result, "active credential was not found"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit llm provider transaction: %w", err)
	}
	committed = true
	return nil
}

func (s SQLStore) ActiveModelProfile(ctx context.Context, owner Scope, purpose Purpose) (ModelProfile, error) {
	owner, err := normalizeScope(owner)
	if err != nil {
		return ModelProfile{}, err
	}
	if err := ValidatePurpose(purpose); err != nil {
		return ModelProfile{}, err
	}
	if s.query == nil {
		return ModelProfile{}, fmt.Errorf("llm provider query database is required")
	}
	rows, err := s.query(ctx, `
select
  id,
  owner_type,
  guild_id,
  user_id,
  purpose,
  credential_id,
  provider_id,
  model_id,
  params::text,
  enabled,
  selected_by_user_id
from llm_model_profiles
where owner_type = $1
  and guild_id is not distinct from $2
  and user_id is not distinct from $3
  and purpose = $4
  and enabled = true
order by updated_at desc
limit 1
`, owner.OwnerType, nullArg(owner.GuildID), nullArg(owner.UserID), purpose)
	if err != nil {
		return ModelProfile{}, err
	}
	defer rows.Close()

	profile, ok, err := scanModelProfile(rows)
	if err != nil {
		return ModelProfile{}, err
	}
	if !ok {
		return ModelProfile{}, fmt.Errorf("active model profile was not found")
	}
	if err := rows.Err(); err != nil {
		return ModelProfile{}, err
	}
	return profile, nil
}

func normalizeCredentialInput(input CredentialInput) (CredentialInput, error) {
	scope, err := normalizeScope(input.Owner)
	if err != nil {
		return CredentialInput{}, err
	}
	input.Owner = scope
	input.ProviderID = ProviderID(strings.TrimSpace(string(input.ProviderID)))
	input.Label = strings.TrimSpace(input.Label)
	input.KeyID = strings.TrimSpace(input.KeyID)
	input.Fingerprint = strings.TrimSpace(input.Fingerprint)
	input.CreatedByUserID = strings.TrimSpace(input.CreatedByUserID)
	input.UpdatedByUserID = strings.TrimSpace(input.UpdatedByUserID)
	if err := ValidateProvider(input.ProviderID); err != nil {
		return CredentialInput{}, err
	}
	if input.Label == "" {
		return CredentialInput{}, fmt.Errorf("credential label is required")
	}
	if len(input.Ciphertext) == 0 {
		return CredentialInput{}, fmt.Errorf("credential ciphertext is required")
	}
	if len(input.Nonce) == 0 {
		return CredentialInput{}, fmt.Errorf("credential nonce is required")
	}
	if input.KeyID == "" {
		return CredentialInput{}, fmt.Errorf("credential key ID is required")
	}
	if input.Fingerprint == "" {
		return CredentialInput{}, fmt.Errorf("credential fingerprint is required")
	}
	if input.CreatedByUserID == "" {
		return CredentialInput{}, fmt.Errorf("actor user ID is required")
	}
	if input.UpdatedByUserID == "" {
		input.UpdatedByUserID = input.CreatedByUserID
	}
	return input, nil
}

func normalizeModelProfileInput(input ModelProfileInput) (ModelProfileInput, error) {
	scope, err := normalizeScope(input.Owner)
	if err != nil {
		return ModelProfileInput{}, err
	}
	input.Owner = scope
	input.ProviderID = ProviderID(strings.TrimSpace(string(input.ProviderID)))
	input.CredentialID = strings.TrimSpace(input.CredentialID)
	input.SelectedByUserID = strings.TrimSpace(input.SelectedByUserID)
	input.ParamsJSON = strings.TrimSpace(input.ParamsJSON)
	if err := ValidatePurpose(input.Purpose); err != nil {
		return ModelProfileInput{}, err
	}
	if err := ValidateProvider(input.ProviderID); err != nil {
		return ModelProfileInput{}, err
	}
	modelID, err := ValidateModelID(input.ModelID)
	if err != nil {
		return ModelProfileInput{}, err
	}
	input.ModelID = modelID
	if !SupportsPurpose(input.ProviderID, input.Purpose) {
		return ModelProfileInput{}, fmt.Errorf("provider does not support purpose")
	}
	if input.CredentialID == "" {
		return ModelProfileInput{}, fmt.Errorf("credential ID is required")
	}
	if input.SelectedByUserID == "" {
		return ModelProfileInput{}, fmt.Errorf("actor user ID is required")
	}
	if input.ParamsJSON == "" {
		input.ParamsJSON = "{}"
	}
	if !json.Valid([]byte(input.ParamsJSON)) {
		return ModelProfileInput{}, fmt.Errorf("model profile params must be valid JSON")
	}
	return input, nil
}

func normalizeScope(scope Scope) (Scope, error) {
	scope.OwnerType = OwnerType(strings.TrimSpace(string(scope.OwnerType)))
	scope.GuildID = strings.TrimSpace(scope.GuildID)
	scope.UserID = strings.TrimSpace(scope.UserID)
	if err := ValidateOwnerType(scope.OwnerType); err != nil {
		return Scope{}, err
	}
	switch scope.OwnerType {
	case OwnerGuild:
		if scope.GuildID == "" {
			return Scope{}, fmt.Errorf("guild ID is required")
		}
		if scope.UserID != "" {
			return Scope{}, fmt.Errorf("guild credential cannot have user ID")
		}
	case OwnerUser:
		if scope.UserID == "" {
			return Scope{}, fmt.Errorf("user ID is required")
		}
		if scope.GuildID != "" {
			return Scope{}, fmt.Errorf("user credential cannot have guild ID")
		}
	case OwnerTenant:
		if scope.GuildID != "" || scope.UserID != "" {
			return Scope{}, fmt.Errorf("tenant credential cannot have guild or user ID")
		}
	default:
		return Scope{}, errors.New("unknown owner type")
	}
	return scope, nil
}

func scanCredentialRecord(rows llmRows) (CredentialRecord, bool, error) {
	if !rows.Next() {
		return CredentialRecord{}, false, nil
	}
	var record CredentialRecord
	var guildID sql.NullString
	var userID sql.NullString
	var lastTestStatus sql.NullString
	var lastErrorCode sql.NullString
	var createdByUserID sql.NullString
	var updatedByUserID sql.NullString
	if err := rows.Scan(
		&record.ID,
		&record.OwnerType,
		&guildID,
		&userID,
		&record.ProviderID,
		&record.Label,
		&record.KeyID,
		&record.Fingerprint,
		&record.Status,
		&lastTestStatus,
		&lastErrorCode,
		&createdByUserID,
		&updatedByUserID,
	); err != nil {
		return CredentialRecord{}, false, fmt.Errorf("scan llm credential: %w", err)
	}
	record.GuildID = stringFromNull(guildID)
	record.UserID = stringFromNull(userID)
	record.LastTestStatus = TestStatus(stringFromNull(lastTestStatus))
	record.LastErrorCode = stringFromNull(lastErrorCode)
	record.CreatedByUserID = stringFromNull(createdByUserID)
	record.UpdatedByUserID = stringFromNull(updatedByUserID)
	return record, true, nil
}

func scanModelProfile(rows llmRows) (ModelProfile, bool, error) {
	if !rows.Next() {
		return ModelProfile{}, false, nil
	}
	var profile ModelProfile
	var guildID sql.NullString
	var userID sql.NullString
	var selectedByUserID sql.NullString
	if err := rows.Scan(
		&profile.ID,
		&profile.OwnerType,
		&guildID,
		&userID,
		&profile.Purpose,
		&profile.CredentialID,
		&profile.ProviderID,
		&profile.ModelID,
		&profile.ParamsJSON,
		&profile.Enabled,
		&selectedByUserID,
	); err != nil {
		return ModelProfile{}, false, fmt.Errorf("scan llm model profile: %w", err)
	}
	profile.GuildID = stringFromNull(guildID)
	profile.UserID = stringFromNull(userID)
	profile.SelectedByUserID = stringFromNull(selectedByUserID)
	return profile, true, nil
}

func nullArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func stringFromNull(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func requireLLMRowsAffected(result sql.Result, emptyMessage string) error {
	if result == nil {
		return nil
	}
	rows, err := result.RowsAffected()
	if err == nil && rows == 0 {
		return fmt.Errorf("%s", emptyMessage)
	}
	return nil
}

type sqlTx struct {
	tx *sql.Tx
}

func (tx sqlTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.tx.ExecContext(ctx, query, args...)
}

func (tx sqlTx) QueryContext(ctx context.Context, query string, args ...any) (llmRows, error) {
	return tx.tx.QueryContext(ctx, query, args...)
}

func (tx sqlTx) Commit() error {
	return tx.tx.Commit()
}

func (tx sqlTx) Rollback() error {
	return tx.tx.Rollback()
}
