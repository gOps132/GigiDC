package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type ExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type IDFunc func() string

type Store struct {
	db    ExecDB
	newID IDFunc
}

func NewStore(db ExecDB, newID IDFunc) Store {
	return Store{db: db, newID: newID}
}

func (s Store) Record(ctx context.Context, event Event) error {
	event.Metadata = SanitizeMetadata(event.Metadata)
	if err := event.Validate(); err != nil {
		return err
	}
	id := ""
	if s.newID != nil {
		id = s.newID()
	}
	if id == "" {
		return fmt.Errorf("audit ID is required")
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
insert into audit_logs (
  id,
  kind,
  guild_id,
  actor_user_id,
  status,
  reason,
  metadata,
  request_id
) values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8)
`, id, event.Kind, event.GuildID, event.ActorID, event.Status, event.Reason, string(metadata), event.RequestID)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
