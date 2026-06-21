package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gOps132/GigiDC/internal/jobs"
)

type mockJobsQueue struct {
	enqueued []jobs.Job
	err      error
}

func (q *mockJobsQueue) Enqueue(ctx context.Context, job jobs.Job) error {
	if q.err != nil {
		return q.err
	}
	q.enqueued = append(q.enqueued, job)
	return nil
}

func (q *mockJobsQueue) Claim(ctx context.Context, workerID string, limit int) ([]jobs.Job, error) {
	return nil, nil
}

func (q *mockJobsQueue) Complete(ctx context.Context, id string) error {
	return nil
}

func (q *mockJobsQueue) Fail(ctx context.Context, id string, cause error) error {
	return nil
}

func TestJobsScheduleToolExecute(t *testing.T) {
	mq := &mockJobsQueue{}
	tool := JobsScheduleTool{
		Queue:        mq,
		NewJobID:     func() string { return "job_sched_123" },
		AllowedKinds: []string{"discord.send_message"},
		Now:          func() time.Time { return time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC) },
	}

	result, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolJobsSchedule,
		Args: map[string]string{
			"task_name":         "discord.send_message",
			"payload":           `{"channel_id":"123","text":"hello"}`,
			"run_after_seconds": "10",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data["job_id"] != "job_sched_123" {
		t.Errorf("expected job_id 'job_sched_123', got %s", result.Data["job_id"])
	}
	if len(mq.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued job, got %d", len(mq.enqueued))
	}
	if mq.enqueued[0].Kind != "discord.send_message" {
		t.Errorf("expected kind 'discord.send_message', got %s", mq.enqueued[0].Kind)
	}
	if mq.enqueued[0].MaxAttempts != 3 {
		t.Errorf("expected max attempts 3, got %d", mq.enqueued[0].MaxAttempts)
	}
}

func TestJobsScheduleToolRejectsUnsupportedKind(t *testing.T) {
	tool := JobsScheduleTool{
		Queue:        &mockJobsQueue{},
		NewJobID:     func() string { return "job_sched_123" },
		AllowedKinds: []string{"allowed.task"},
	}

	_, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolJobsSchedule,
		Args: map[string]string{"task_name": "other.task", "payload": `{}`},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported task_name") {
		t.Fatalf("expected unsupported task error, got %v", err)
	}
}

func TestJobsListToolExecute(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tool := JobsListTool{DB: db}

	rows := sqlmock.NewRows([]string{"id", "kind", "status", "attempts", "run_after", "has_error"}).
		AddRow("job_1", "task_1", "queued", 0, time.Now(), false).
		AddRow("job_2", "task_2", "failed", 2, time.Now(), true)

	mock.ExpectQuery("SELECT id, kind, status, attempts, run_after").
		WithArgs(10).
		WillReturnRows(rows)

	result, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolJobsList,
		Args: map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data["count"] != "2" {
		t.Errorf("expected count 2, got %s", result.Data["count"])
	}
	if strings.Contains(result.Summary, "some failure") || !strings.Contains(result.Summary, "error recorded") || result.Data["has_error_2"] != "true" {
		t.Errorf("expected summary to redact error detail, got %s", result.Summary)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestJobsCancelToolExecute(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	tool := JobsCancelTool{DB: db}

	mock.ExpectExec("UPDATE jobs SET status = 'cancelled'").
		WithArgs("job_to_cancel").
		WillReturnResult(sqlmock.NewResult(0, 1))

	result, err := tool.Execute(context.Background(), Request{}, ToolCall{
		Name: ToolJobsCancel,
		Args: map[string]string{"job_id": "job_to_cancel"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Data["cancelled"] != "true" {
		t.Errorf("expected cancelled true, got %s", result.Data["cancelled"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
