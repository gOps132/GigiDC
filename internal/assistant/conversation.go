package assistant

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const ContentStorageMetadataOnly = "metadata_only"

type TurnRole string

const (
	TurnRoleUser      TurnRole = "user"
	TurnRoleAssistant TurnRole = "assistant"
)

type ConversationTurn struct {
	RequestID      string
	Surface        Surface
	GuildID        string
	ChannelID      string
	ActorUserID    string
	Role           TurnRole
	ContentStorage string
	ContentChars   int
	ProviderID     string
	ModelID        string
}

type ConversationRecorder interface {
	RecordTurn(ctx context.Context, turn ConversationTurn) error
}

type conversationExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type SQLConversationStore struct {
	db    conversationExecDB
	newID func() string
}

func NewSQLConversationStore(db conversationExecDB, newID func() string) SQLConversationStore {
	return SQLConversationStore{db: db, newID: newID}
}

func (s SQLConversationStore) RecordTurn(ctx context.Context, turn ConversationTurn) error {
	if s.db == nil {
		return fmt.Errorf("conversation database is required")
	}
	turn = normalizeConversationTurn(turn)
	if err := validateConversationTurn(turn); err != nil {
		return err
	}
	id := ""
	if s.newID != nil {
		id = strings.TrimSpace(s.newID())
	}
	if id == "" {
		return fmt.Errorf("conversation turn ID is required")
	}
	_, err := s.db.ExecContext(ctx, `
insert into assistant_conversation_turns (
  id,
  request_id,
  surface,
  guild_id,
  channel_id,
  actor_user_id,
  role,
  content_storage,
  content_chars,
  provider_id,
  model_id
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`, id,
		turn.RequestID,
		turn.Surface,
		nullConversationStringArg(turn.GuildID),
		nullConversationStringArg(turn.ChannelID),
		turn.ActorUserID,
		turn.Role,
		turn.ContentStorage,
		turn.ContentChars,
		nullConversationStringArg(turn.ProviderID),
		nullConversationStringArg(turn.ModelID),
	)
	if err != nil {
		return fmt.Errorf("insert conversation turn: %w", err)
	}
	return nil
}

func normalizeConversationTurn(turn ConversationTurn) ConversationTurn {
	turn.RequestID = strings.TrimSpace(turn.RequestID)
	turn.Surface = Surface(strings.TrimSpace(string(turn.Surface)))
	turn.GuildID = strings.TrimSpace(turn.GuildID)
	turn.ChannelID = strings.TrimSpace(turn.ChannelID)
	turn.ActorUserID = strings.TrimSpace(turn.ActorUserID)
	turn.Role = TurnRole(strings.TrimSpace(string(turn.Role)))
	turn.ContentStorage = strings.TrimSpace(turn.ContentStorage)
	turn.ProviderID = strings.TrimSpace(turn.ProviderID)
	turn.ModelID = strings.TrimSpace(turn.ModelID)
	if turn.ContentStorage == "" {
		turn.ContentStorage = ContentStorageMetadataOnly
	}
	return turn
}

func validateConversationTurn(turn ConversationTurn) error {
	if turn.RequestID == "" {
		return fmt.Errorf("request ID is required")
	}
	switch turn.Surface {
	case SurfaceDM, SurfaceGuildMention:
	default:
		return fmt.Errorf("unknown conversation surface")
	}
	if turn.ActorUserID == "" {
		return fmt.Errorf("actor user ID is required")
	}
	switch turn.Role {
	case TurnRoleUser, TurnRoleAssistant:
	default:
		return fmt.Errorf("unknown conversation role")
	}
	if turn.ContentStorage != ContentStorageMetadataOnly {
		return fmt.Errorf("conversation content storage must be metadata_only")
	}
	if turn.ContentChars < 0 {
		return fmt.Errorf("content chars must be nonnegative")
	}
	return nil
}

func nullConversationStringArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}
