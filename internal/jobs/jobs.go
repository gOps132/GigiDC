package jobs

import (
	"context"
	"encoding/json"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Job struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Status      Status          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	RunAfter    time.Time       `json:"run_after"`
	LockedBy    string          `json:"locked_by,omitempty"`
	LockedAt    time.Time       `json:"locked_at,omitempty"`
	LastError   string          `json:"last_error,omitempty"`
}

type Queue interface {
	Enqueue(ctx context.Context, job Job) error
	Claim(ctx context.Context, workerID string, limit int) ([]Job, error)
	Complete(ctx context.Context, id string) error
	Fail(ctx context.Context, id string, cause error) error
}
