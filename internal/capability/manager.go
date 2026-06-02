package capability

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type SQLExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type SQLGrantManager struct {
	db    SQLExecDB
	newID func() string
}

func NewSQLGrantManager(db SQLExecDB, newID func() string) SQLGrantManager {
	return SQLGrantManager{db: db, newID: newID}
}

func (m SQLGrantManager) GrantRole(ctx context.Context, guildID string, roleID string, cap Capability, actorID string) error {
	return m.grant(ctx, "role_capability_grants", "role_id", guildID, roleID, cap, actorID)
}

func (m SQLGrantManager) RevokeRole(ctx context.Context, guildID string, roleID string, cap Capability) error {
	return m.revoke(ctx, "role_capability_grants", "role_id", guildID, roleID, cap)
}

func (m SQLGrantManager) GrantUser(ctx context.Context, guildID string, userID string, cap Capability, actorID string) error {
	return m.grant(ctx, "user_capability_grants", "user_id", guildID, userID, cap, actorID)
}

func (m SQLGrantManager) RevokeUser(ctx context.Context, guildID string, userID string, cap Capability) error {
	return m.revoke(ctx, "user_capability_grants", "user_id", guildID, userID, cap)
}

func (m SQLGrantManager) grant(ctx context.Context, table string, targetColumn string, guildID string, targetID string, cap Capability, actorID string) error {
	if err := validateGrant(guildID, targetID, cap); err != nil {
		return err
	}
	if m.db == nil {
		return fmt.Errorf("capability grant database is required")
	}
	id := ""
	if m.newID != nil {
		id = m.newID()
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("capability grant ID is required")
	}
	if err := m.ensureGuild(ctx, guildID); err != nil {
		return err
	}

	_, err := m.db.ExecContext(ctx, fmt.Sprintf(`
insert into %s (id, guild_id, %s, capability, created_by_user_id)
values ($1, $2, $3, $4, $5)
on conflict (guild_id, %s, capability) do nothing
`, table, targetColumn, targetColumn), id, strings.TrimSpace(guildID), strings.TrimSpace(targetID), string(cap), strings.TrimSpace(actorID))
	if err != nil {
		return fmt.Errorf("grant capability: %w", err)
	}
	return nil
}

func (m SQLGrantManager) revoke(ctx context.Context, table string, targetColumn string, guildID string, targetID string, cap Capability) error {
	if err := validateGrant(guildID, targetID, cap); err != nil {
		return err
	}
	if m.db == nil {
		return fmt.Errorf("capability grant database is required")
	}
	_, err := m.db.ExecContext(ctx, fmt.Sprintf(`
delete from %s
where guild_id = $1
  and %s = $2
  and capability = $3
`, table, targetColumn), strings.TrimSpace(guildID), strings.TrimSpace(targetID), string(cap))
	if err != nil {
		return fmt.Errorf("revoke capability: %w", err)
	}
	return nil
}

func (m SQLGrantManager) ensureGuild(ctx context.Context, guildID string) error {
	_, err := m.db.ExecContext(ctx, `
insert into guilds (id)
values ($1)
on conflict (id) do nothing
`, strings.TrimSpace(guildID))
	if err != nil {
		return fmt.Errorf("ensure guild: %w", err)
	}
	return nil
}

func validateGrant(guildID string, targetID string, cap Capability) error {
	if strings.TrimSpace(guildID) == "" {
		return fmt.Errorf("guild ID is required")
	}
	if strings.TrimSpace(targetID) == "" {
		return fmt.Errorf("target ID is required")
	}
	if _, err := Normalize(string(cap)); err != nil {
		return err
	}
	return nil
}
