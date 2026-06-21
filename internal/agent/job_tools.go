package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gOps132/GigiDC/internal/jobs"
)

const (
	ToolJobsSchedule = "jobs.schedule"
	ToolJobsList     = "jobs.list"
	ToolJobsCancel   = "jobs.cancel"
)

// JobsScheduleTool schedules a background task.
type JobsScheduleTool struct {
	Queue        jobs.Queue
	NewJobID     func() string
	AllowedKinds []string
	Now          func() time.Time
}

func (t JobsScheduleTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolJobsSchedule,
		Description: "Schedule an allowed background job. Arguments: task_name, payload (JSON object), run_after_seconds (optional).",
		Kind:        ToolKindWrite,
		Capability:  "job.schedule",
	}
}

func (t JobsScheduleTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if t.Queue == nil {
		return ToolResult{}, fmt.Errorf("jobs queue is required")
	}

	taskName := strings.TrimSpace(call.Args["task_name"])
	if taskName == "" {
		return ToolResult{}, fmt.Errorf("task_name is required")
	}
	if !allowedJobKind(taskName, t.AllowedKinds) {
		return ToolResult{}, fmt.Errorf("unsupported task_name %q", taskName)
	}

	payloadStr := strings.TrimSpace(call.Args["payload"])
	if payloadStr == "" {
		payloadStr = "{}"
	}
	if !json.Valid([]byte(payloadStr)) {
		return ToolResult{}, fmt.Errorf("payload must be valid JSON")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil || payload == nil {
		return ToolResult{}, fmt.Errorf("payload must be a JSON object")
	}

	now := time.Now
	if t.Now != nil {
		now = t.Now
	}
	runAfter := now()
	if secStr, ok := call.Args["run_after_seconds"]; ok {
		if seconds, err := strconv.Atoi(secStr); err == nil && seconds > 0 {
			runAfter = runAfter.Add(time.Duration(seconds) * time.Second)
		}
	}

	if t.NewJobID == nil {
		return ToolResult{}, fmt.Errorf("job ID generator is required")
	}
	jobID := strings.TrimSpace(t.NewJobID())
	if jobID == "" {
		return ToolResult{}, fmt.Errorf("job ID is required")
	}

	job := jobs.Job{
		ID:          jobID,
		Kind:        taskName,
		Payload:     json.RawMessage(payloadStr),
		Status:      jobs.StatusQueued,
		MaxAttempts: 3,
		RunAfter:    runAfter,
	}

	err := t.Queue.Enqueue(ctx, job)
	if err != nil {
		return ToolResult{}, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return ToolResult{
		Name:    ToolJobsSchedule,
		Summary: fmt.Sprintf("Scheduled background task %q (ID: %s) for %s.", taskName, job.ID, runAfter.Format(time.RFC3339)),
		Data: map[string]string{
			"job_id":    job.ID,
			"task_name": taskName,
			"run_after": runAfter.Format(time.RFC3339),
			"status":    string(jobs.StatusQueued),
		},
	}, nil
}

// JobsListTool lists current background jobs.
type JobsListTool struct {
	DB jobs.DB
}

func (t JobsListTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolJobsList,
		Description: "List background jobs. Arguments: status (optional: queued, running, succeeded, failed, cancelled), limit (optional).",
		Kind:        ToolKindRead,
		Capability:  "job.read",
	}
}

func (t JobsListTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if t.DB == nil {
		return ToolResult{}, fmt.Errorf("database access is required")
	}

	status := strings.TrimSpace(call.Args["status"])
	if status != "" && !validJobStatus(status) {
		return ToolResult{}, fmt.Errorf("invalid job status %q", status)
	}
	limit := parseLimit(call.Args["limit"], 10, 25)

	var query string
	var args []any

	if status != "" {
		query = "SELECT id, kind, status, attempts, run_after, last_error IS NOT NULL FROM jobs WHERE status = $1 ORDER BY run_after DESC LIMIT $2"
		args = []any{status, limit}
	} else {
		query = "SELECT id, kind, status, attempts, run_after, last_error IS NOT NULL FROM jobs ORDER BY run_after DESC LIMIT $1"
		args = []any{limit}
	}

	dbRows, err := t.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return ToolResult{}, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer dbRows.Close()

	var resultList []string
	count := 0
	data := make(map[string]string)

	for dbRows.Next() {
		var id, kind, stat string
		var attempts int
		var runAfter time.Time
		var hasError bool

		err := dbRows.Scan(&id, &kind, &stat, &attempts, &runAfter, &hasError)
		if err != nil {
			return ToolResult{}, fmt.Errorf("failed to scan job: %w", err)
		}

		count++
		idxStr := strconv.Itoa(count)
		data["job_id_"+idxStr] = id
		data["kind_"+idxStr] = kind
		data["status_"+idxStr] = stat
		data["has_error_"+idxStr] = strconv.FormatBool(hasError)

		errDetail := ""
		if hasError {
			errDetail = " (error recorded)"
		}
		resultList = append(resultList, fmt.Sprintf("- ID: %s | Task: %s | Status: %s | Attempts: %d | RunAfter: %s%s", id, kind, stat, attempts, runAfter.Format(time.RFC3339), errDetail))
	}

	if err := dbRows.Err(); err != nil {
		return ToolResult{}, err
	}

	data["count"] = strconv.Itoa(count)

	summary := "No background jobs found."
	if count > 0 {
		summary = fmt.Sprintf("Background jobs:\n%s", strings.Join(resultList, "\n"))
	}

	return ToolResult{
		Name:    ToolJobsList,
		Summary: summary,
		Data:    data,
	}, nil
}

// JobsCancelTool cancels a pending background job.
type JobsCancelTool struct {
	DB jobs.DB
}

func (t JobsCancelTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ToolJobsCancel,
		Description: "Cancel a queued or failed background job. Arguments: job_id.",
		Kind:        ToolKindWrite,
		Capability:  "job.write",
	}
}

func (t JobsCancelTool) Execute(ctx context.Context, request Request, call ToolCall) (ToolResult, error) {
	if t.DB == nil {
		return ToolResult{}, fmt.Errorf("database access is required")
	}

	jobID := strings.TrimSpace(call.Args["job_id"])
	if jobID == "" {
		return ToolResult{}, fmt.Errorf("job_id is required")
	}

	query := "UPDATE jobs SET status = 'cancelled', locked_by = NULL, locked_at = NULL, updated_at = NOW() WHERE id = $1 AND status IN ('queued', 'failed')"
	res, err := t.DB.ExecContext(ctx, query, jobID)
	if err != nil {
		return ToolResult{}, fmt.Errorf("failed to cancel job: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return ToolResult{}, err
	}

	if affected == 0 {
		return ToolResult{
			Name:    ToolJobsCancel,
			Summary: fmt.Sprintf("Job %s could not be cancelled (either not found or currently running/succeeded).", jobID),
			Data: map[string]string{
				"job_id":    jobID,
				"cancelled": "false",
			},
		}, nil
	}

	return ToolResult{
		Name:    ToolJobsCancel,
		Summary: fmt.Sprintf("Successfully cancelled background job %s.", jobID),
		Data: map[string]string{
			"job_id":    jobID,
			"cancelled": "true",
		},
	}, nil
}

func allowedJobKind(kind string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(candidate) == kind {
			return true
		}
	}
	return false
}

func validJobStatus(status string) bool {
	switch jobs.Status(status) {
	case jobs.StatusQueued, jobs.StatusRunning, jobs.StatusSucceeded, jobs.StatusFailed, jobs.StatusCancelled:
		return true
	default:
		return false
	}
}
