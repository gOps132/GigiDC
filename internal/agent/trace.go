package agent

import (
	"context"
	"strconv"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
)

type Trace struct {
	Recorder AuditRecorder
	Source   string
	RunID    string
	Step     int
}

func (t Trace) WithRunID(runID string) Trace {
	t.RunID = strings.TrimSpace(runID)
	return t
}

func (t Trace) WithStep(step int) Trace {
	t.Step = step
	return t
}

func (t Trace) inherit(parent Trace) Trace {
	if t.Recorder == nil {
		t.Recorder = parent.Recorder
	}
	if strings.TrimSpace(t.Source) == "" {
		t.Source = parent.Source
	}
	if strings.TrimSpace(t.RunID) == "" {
		t.RunID = parent.RunID
	}
	return t
}

func (t Trace) Record(ctx context.Context, request Request, kind string, status audit.Status, reason string, metadata map[string]string) error {
	if t.Recorder == nil || strings.TrimSpace(request.ActorUserID) == "" {
		return nil
	}
	cleanMetadata := map[string]string{}
	for key, value := range metadata {
		cleanMetadata[key] = value
	}
	source := strings.TrimSpace(t.Source)
	if source == "" {
		source = "agent"
	}
	cleanMetadata["source"] = source
	if strings.TrimSpace(t.RunID) != "" {
		cleanMetadata["run_id"] = safeAuditValue(t.RunID)
	}
	if t.Step > 0 {
		cleanMetadata["step_index"] = strconv.Itoa(t.Step)
	}
	return t.Recorder.Record(ctx, audit.Event{
		Kind:     kind,
		GuildID:  request.GuildID,
		ActorID:  request.ActorUserID,
		Status:   status,
		Reason:   reason,
		Metadata: cleanMetadata,
	})
}

func safeAuditValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("\n", " ", "\r", " ", "`", "'").Replace(value)
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}
