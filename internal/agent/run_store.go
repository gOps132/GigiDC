package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gOps132/GigiDC/internal/audit"
)

type RunStatus string

const (
	RunStatusRunning              RunStatus = "running"
	RunStatusSucceeded            RunStatus = "succeeded"
	RunStatusFailed               RunStatus = "failed"
	RunStatusDenied               RunStatus = "denied"
	RunStatusDryRun               RunStatus = "dry_run"
	RunStatusConfirmationRequired RunStatus = "confirmation_required"
	RunStatusCanceled             RunStatus = "canceled"
)

type TerminationReason string

const (
	TerminationCompleted            TerminationReason = "completed"
	TerminationDryRun               TerminationReason = "dry_run"
	TerminationCanceled             TerminationReason = "canceled"
	TerminationIgnored              TerminationReason = "ignored"
	TerminationRoutingPolicyFailed  TerminationReason = "routing_policy_failed"
	TerminationRoutingOff           TerminationReason = "routing_off"
	TerminationPermissionDenied     TerminationReason = "permission_denied"
	TerminationPermissionFailed     TerminationReason = "permission_check_failed"
	TerminationPlannerFailed        TerminationReason = "planner_failed"
	TerminationClarify              TerminationReason = "clarify"
	TerminationConfirmationRequired TerminationReason = "confirmation_required"
	TerminationStepBudgetExceeded   TerminationReason = "step_budget_exceeded"
	TerminationToolBudgetExceeded   TerminationReason = "tool_budget_exceeded"
	TerminationExecutorFailed       TerminationReason = "executor_failed"
	TerminationLLMBudgetExceeded    TerminationReason = "llm_budget_exceeded"
)

type ConfirmationStatus string

const (
	ConfirmationStatusPending   ConfirmationStatus = "pending"
	ConfirmationStatusConfirmed ConfirmationStatus = "confirmed"
	ConfirmationStatusCanceled  ConfirmationStatus = "canceled"
	ConfirmationStatusExpired   ConfirmationStatus = "expired"
)

type RunRecord struct {
	ID              string
	GuildID         string
	ChannelID       string
	ActorUserID     string
	Surface         Surface
	ContextScope    string
	Status          RunStatus
	MaxSteps        int
	MaxToolCalls    int
	MaxLLMCalls     int
	MaxInputTokens  int
	MaxOutputTokens int
}

type StepRecord struct {
	RunID       string
	StepIndex   int
	Kind        string
	Status      audit.Status
	Reason      string
	Observation map[string]string
}

type ConfirmationRecord struct {
	ID               string
	RunID            string
	StepIndex        int
	Status           ConfirmationStatus
	ToolName         string
	Payload          map[string]string
	CreatedByUserID  string
	ResolvedByUserID string
	ExpiresAt        time.Time
}

type RunStore interface {
	StartRun(context.Context, RunRecord) error
	CompleteRun(context.Context, string, RunStatus, TerminationReason) error
	RecordStep(context.Context, StepRecord) error
	IsRunCanceled(context.Context, string) (bool, error)
	RequestCancelRun(context.Context, string, string) error
	CreateConfirmation(context.Context, ConfirmationRecord) error
	PendingConfirmation(context.Context, string) (ConfirmationRecord, bool, error)
	ResolveConfirmation(context.Context, string, ConfirmationStatus, string) error
}

type runExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type runQueryRowDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type SQLRunStore struct {
	exec     func(context.Context, string, ...any) (sql.Result, error)
	queryRow func(context.Context, string, ...any) runScanner
}

type runScanner interface {
	Scan(dest ...any) error
}

func NewSQLRunStore(db any) SQLRunStore {
	store := SQLRunStore{}
	if execDB, ok := db.(runExecDB); ok {
		store.exec = execDB.ExecContext
	}
	if queryDB, ok := db.(runQueryRowDB); ok {
		store.queryRow = func(ctx context.Context, query string, args ...any) runScanner {
			return queryDB.QueryRowContext(ctx, query, args...)
		}
	}
	return store
}

func (s SQLRunStore) StartRun(ctx context.Context, record RunRecord) error {
	record.ID = strings.TrimSpace(record.ID)
	if record.ID == "" {
		return fmt.Errorf("agent run ID is required")
	}
	if s.exec == nil {
		return fmt.Errorf("agent run exec database is required")
	}
	if record.Status == "" {
		record.Status = RunStatusRunning
	}
	_, err := s.exec(ctx, `
insert into agent_runs (
  id,
  guild_id,
  channel_id,
  actor_user_id,
  surface,
  context_scope,
  status,
	  max_steps,
	  max_tool_calls,
	  max_llm_calls,
	  max_input_tokens,
	  max_output_tokens,
	  updated_at
	)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
on conflict (id) do update set
  status = case
    when agent_runs.status = 'canceled' or agent_runs.cancel_requested_at is not null then agent_runs.status
    else excluded.status
  end,
  termination_reason = case
    when agent_runs.status = 'canceled' or agent_runs.cancel_requested_at is not null then agent_runs.termination_reason
    else null
  end,
  completed_at = case
    when agent_runs.status = 'canceled' or agent_runs.cancel_requested_at is not null then agent_runs.completed_at
    else null
  end,
  updated_at = now()
`, record.ID, strings.TrimSpace(record.GuildID), strings.TrimSpace(record.ChannelID), strings.TrimSpace(record.ActorUserID), string(record.Surface), strings.TrimSpace(record.ContextScope), string(record.Status), record.MaxSteps, record.MaxToolCalls, record.MaxLLMCalls, record.MaxInputTokens, record.MaxOutputTokens)
	if err != nil {
		return fmt.Errorf("start agent run: %w", err)
	}
	return nil
}

func (s SQLRunStore) LoadRun(ctx context.Context, runID string) (RunRecord, bool, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" || s.queryRow == nil {
		return RunRecord{}, false, nil
	}
	var record RunRecord
	var surface string
	var status string
	if err := s.queryRow(ctx, `
select id,
       coalesce(guild_id, ''),
       coalesce(channel_id, ''),
       coalesce(actor_user_id, ''),
       coalesce(surface, ''),
       coalesce(context_scope, ''),
       status,
       max_steps,
       max_tool_calls,
       max_llm_calls,
       max_input_tokens,
       max_output_tokens
from agent_runs
where id = $1
`, runID).Scan(&record.ID, &record.GuildID, &record.ChannelID, &record.ActorUserID, &surface, &record.ContextScope, &status, &record.MaxSteps, &record.MaxToolCalls, &record.MaxLLMCalls, &record.MaxInputTokens, &record.MaxOutputTokens); err != nil {
		if err == sql.ErrNoRows {
			return RunRecord{}, false, nil
		}
		return RunRecord{}, false, fmt.Errorf("load agent run: %w", err)
	}
	record.Surface = Surface(surface)
	record.Status = RunStatus(status)
	return record, true, nil
}

func (s SQLRunStore) CompleteRun(ctx context.Context, runID string, status RunStatus, reason TerminationReason) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("agent run ID is required")
	}
	if s.exec == nil {
		return fmt.Errorf("agent run exec database is required")
	}
	_, err := s.exec(ctx, `
update agent_runs
set status = case
      when status = 'canceled' or cancel_requested_at is not null then 'canceled'
      else $2
    end,
    termination_reason = case
      when status = 'canceled' or cancel_requested_at is not null then 'canceled'
      else $3
    end,
    completed_at = now(),
    canceled_at = case when $2 = 'canceled' or cancel_requested_at is not null then now() else canceled_at end,
    updated_at = now()
where id = $1
`, runID, string(status), string(reason))
	if err != nil {
		return fmt.Errorf("complete agent run: %w", err)
	}
	return nil
}

func (s SQLRunStore) RecordStep(ctx context.Context, record StepRecord) error {
	record.RunID = strings.TrimSpace(record.RunID)
	if record.RunID == "" {
		return fmt.Errorf("agent run ID is required")
	}
	if s.exec == nil {
		return fmt.Errorf("agent run exec database is required")
	}
	observation, err := json.Marshal(sanitizeStepObservation(record.Observation))
	if err != nil {
		return fmt.Errorf("marshal agent run step observation: %w", err)
	}
	_, err = s.exec(ctx, `
with inserted as (
insert into agent_run_steps (
  run_id,
  step_index,
  kind,
  status,
  reason,
  observation
)
values ($1, $2, $3, $4, $5, $6::jsonb)
returning run_id
)
update agent_runs
set steps_used = steps_used + 1,
    tool_calls_used = tool_calls_used + case when $3 = 'agent.tool' then 1 else 0 end,
    llm_calls_used = llm_calls_used + case when $3 in ('agent.plan', 'agent.answer') then 1 else 0 end,
    updated_at = now()
where id = $1
  and exists (select 1 from inserted)
`, record.RunID, record.StepIndex, strings.TrimSpace(record.Kind), string(record.Status), strings.TrimSpace(record.Reason), string(observation))
	if err != nil {
		return fmt.Errorf("record agent run step: %w", err)
	}
	return nil
}

func (s SQLRunStore) IsRunCanceled(ctx context.Context, runID string) (bool, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" || s.queryRow == nil {
		return false, nil
	}
	var status string
	var cancelRequested bool
	if err := s.queryRow(ctx, `
select status, cancel_requested_at is not null
from agent_runs
where id = $1
	`, runID).Scan(&status, &cancelRequested); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("check agent run cancellation: %w", err)
	}
	return status == string(RunStatusCanceled) || cancelRequested, nil
}

func (s SQLRunStore) RequestCancelRun(ctx context.Context, runID string, actorID string) error {
	runID = strings.TrimSpace(runID)
	actorID = strings.TrimSpace(actorID)
	if runID == "" {
		return fmt.Errorf("agent run ID is required")
	}
	if s.exec == nil {
		return fmt.Errorf("agent run exec database is required")
	}
	result, err := s.exec(ctx, `
update agent_runs
set cancel_requested_at = now(),
    cancel_requested_by_user_id = $2,
    updated_at = now()
where id = $1
  and status = 'running'
  and completed_at is null
  and cancel_requested_at is null
`, runID, actorID)
	if err != nil {
		return fmt.Errorf("request cancel agent run: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("request cancel agent run rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s SQLRunStore) CreateConfirmation(ctx context.Context, record ConfirmationRecord) error {
	record.ID = strings.TrimSpace(record.ID)
	record.RunID = strings.TrimSpace(record.RunID)
	if record.ID == "" {
		return fmt.Errorf("agent confirmation ID is required")
	}
	if record.RunID == "" {
		return fmt.Errorf("agent run ID is required")
	}
	if record.Status == "" {
		record.Status = ConfirmationStatusPending
	}
	if s.exec == nil {
		return fmt.Errorf("agent run exec database is required")
	}
	payload, err := json.Marshal(sanitizeStepObservation(record.Payload))
	if err != nil {
		return fmt.Errorf("marshal agent confirmation payload: %w", err)
	}
	_, err = s.exec(ctx, `
insert into agent_run_confirmations (
  id,
  run_id,
  step_index,
  status,
  tool_name,
  payload,
  created_by_user_id,
  expires_at
)
values ($1, $2, $3, $4, $5, $6::jsonb, $7, nullif($8, '0001-01-01T00:00:00Z')::timestamptz)
`, record.ID, record.RunID, record.StepIndex, string(record.Status), strings.TrimSpace(record.ToolName), string(payload), strings.TrimSpace(record.CreatedByUserID), record.ExpiresAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("create agent confirmation: %w", err)
	}
	return nil
}

func (s SQLRunStore) PendingConfirmation(ctx context.Context, runID string) (ConfirmationRecord, bool, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" || s.queryRow == nil {
		return ConfirmationRecord{}, false, nil
	}
	var record ConfirmationRecord
	var status string
	var payload []byte
	if err := s.queryRow(ctx, `
select id, run_id, step_index, status, tool_name, payload, coalesce(created_by_user_id, ''), coalesce(resolved_by_user_id, '')
from agent_run_confirmations
where run_id = $1
  and status = 'pending'
order by created_at desc
limit 1
`, runID).Scan(&record.ID, &record.RunID, &record.StepIndex, &status, &record.ToolName, &payload, &record.CreatedByUserID, &record.ResolvedByUserID); err != nil {
		if err == sql.ErrNoRows {
			return ConfirmationRecord{}, false, nil
		}
		return ConfirmationRecord{}, false, fmt.Errorf("load pending agent confirmation: %w", err)
	}
	record.Status = ConfirmationStatus(status)
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &record.Payload); err != nil {
			return ConfirmationRecord{}, false, fmt.Errorf("decode agent confirmation payload: %w", err)
		}
	}
	return record, true, nil
}

func (s SQLRunStore) ResolveConfirmation(ctx context.Context, confirmationID string, status ConfirmationStatus, actorID string) error {
	confirmationID = strings.TrimSpace(confirmationID)
	actorID = strings.TrimSpace(actorID)
	if confirmationID == "" {
		return fmt.Errorf("agent confirmation ID is required")
	}
	switch status {
	case ConfirmationStatusConfirmed, ConfirmationStatusCanceled, ConfirmationStatusExpired:
	default:
		return fmt.Errorf("unsupported agent confirmation status")
	}
	if s.exec == nil {
		return fmt.Errorf("agent run exec database is required")
	}
	result, err := s.exec(ctx, `
update agent_run_confirmations
set status = $2,
    resolved_by_user_id = $3,
    confirmed_at = case when $2 = 'confirmed' then now() else confirmed_at end,
    canceled_at = case when $2 = 'canceled' then now() else canceled_at end,
    updated_at = now()
where id = $1
  and status = 'pending'
`, confirmationID, string(status), actorID)
	if err != nil {
		return fmt.Errorf("resolve agent confirmation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("resolve agent confirmation rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func sanitizeStepObservation(observation map[string]string) map[string]string {
	cleaned := map[string]string{}
	for key, value := range audit.SanitizeMetadata(observation) {
		key = safeAuditValue(key)
		if key == "" {
			continue
		}
		cleaned[key] = safeAuditValue(value)
	}
	return cleaned
}
