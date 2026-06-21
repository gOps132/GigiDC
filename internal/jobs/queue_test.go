package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLQueueEnqueue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	queue := NewSQLQueue(db, func() string { return "job_123" })

	payload := json.RawMessage(`{"key": "value"}`)
	job := Job{
		Kind:    "test_job",
		Payload: payload,
	}

	mock.ExpectExec("INSERT INTO jobs").
		WithArgs("job_123", "test_job", payload, "queued", 0, 3, sqlmock.AnyArg(), "", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSQLQueueClaim(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	queue := NewSQLQueue(db, nil)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "kind", "payload", "status", "attempts", "max_attempts", "run_after", "locked_by", "locked_at", "last_error"}).
		AddRow("job_1", "test_job", []byte(`{"arg":1}`), "running", 1, 3, now, "worker_1", now, nil)

	mock.ExpectQuery("UPDATE jobs").
		WithArgs(1, "worker_1").
		WillReturnRows(rows)

	jobs, err := queue.Claim(context.Background(), "worker_1", 1)
	if err != nil {
		t.Fatalf("Claim returned error: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].ID != "job_1" || jobs[0].Kind != "test_job" || jobs[0].Status != StatusRunning || jobs[0].LockedBy != "worker_1" || jobs[0].MaxAttempts != 3 {
		t.Errorf("unexpected job: %+v", jobs[0])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSQLQueueComplete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	queue := NewSQLQueue(db, nil)

	mock.ExpectExec("UPDATE jobs").
		WithArgs("job_1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = queue.Complete(context.Background(), "job_1")
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSQLQueueFail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	queue := NewSQLQueue(db, nil)

	mock.ExpectExec("UPDATE jobs").
		WithArgs("job_1", "some error").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = queue.Fail(context.Background(), "job_1", errors.New("some error"))
	if err != nil {
		t.Fatalf("Fail returned error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
