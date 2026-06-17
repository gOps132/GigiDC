package assistant

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestSQLConversationStoreRecordsMetadataOnlyTurn(t *testing.T) {
	db := &fakeConversationDB{}
	store := NewSQLConversationStore(db, func() string { return "turn-id" })

	err := store.RecordTurn(context.Background(), validConversationTurn())
	if err != nil {
		t.Fatalf("RecordTurn returned error: %v", err)
	}
	for _, want := range []string{
		"insert into assistant_conversation_turns",
		"request_id",
		"content_storage",
		"content_chars",
		"provider_id",
		"model_id",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("query = %q, want %q", db.query, want)
		}
	}
	for _, forbidden := range []string{"prompt", "completion", "raw_content", "message_text"} {
		if strings.Contains(db.query, forbidden) {
			t.Fatalf("query = %q, must not include raw text field %q", db.query, forbidden)
		}
	}
	wantArgs := []any{
		"turn-id",
		"request-id",
		SurfaceGuildMention,
		"guild-id",
		"channel-id",
		"actor-id",
		TurnRoleUser,
		ContentStorageMetadataOnly,
		5,
		"openai",
		"model-id",
	}
	if !reflect.DeepEqual(db.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", db.args, wantArgs)
	}
}

func TestSQLConversationStoreRejectsRawContentStorage(t *testing.T) {
	db := &fakeConversationDB{}
	store := NewSQLConversationStore(db, func() string { return "turn-id" })
	turn := validConversationTurn()
	turn.ContentStorage = "raw"

	err := store.RecordTurn(context.Background(), turn)
	if err == nil || !strings.Contains(err.Error(), "metadata_only") {
		t.Fatalf("error = %v, want metadata-only validation", err)
	}
	if db.calls != 0 {
		t.Fatalf("exec calls = %d, want none", db.calls)
	}
}

func TestSQLConversationStoreRejectsInvalidTurn(t *testing.T) {
	tests := []struct {
		name string
		turn ConversationTurn
		want string
	}{
		{name: "missing request", turn: conversationTurnWithRequest(" "), want: "request ID is required"},
		{name: "bad surface", turn: conversationTurnWithSurface("thread"), want: "unknown conversation surface"},
		{name: "missing actor", turn: conversationTurnWithActor(" "), want: "actor user ID is required"},
		{name: "bad role", turn: conversationTurnWithRole("system"), want: "unknown conversation role"},
		{name: "negative chars", turn: conversationTurnWithChars(-1), want: "content chars must be nonnegative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSQLConversationStore(&fakeConversationDB{}, func() string { return "turn-id" })
			err := store.RecordTurn(context.Background(), tt.turn)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func validConversationTurn() ConversationTurn {
	return ConversationTurn{
		RequestID:    "request-id",
		Surface:      SurfaceGuildMention,
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		ActorUserID:  "actor-id",
		Role:         TurnRoleUser,
		ContentChars: 5,
		ProviderID:   "openai",
		ModelID:      "model-id",
	}
}

func conversationTurnWithRequest(requestID string) ConversationTurn {
	turn := validConversationTurn()
	turn.RequestID = requestID
	return turn
}

func conversationTurnWithSurface(surface Surface) ConversationTurn {
	turn := validConversationTurn()
	turn.Surface = surface
	return turn
}

func conversationTurnWithActor(actorID string) ConversationTurn {
	turn := validConversationTurn()
	turn.ActorUserID = actorID
	return turn
}

func conversationTurnWithRole(role TurnRole) ConversationTurn {
	turn := validConversationTurn()
	turn.Role = role
	return turn
}

func conversationTurnWithChars(chars int) ConversationTurn {
	turn := validConversationTurn()
	turn.ContentChars = chars
	return turn
}

type fakeConversationDB struct {
	query string
	args  []any
	calls int
}

func (db *fakeConversationDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.calls++
	db.query = query
	db.args = args
	return fakeConversationResult(1), nil
}

type fakeConversationResult int64

func (r fakeConversationResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r fakeConversationResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
