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

type grantTx interface {
	SQLExecDB
	Commit() error
	Rollback() error
}

type sqlTxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type SQLGrantManager struct {
	db      SQLExecDB
	beginTx func(context.Context) (grantTx, error)
	newID   func() string
}

func NewSQLGrantManager(db SQLExecDB, newID func() string) SQLGrantManager {
	manager := SQLGrantManager{db: db, newID: newID}
	if beginner, ok := db.(sqlTxBeginner); ok {
		manager.beginTx = func(ctx context.Context) (grantTx, error) {
			return beginner.BeginTx(ctx, nil)
		}
	}
	return manager
}

type grantTxBeginner interface {
	SQLExecDB
	beginGrantTx(context.Context) (grantTx, error)
}

func newSQLGrantManagerWithTx(db grantTxBeginner, newID func() string) SQLGrantManager {
	return SQLGrantManager{db: db, beginTx: db.beginGrantTx, newID: newID}
}

func (m SQLGrantManager) GrantRole(ctx context.Context, guildID string, roleID string, cap Capability, actorID string) error {
	return m.grant(ctx, "role_capability_grants", "role_id", guildID, roleID, cap, actorID)
}

func (m SQLGrantManager) RevokeRole(ctx context.Context, guildID string, roleID string, cap Capability) error {
	return m.revoke(ctx, "role_capability_grants", "role_id", guildID, roleID, cap)
}

func (m SQLGrantManager) GrantRoleCapabilities(ctx context.Context, guildID string, roleID string, caps []Capability, actorID string) error {
	return m.applyRoleCapabilities(ctx, true, guildID, roleID, caps, actorID)
}

func (m SQLGrantManager) RevokeRoleCapabilities(ctx context.Context, guildID string, roleID string, caps []Capability) error {
	return m.applyRoleCapabilities(ctx, false, guildID, roleID, caps, "")
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
	if err := m.ensureGuild(ctx, m.db, guildID); err != nil {
		return err
	}

	if err := m.grantWithExecutor(ctx, m.db, table, targetColumn, guildID, targetID, cap, actorID); err != nil {
		return err
	}
	return nil
}

func (m SQLGrantManager) grantWithExecutor(ctx context.Context, exec SQLExecDB, table string, targetColumn string, guildID string, targetID string, cap Capability, actorID string) error {
	id := ""
	if m.newID != nil {
		id = m.newID()
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("capability grant ID is required")
	}
	_, err := exec.ExecContext(ctx, fmt.Sprintf(`
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

func (m SQLGrantManager) applyRoleCapabilities(ctx context.Context, grant bool, guildID string, roleID string, caps []Capability, actorID string) error {
	caps, err := normalizeCapabilities(caps)
	if err != nil {
		return err
	}
	if strings.TrimSpace(guildID) == "" {
		return fmt.Errorf("guild ID is required")
	}
	if strings.TrimSpace(roleID) == "" {
		return fmt.Errorf("target ID is required")
	}
	if m.beginTx == nil {
		return fmt.Errorf("capability grant transaction is required")
	}
	tx, err := m.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin capability grant transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if grant {
		if err := m.ensureGuild(ctx, tx, guildID); err != nil {
			return err
		}
		for _, cap := range caps {
			if err := m.grantWithExecutor(ctx, tx, "role_capability_grants", "role_id", guildID, roleID, cap, actorID); err != nil {
				return err
			}
		}
	} else {
		for _, cap := range caps {
			if err := m.revokeWithExecutor(ctx, tx, "role_capability_grants", "role_id", guildID, roleID, cap); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit capability grant transaction: %w", err)
	}
	committed = true
	return nil
}

func (m SQLGrantManager) revokeWithExecutor(ctx context.Context, exec SQLExecDB, table string, targetColumn string, guildID string, targetID string, cap Capability) error {
	_, err := exec.ExecContext(ctx, fmt.Sprintf(`
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

func (m SQLGrantManager) ensureGuild(ctx context.Context, exec SQLExecDB, guildID string) error {
	_, err := exec.ExecContext(ctx, `
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

func normalizeCapabilities(caps []Capability) ([]Capability, error) {
	if len(caps) == 0 {
		return nil, fmt.Errorf("capabilities are required")
	}
	normalized := make([]Capability, 0, len(caps))
	for _, cap := range caps {
		cleaned, err := Normalize(string(cap))
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, cleaned)
	}
	return normalized, nil
}
