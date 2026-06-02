package capability

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type grantRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type SQLQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type rowQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (grantRows, error)
}

type SQLGrantStore struct {
	query func(ctx context.Context, query string, args ...any) (grantRows, error)
}

func NewSQLGrantStore(db SQLQueryDB) SQLGrantStore {
	return SQLGrantStore{
		query: func(ctx context.Context, query string, args ...any) (grantRows, error) {
			return db.QueryContext(ctx, query, args...)
		},
	}
}

func newSQLGrantStoreWithRows(db rowQueryDB) SQLGrantStore {
	return SQLGrantStore{query: db.QueryContext}
}

func (s SQLGrantStore) UserCapabilities(ctx context.Context, guildID string, userID string) ([]Capability, error) {
	return s.queryCapabilities(ctx, `
select capability
from user_capability_grants
where guild_id = $1
  and user_id = $2
`, strings.TrimSpace(guildID), strings.TrimSpace(userID))
}

func (s SQLGrantStore) RoleCapabilities(ctx context.Context, guildID string, roleIDs []string) ([]Capability, error) {
	roleIDs = cleanStrings(roleIDs)
	if len(roleIDs) == 0 {
		return nil, nil
	}
	return s.queryCapabilities(ctx, `
select capability
from role_capability_grants
where guild_id = $1
  and role_id = any($2)
`, strings.TrimSpace(guildID), roleIDs)
}

func (s SQLGrantStore) queryCapabilities(ctx context.Context, query string, args ...any) ([]Capability, error) {
	rows, err := s.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var capabilities []Capability
	for rows.Next() {
		var capability string
		if err := rows.Scan(&capability); err != nil {
			return nil, fmt.Errorf("scan capability: %w", err)
		}
		capability = strings.TrimSpace(capability)
		if capability != "" {
			capabilities = append(capabilities, Capability(capability))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return capabilities, nil
}

func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}
