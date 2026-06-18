package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PersonalKeysMode string
type ToolRoutingMode string

const (
	PersonalKeysOff          PersonalKeysMode = "off"
	PersonalKeysDMOnly       PersonalKeysMode = "dm-only"
	PersonalKeysGuildAllowed PersonalKeysMode = "guild-allowed"

	ToolRoutingOff     ToolRoutingMode = "off"
	ToolRoutingDryRun  ToolRoutingMode = "dry-run"
	ToolRoutingEnabled ToolRoutingMode = "enabled"
)

type GuildPolicy struct {
	GuildID          string
	PersonalKeysMode PersonalKeysMode
	ToolRoutingMode  ToolRoutingMode
}

type GuildPolicyInput struct {
	GuildID          string
	PersonalKeysMode PersonalKeysMode
	ToolRoutingMode  ToolRoutingMode
	ActorUserID      string
}

type policyExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type policyQueryDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) usageRow
}

type sqlPolicyQueryDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type SQLPolicyStore struct {
	db any
}

func NewSQLPolicyStore(db any) SQLPolicyStore {
	return SQLPolicyStore{db: db}
}

func DefaultGuildPolicy(guildID string) GuildPolicy {
	return GuildPolicy{GuildID: strings.TrimSpace(guildID), PersonalKeysMode: PersonalKeysOff, ToolRoutingMode: ToolRoutingOff}
}

func ValidatePersonalKeysMode(mode PersonalKeysMode) error {
	switch mode {
	case PersonalKeysOff, PersonalKeysDMOnly, PersonalKeysGuildAllowed:
		return nil
	default:
		return fmt.Errorf("unknown personal keys mode")
	}
}

func ValidateToolRoutingMode(mode ToolRoutingMode) error {
	switch mode {
	case ToolRoutingOff, ToolRoutingDryRun, ToolRoutingEnabled:
		return nil
	default:
		return fmt.Errorf("unknown tool routing mode")
	}
}

func (s SQLPolicyStore) GuildPolicy(ctx context.Context, guildID string) (GuildPolicy, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return GuildPolicy{}, fmt.Errorf("guild ID is required")
	}
	policy := DefaultGuildPolicy(guildID)
	row, err := s.queryRow(ctx, `
select personal_keys_mode, tool_routing_mode
from llm_guild_policies
where guild_id = $1
`, guildID)
	if err != nil {
		return GuildPolicy{}, err
	}
	if err := row.Scan(&policy.PersonalKeysMode, &policy.ToolRoutingMode); err != nil {
		if err == sql.ErrNoRows {
			return policy, nil
		}
		return GuildPolicy{}, fmt.Errorf("query guild llm policy: %w", err)
	}
	if err := ValidatePersonalKeysMode(policy.PersonalKeysMode); err != nil {
		return GuildPolicy{}, err
	}
	if err := ValidateToolRoutingMode(policy.ToolRoutingMode); err != nil {
		return GuildPolicy{}, err
	}
	return policy, nil
}

func (s SQLPolicyStore) SetGuildPolicy(ctx context.Context, input GuildPolicyInput) error {
	input.GuildID = strings.TrimSpace(input.GuildID)
	input.PersonalKeysMode = PersonalKeysMode(strings.TrimSpace(string(input.PersonalKeysMode)))
	input.ToolRoutingMode = ToolRoutingMode(strings.TrimSpace(string(input.ToolRoutingMode)))
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	if input.GuildID == "" {
		return fmt.Errorf("guild ID is required")
	}
	if input.PersonalKeysMode == "" {
		input.PersonalKeysMode = PersonalKeysOff
	}
	if input.ToolRoutingMode == "" {
		input.ToolRoutingMode = ToolRoutingOff
	}
	if err := ValidatePersonalKeysMode(input.PersonalKeysMode); err != nil {
		return err
	}
	if err := ValidateToolRoutingMode(input.ToolRoutingMode); err != nil {
		return err
	}
	if input.ActorUserID == "" {
		return fmt.Errorf("actor user ID is required")
	}
	execDB, ok := s.db.(policyExecDB)
	if s.db == nil || !ok {
		return fmt.Errorf("policy exec database is required")
	}
	_, err := execDB.ExecContext(ctx, `
insert into llm_guild_policies (
  guild_id,
  personal_keys_mode,
  tool_routing_mode,
  updated_by_user_id,
  updated_at
) values ($1, $2, $3, $4, now())
on conflict (guild_id) do update set
  personal_keys_mode = excluded.personal_keys_mode,
  tool_routing_mode = excluded.tool_routing_mode,
  updated_by_user_id = excluded.updated_by_user_id,
  updated_at = now()
`, input.GuildID, input.PersonalKeysMode, input.ToolRoutingMode, input.ActorUserID)
	if err != nil {
		return fmt.Errorf("upsert guild llm policy: %w", err)
	}
	return nil
}

func (s SQLPolicyStore) queryRow(ctx context.Context, query string, args ...any) (usageRow, error) {
	if s.db == nil {
		return nil, fmt.Errorf("policy query database is required")
	}
	if queryDB, ok := s.db.(policyQueryDB); ok {
		return queryDB.QueryRowContext(ctx, query, args...), nil
	}
	if queryDB, ok := s.db.(sqlPolicyQueryDB); ok {
		return queryDB.QueryRowContext(ctx, query, args...), nil
	}
	return nil, fmt.Errorf("policy query database is required")
}
