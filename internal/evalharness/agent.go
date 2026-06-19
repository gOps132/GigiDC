package evalharness

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gOps132/GigiDC/internal/agent"
	"github.com/gOps132/GigiDC/internal/audit"
)

type AgentCase struct {
	Name                         string
	Runner                       agent.Runner
	Request                      agent.Request
	WantHandled                  bool
	WantTextContains             []string
	WantRunStatus                agent.RunStatus
	WantTerminationReason        agent.TerminationReason
	ForbiddenObservabilityValues []string
}

type AgentCaseResult struct {
	Name          string
	Response      agent.Response
	Handled       bool
	Err           error
	Events        []audit.Event
	StartedRuns   []agent.RunRecord
	CompletedRuns []CompletedRun
	Steps         []agent.StepRecord
	Confirmations []agent.ConfirmationRecord
	Failures      []string
}

type CompletedRun struct {
	RunID  string
	Status agent.RunStatus
	Reason agent.TerminationReason
}

func RunAgentCase(ctx context.Context, eval AgentCase) AgentCaseResult {
	recorder := &MemoryAuditRecorder{}
	store := NewMemoryRunStore()
	runner := eval.Runner
	runner.Trace.Recorder = recorder
	runner.RunStore = store
	response, handled, err := runner.Run(ctx, eval.Request)

	result := AgentCaseResult{
		Name:          strings.TrimSpace(eval.Name),
		Response:      response,
		Handled:       handled,
		Err:           err,
		Events:        recorder.Events(),
		StartedRuns:   store.StartedRuns(),
		CompletedRuns: store.CompletedRuns(),
		Steps:         store.Steps(),
		Confirmations: store.Confirmations(),
	}
	result.Failures = evaluateAgentCase(eval, result)
	return result
}

func evaluateAgentCase(eval AgentCase, result AgentCaseResult) []string {
	var failures []string
	if result.Err != nil {
		failures = append(failures, fmt.Sprintf("unexpected error: %v", result.Err))
	}
	if result.Handled != eval.WantHandled {
		failures = append(failures, fmt.Sprintf("handled=%v want %v", result.Handled, eval.WantHandled))
	}
	for _, want := range eval.WantTextContains {
		if !strings.Contains(result.Response.Text, want) {
			failures = append(failures, fmt.Sprintf("response text missing %q", want))
		}
	}
	if eval.WantRunStatus != "" && result.Response.RunStatus != eval.WantRunStatus {
		failures = append(failures, fmt.Sprintf("run status=%q want %q", result.Response.RunStatus, eval.WantRunStatus))
	}
	if eval.WantTerminationReason != "" && result.Response.TerminationReason != eval.WantTerminationReason {
		failures = append(failures, fmt.Sprintf("termination reason=%q want %q", result.Response.TerminationReason, eval.WantTerminationReason))
	}
	observability := result.observabilityText()
	for _, forbidden := range eval.ForbiddenObservabilityValues {
		forbidden = strings.TrimSpace(forbidden)
		if forbidden != "" && strings.Contains(observability, forbidden) {
			failures = append(failures, fmt.Sprintf("observability leaked %q", forbidden))
		}
	}
	return failures
}

func (r AgentCaseResult) observabilityText() string {
	var builder strings.Builder
	for _, event := range r.Events {
		builder.WriteString(event.Kind)
		builder.WriteByte(' ')
		builder.WriteString(event.Reason)
		builder.WriteByte(' ')
		for key, value := range event.Metadata {
			builder.WriteString(key)
			builder.WriteByte('=')
			builder.WriteString(value)
			builder.WriteByte(' ')
		}
	}
	for _, step := range r.Steps {
		builder.WriteString(step.Kind)
		builder.WriteByte(' ')
		builder.WriteString(step.Reason)
		builder.WriteByte(' ')
		for key, value := range step.Observation {
			builder.WriteString(key)
			builder.WriteByte('=')
			builder.WriteString(value)
			builder.WriteByte(' ')
		}
	}
	for _, confirmation := range r.Confirmations {
		builder.WriteString(confirmation.ToolName)
		builder.WriteByte(' ')
		for key, value := range confirmation.Payload {
			builder.WriteString(key)
			builder.WriteByte('=')
			builder.WriteString(value)
			builder.WriteByte(' ')
		}
	}
	return builder.String()
}

type MemoryAuditRecorder struct {
	events []audit.Event
}

func (r *MemoryAuditRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

func (r *MemoryAuditRecorder) Events() []audit.Event {
	return append([]audit.Event(nil), r.events...)
}

type MemoryRunStore struct {
	started       []agent.RunRecord
	completed     []CompletedRun
	steps         []agent.StepRecord
	confirmations []agent.ConfirmationRecord
	cancelRunIDs  map[string]bool
}

func NewMemoryRunStore() *MemoryRunStore {
	return &MemoryRunStore{cancelRunIDs: map[string]bool{}}
}

func (s *MemoryRunStore) StartRun(_ context.Context, record agent.RunRecord) error {
	s.started = append(s.started, record)
	return nil
}

func (s *MemoryRunStore) CompleteRun(_ context.Context, runID string, status agent.RunStatus, reason agent.TerminationReason) error {
	s.completed = append(s.completed, CompletedRun{RunID: strings.TrimSpace(runID), Status: status, Reason: reason})
	return nil
}

func (s *MemoryRunStore) RecordStep(_ context.Context, record agent.StepRecord) error {
	copied := record
	copied.Observation = copyStringMap(record.Observation)
	s.steps = append(s.steps, copied)
	return nil
}

func (s *MemoryRunStore) IsRunCanceled(_ context.Context, runID string) (bool, error) {
	return s.cancelRunIDs[strings.TrimSpace(runID)], nil
}

func (s *MemoryRunStore) RequestCancelRun(_ context.Context, runID string, _ string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return sql.ErrNoRows
	}
	s.cancelRunIDs[runID] = true
	return nil
}

func (s *MemoryRunStore) CreateConfirmation(_ context.Context, record agent.ConfirmationRecord) error {
	copied := record
	copied.Payload = copyStringMap(record.Payload)
	s.confirmations = append(s.confirmations, copied)
	return nil
}

func (s *MemoryRunStore) PendingConfirmation(_ context.Context, runID string) (agent.ConfirmationRecord, bool, error) {
	runID = strings.TrimSpace(runID)
	for index := len(s.confirmations) - 1; index >= 0; index-- {
		confirmation := s.confirmations[index]
		if confirmation.RunID == runID && confirmation.Status == agent.ConfirmationStatusPending {
			confirmation.Payload = copyStringMap(confirmation.Payload)
			return confirmation, true, nil
		}
	}
	return agent.ConfirmationRecord{}, false, nil
}

func (s *MemoryRunStore) ResolveConfirmation(_ context.Context, confirmationID string, status agent.ConfirmationStatus, actorID string) error {
	confirmationID = strings.TrimSpace(confirmationID)
	for index := range s.confirmations {
		if s.confirmations[index].ID == confirmationID && s.confirmations[index].Status == agent.ConfirmationStatusPending {
			s.confirmations[index].Status = status
			s.confirmations[index].ResolvedByUserID = strings.TrimSpace(actorID)
			return nil
		}
	}
	return sql.ErrNoRows
}

func (s *MemoryRunStore) StartedRuns() []agent.RunRecord {
	return append([]agent.RunRecord(nil), s.started...)
}

func (s *MemoryRunStore) CompletedRuns() []CompletedRun {
	return append([]CompletedRun(nil), s.completed...)
}

func (s *MemoryRunStore) Steps() []agent.StepRecord {
	steps := make([]agent.StepRecord, 0, len(s.steps))
	for _, step := range s.steps {
		step.Observation = copyStringMap(step.Observation)
		steps = append(steps, step)
	}
	return steps
}

func (s *MemoryRunStore) Confirmations() []agent.ConfirmationRecord {
	confirmations := make([]agent.ConfirmationRecord, 0, len(s.confirmations))
	for _, confirmation := range s.confirmations {
		confirmation.Payload = copyStringMap(confirmation.Payload)
		confirmations = append(confirmations, confirmation)
	}
	return confirmations
}

func copyStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
