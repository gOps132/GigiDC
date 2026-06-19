package agent

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/gOps132/GigiDC/internal/audit"
)

func TestSQLRunStorePersistsLifecycleAndSanitizesStepObservation(t *testing.T) {
	db := &fakeRunDB{}
	store := SQLRunStore{
		exec: db.ExecContext,
	}

	if err := store.StartRun(context.Background(), RunRecord{
		ID:           " run-id ",
		GuildID:      "guild-id",
		ChannelID:    "channel-id",
		ActorUserID:  "actor-id",
		Surface:      SurfaceGuildMention,
		ContextScope: "channel",
		MaxSteps:     4,
		MaxToolCalls: 2,
		MaxLLMCalls:  1,
	}); err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if err := store.RecordStep(context.Background(), StepRecord{
		RunID:     "run-id",
		StepIndex: 1,
		Kind:      "agent.tool",
		Status:    audit.StatusSucceeded,
		Reason:    "",
		Observation: map[string]string{
			"tool":    "memory.search",
			"api_key": "sk-secret",
			"value":   "sk-secret",
		},
	}); err != nil {
		t.Fatalf("RecordStep returned error: %v", err)
	}
	if err := store.CompleteRun(context.Background(), "run-id", RunStatusSucceeded, TerminationCompleted); err != nil {
		t.Fatalf("CompleteRun returned error: %v", err)
	}

	if len(db.queries) != 3 {
		t.Fatalf("queries = %+v, want start, step, complete", db.queries)
	}
	if !strings.Contains(db.queries[0], "insert into agent_runs") || !strings.Contains(db.queries[1], "insert into agent_run_steps") || !strings.Contains(db.queries[2], "update agent_runs") {
		t.Fatalf("queries = %+v, want run lifecycle SQL", db.queries)
	}
	stepJSON, ok := db.args[1][5].(string)
	if !ok {
		t.Fatalf("step observation arg = %T, want JSON string", db.args[1][5])
	}
	if strings.Contains(stepJSON, "sk-secret") || strings.Contains(stepJSON, "api_key") || !strings.Contains(stepJSON, "[REDACTED]") {
		t.Fatalf("step observation JSON = %s, want sanitized observation", stepJSON)
	}
}

func TestSQLRunStoreChecksCanceledStatus(t *testing.T) {
	store := SQLRunStore{
		queryRow: func(context.Context, string, ...any) runScanner {
			return fakeRunRow{values: []any{string(RunStatusCanceled), false}}
		},
	}

	canceled, err := store.IsRunCanceled(context.Background(), "run-id")
	if err != nil {
		t.Fatalf("IsRunCanceled returned error: %v", err)
	}
	if !canceled {
		t.Fatal("canceled = false, want true")
	}
}

func TestSQLRunStoreRequestsCancelAndPreservesCancelOnStart(t *testing.T) {
	db := &fakeRunDB{}
	store := SQLRunStore{exec: db.ExecContext}

	if err := store.RequestCancelRun(context.Background(), "run-id", "actor-id"); err != nil {
		t.Fatalf("RequestCancelRun returned error: %v", err)
	}
	if err := store.StartRun(context.Background(), RunRecord{ID: "run-id", Surface: SurfaceGuildMention}); err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if !strings.Contains(db.queries[0], "cancel_requested_at") || !strings.Contains(db.queries[0], "status = 'running'") || !strings.Contains(db.queries[0], "completed_at is null") || !strings.Contains(db.queries[1], "cancel_requested_at is not null") {
		t.Fatalf("queries=%+v, want cancel request and cancel-preserving start", db.queries)
	}
}

func TestSQLRunStoreCompletePreservesCancelRequest(t *testing.T) {
	db := &fakeRunDB{}
	store := SQLRunStore{exec: db.ExecContext}

	if err := store.CompleteRun(context.Background(), "run-id", RunStatusSucceeded, TerminationCompleted); err != nil {
		t.Fatalf("CompleteRun returned error: %v", err)
	}
	for _, want := range []string{"cancel_requested_at is not null", "then 'canceled'", "termination_reason", "canceled_at"} {
		if !strings.Contains(db.queries[0], want) {
			t.Fatalf("complete query=%q, want %q", db.queries[0], want)
		}
	}
}

func TestSQLRunStoreCreatesLoadsAndResolvesConfirmation(t *testing.T) {
	db := &fakeRunDB{}
	store := SQLRunStore{
		exec: db.ExecContext,
		queryRow: func(context.Context, string, ...any) runScanner {
			return fakeRunRow{values: []any{
				"confirmation-id",
				"run-id",
				1,
				string(ConfirmationStatusPending),
				"fake.write",
				[]byte(`{"target":"message-id"}`),
				"actor-id",
				"",
			}}
		},
	}

	if err := store.CreateConfirmation(context.Background(), ConfirmationRecord{
		ID:              "confirmation-id",
		RunID:           "run-id",
		StepIndex:       1,
		Status:          ConfirmationStatusPending,
		ToolName:        "fake.write",
		Payload:         map[string]string{"target": "message-id", "api_key": "sk-secret"},
		CreatedByUserID: "actor-id",
	}); err != nil {
		t.Fatalf("CreateConfirmation returned error: %v", err)
	}
	got, ok, err := store.PendingConfirmation(context.Background(), "run-id")
	if err != nil {
		t.Fatalf("PendingConfirmation returned error: %v", err)
	}
	if !ok || got.ID != "confirmation-id" || got.Payload["target"] != "message-id" {
		t.Fatalf("confirmation=%+v ok=%v, want loaded pending confirmation", got, ok)
	}
	if err := store.ResolveConfirmation(context.Background(), "confirmation-id", ConfirmationStatusCanceled, "resolver-id"); err != nil {
		t.Fatalf("ResolveConfirmation returned error: %v", err)
	}
	payloadJSON := db.args[0][5].(string)
	if strings.Contains(payloadJSON, "api_key") || strings.Contains(payloadJSON, "sk-secret") {
		t.Fatalf("payload JSON=%s, want sanitized confirmation payload", payloadJSON)
	}
	if !strings.Contains(db.queries[1], "resolved_by_user_id") || !strings.Contains(db.queries[1], "canceled_at") || !strings.Contains(db.queries[1], "status = 'pending'") {
		t.Fatalf("resolve query=%q, want resolution metadata", db.queries[1])
	}
}

func TestSQLRunStoreRejectsInvalidConfirmationResolutionStatus(t *testing.T) {
	store := SQLRunStore{exec: (&fakeRunDB{}).ExecContext}

	err := store.ResolveConfirmation(context.Background(), "confirmation-id", ConfirmationStatusPending, "actor-id")
	if err == nil || !strings.Contains(err.Error(), "unsupported agent confirmation status") {
		t.Fatalf("error=%v, want invalid confirmation status", err)
	}
}

type fakeRunDB struct {
	queries []string
	args    [][]any
}

func (db *fakeRunDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.queries = append(db.queries, query)
	db.args = append(db.args, args)
	return fakeRunResult(1), nil
}

type fakeRunResult int64

func (r fakeRunResult) LastInsertId() (int64, error) { return 0, nil }
func (r fakeRunResult) RowsAffected() (int64, error) { return int64(r), nil }

type fakeRunRow struct {
	value  string
	values []any
	err    error
}

func (r fakeRunRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(r.values) > 0 {
		for i := range dest {
			switch d := dest[i].(type) {
			case *string:
				*d = r.values[i].(string)
			case *int:
				*d = r.values[i].(int)
			case *bool:
				*d = r.values[i].(bool)
			case *[]byte:
				*d = r.values[i].([]byte)
			default:
				return sql.ErrNoRows
			}
		}
		return nil
	}
	target := dest[0].(*string)
	*target = r.value
	return nil
}
