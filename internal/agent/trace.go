package agent

import (
	"context"
	"strings"

	"github.com/gOps132/GigiDC/internal/audit"
)

type Trace struct {
	Recorder AuditRecorder
	Source   string
}

func (t Trace) Record(ctx context.Context, request Request, kind string, status audit.Status, reason string, metadata map[string]string) error {
	if t.Recorder == nil || strings.TrimSpace(request.ActorUserID) == "" {
		return nil
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	source := strings.TrimSpace(t.Source)
	if source == "" {
		source = "agent"
	}
	metadata["source"] = source
	return t.Recorder.Record(ctx, audit.Event{
		Kind:     kind,
		GuildID:  request.GuildID,
		ActorID:  request.ActorUserID,
		Status:   status,
		Reason:   reason,
		Metadata: metadata,
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
