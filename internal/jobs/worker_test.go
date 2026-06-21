package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

type fakeQueue struct {
	mu        sync.Mutex
	enqueued  []Job
	claimed   []Job
	completed []string
	failed    map[string]string
}

func (q *fakeQueue) Enqueue(ctx context.Context, job Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueued = append(q.enqueued, job)
	return nil
}

func (q *fakeQueue) Claim(ctx context.Context, workerID string, limit int) ([]Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.enqueued) == 0 {
		return nil, nil
	}
	count := limit
	if count > len(q.enqueued) {
		count = len(q.enqueued)
	}
	claimed := q.enqueued[:count]
	q.enqueued = q.enqueued[count:]
	for i := range claimed {
		claimed[i].Status = StatusRunning
		claimed[i].Attempts++
	}
	q.claimed = append(q.claimed, claimed...)
	return claimed, nil
}

func (q *fakeQueue) Complete(ctx context.Context, id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.completed = append(q.completed, id)
	return nil
}

func (q *fakeQueue) Fail(ctx context.Context, id string, cause error) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.failed == nil {
		q.failed = make(map[string]string)
	}
	var errStr string
	if cause != nil {
		errStr = cause.Error()
	}
	q.failed[id] = errStr
	return nil
}

func TestWorkerPoolSuccess(t *testing.T) {
	fq := &fakeQueue{
		enqueued: []Job{
			{
				ID:      "job_1",
				Kind:    "test_task",
				Payload: json.RawMessage(`{"val": 42}`),
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	pool := NewWorkerPool(fq, logger, WorkerOptions{
		MaxWorkers:   2,
		PollInterval: 10 * time.Millisecond,
	})

	var calledVal int
	var mu sync.Mutex
	done := make(chan struct{})

	pool.Register("test_task", func(ctx context.Context, payload json.RawMessage) error {
		mu.Lock()
		var args struct{ Val int }
		_ = json.Unmarshal(payload, &args)
		calledVal = args.Val
		mu.Unlock()
		close(done)
		return nil
	})

	pool.Start(context.Background())
	defer pool.Stop()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for job to execute")
	}

	mu.Lock()
	defer mu.Unlock()
	if calledVal != 42 {
		t.Errorf("expected calledVal 42, got %d", calledVal)
	}

	fq.mu.Lock()
	defer fq.mu.Unlock()
	if len(fq.completed) != 1 || fq.completed[0] != "job_1" {
		t.Errorf("expected job_1 to be completed, got: %+v", fq.completed)
	}
}

func TestWorkerPoolFailure(t *testing.T) {
	fq := &fakeQueue{
		enqueued: []Job{
			{
				ID:      "job_2",
				Kind:    "failing_task",
				Payload: json.RawMessage(`{}`),
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	pool := NewWorkerPool(fq, logger, WorkerOptions{
		MaxWorkers:   2,
		PollInterval: 10 * time.Millisecond,
	})

	done := make(chan struct{})

	pool.Register("failing_task", func(ctx context.Context, payload json.RawMessage) error {
		defer close(done)
		return errors.New("boom")
	})

	pool.Start(context.Background())
	defer pool.Stop()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for job to execute")
	}

	// Give a tiny moment for post-processing queue updates to write
	time.Sleep(10 * time.Millisecond)

	fq.mu.Lock()
	defer fq.mu.Unlock()
	if fq.failed["job_2"] != "boom" {
		t.Errorf("expected job_2 to fail with 'boom', got: %s", fq.failed["job_2"])
	}
}

func TestWorkerPoolFailsUnknownJobKind(t *testing.T) {
	fq := &fakeQueue{
		enqueued: []Job{{
			ID:      "job_unknown",
			Kind:    "missing_task",
			Payload: json.RawMessage(`{}`),
		}},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	pool := NewWorkerPool(fq, logger, WorkerOptions{
		MaxWorkers:   1,
		PollInterval: 10 * time.Millisecond,
	})
	pool.Start(context.Background())
	defer pool.Stop()

	deadline := time.After(500 * time.Millisecond)
	for {
		fq.mu.Lock()
		got := fq.failed["job_unknown"]
		fq.mu.Unlock()
		if got == "no handler registered" {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for unknown job failure, got %q", got)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
