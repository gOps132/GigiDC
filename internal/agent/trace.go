package agent

import (
	"context"
	"strconv"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
)

type Trace struct {
	Recorder AuditRecorder
	Store    RunStore
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
	if t.Store == nil {
		t.Store = parent.Store
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
	if strings.TrimSpace(request.ActorUserID) == "" {
		return nil
	}
	cleanMetadata := audit.SanitizeMetadata(metadata)
	if cleanMetadata == nil {
		cleanMetadata = map[string]string{}
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
	if t.Store != nil && strings.TrimSpace(t.RunID) != "" {
		_ = t.Store.RecordStep(ctx, StepRecord{
			RunID:       t.RunID,
			StepIndex:   t.Step,
			Kind:        strings.TrimSpace(kind),
			Status:      status,
			Reason:      strings.TrimSpace(reason),
			Observation: cleanMetadata,
		})
	}
	if t.Recorder == nil {
		return nil
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
