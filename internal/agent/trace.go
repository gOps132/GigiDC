package agent

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gOps132/GigiDC/internal/audit"
)

type Trace struct {
	Recorder AuditRecorder
	Store    RunStore
	Sink     TraceSink
	Source   string
	RunID    string
	Step     int
}

type TraceEvent struct {
	RunID       string
	StepIndex   int
	Phase       string
	Source      string
	Kind        string
	Status      string
	Reason      string
	Intent      string
	Planner     string
	ToolName    string
	ToolKind    string
	Capability  string
	RoutingMode string
	CreatedAt   time.Time
	Details     map[string]string
}

type TraceRun struct {
	RunID       string
	GuildID     string
	ChannelID   string
	ActorUserID string
	Surface     string
	Status      string
	UpdatedAt   time.Time
	Events      []TraceEvent
}

type TraceQuery struct {
	GuildID     string
	ChannelID   string
	ActorUserID string
}

type TraceSink interface {
	RecordTraceEvent(context.Context, Request, TraceEvent) error
}

type MultiTraceSink []TraceSink

func (s MultiTraceSink) RecordTraceEvent(ctx context.Context, request Request, event TraceEvent) error {
	for _, sink := range s {
		if sink == nil {
			continue
		}
		if err := sink.RecordTraceEvent(ctx, request, event); err != nil {
			return err
		}
	}
	return nil
}

type TraceReader interface {
	LastTrace(context.Context, TraceQuery) (TraceRun, bool, error)
}

type MemoryTraceStore struct {
	mu      sync.Mutex
	maxRuns int
	clock   func() time.Time
	runs    map[string]TraceRun
	keys    map[string]string
	order   []string
}

func NewMemoryTraceStore(maxRuns int) *MemoryTraceStore {
	if maxRuns <= 0 {
		maxRuns = 256
	}
	return &MemoryTraceStore{
		maxRuns: maxRuns,
		clock:   time.Now,
		runs:    map[string]TraceRun{},
		keys:    map[string]string{},
	}
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
	if t.Sink == nil {
		t.Sink = parent.Sink
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
	cleanMetadata := sanitizeTraceMetadata(metadata)
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
	event := TraceEvent{
		RunID:       cleanMetadata["run_id"],
		StepIndex:   t.Step,
		Phase:       tracePhase(kind),
		Source:      source,
		Kind:        safeAuditValue(kind),
		Status:      safeAuditValue(string(status)),
		Reason:      safeAuditValue(reason),
		Intent:      cleanMetadata["intent"],
		Planner:     cleanMetadata["planner"],
		ToolName:    cleanMetadata["tool"],
		ToolKind:    cleanMetadata["kind"],
		Capability:  cleanMetadata["capability"],
		RoutingMode: cleanMetadata["routing_mode"],
		Details:     traceEventDetails(cleanMetadata),
	}
	if t.Sink != nil {
		if err := t.Sink.RecordTraceEvent(ctx, request, event); err != nil {
			return err
		}
	}
	if t.Recorder == nil {
		return nil
	}
	return t.Recorder.Record(ctx, audit.Event{
		Kind:     safeAuditValue(kind),
		GuildID:  request.GuildID,
		ActorID:  request.ActorUserID,
		Status:   status,
		Reason:   safeAuditValue(reason),
		Metadata: cleanMetadata,
	})
}

func sanitizeTraceMetadata(metadata map[string]string) map[string]string {
	sanitized := audit.SanitizeMetadata(metadata)
	cleaned := map[string]string{}
	for key, value := range sanitized {
		key = safeAuditValue(key)
		value = safeAuditValue(value)
		if key != "" {
			cleaned[key] = value
		}
	}
	return cleaned
}

func traceEventDetails(metadata map[string]string) map[string]string {
	details := map[string]string{}
	for key, value := range metadata {
		switch key {
		case "source", "run_id", "step_index":
			continue
		}
		if key != "" && value != "" {
			details[key] = value
		}
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func (s *MemoryTraceStore) RecordTraceEvent(ctx context.Context, request Request, event TraceEvent) error {
	if s == nil {
		return nil
	}
	key := traceKey(TraceQuery{GuildID: request.GuildID, ChannelID: request.ChannelID, ActorUserID: request.ActorUserID})
	if key == "" || strings.TrimSpace(event.RunID) == "" {
		return nil
	}
	runKey := traceRunKey(key, event.RunID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs == nil {
		s.runs = map[string]TraceRun{}
	}
	if s.keys == nil {
		s.keys = map[string]string{}
	}
	now := s.now()
	event.CreatedAt = now
	run, ok := s.runs[runKey]
	if !ok {
		run = TraceRun{
			RunID:       event.RunID,
			GuildID:     strings.TrimSpace(request.GuildID),
			ChannelID:   strings.TrimSpace(request.ChannelID),
			ActorUserID: strings.TrimSpace(request.ActorUserID),
			Surface:     safeAuditValue(string(request.Surface)),
		}
		s.order = append(s.order, runKey)
	}
	run.Status = event.Status
	run.UpdatedAt = now
	run.Events = append(run.Events, event)
	s.runs[runKey] = run
	s.keys[key] = runKey
	s.trimLocked()
	return nil
}

func (s *MemoryTraceStore) LastTrace(ctx context.Context, query TraceQuery) (TraceRun, bool, error) {
	if s == nil {
		return TraceRun{}, false, nil
	}
	key := traceKey(query)
	if key == "" {
		return TraceRun{}, false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runKey, ok := s.keys[key]
	if !ok {
		return TraceRun{}, false, nil
	}
	run, ok := s.runs[runKey]
	if !ok {
		return TraceRun{}, false, nil
	}
	return copyTraceRun(run), true, nil
}

func (s *MemoryTraceStore) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func (s *MemoryTraceStore) trimLocked() {
	for s.maxRuns > 0 && len(s.order) > s.maxRuns {
		runKey := s.order[0]
		s.order = s.order[1:]
		delete(s.runs, runKey)
		for key, value := range s.keys {
			if value == runKey {
				delete(s.keys, key)
			}
		}
	}
}

func copyTraceRun(run TraceRun) TraceRun {
	run.Events = append([]TraceEvent(nil), run.Events...)
	for index := range run.Events {
		run.Events[index].Details = copyStringMap(run.Events[index].Details)
	}
	return run
}

func traceKey(query TraceQuery) string {
	if strings.TrimSpace(query.GuildID) == "" || strings.TrimSpace(query.ChannelID) == "" || strings.TrimSpace(query.ActorUserID) == "" {
		return ""
	}
	return strings.Join([]string{strings.TrimSpace(query.GuildID), strings.TrimSpace(query.ChannelID), strings.TrimSpace(query.ActorUserID)}, ":")
}

func traceRunKey(scopeKey string, runID string) string {
	return strings.TrimSpace(scopeKey) + ":" + strings.TrimSpace(runID)
}

func tracePhase(kind string) string {
	switch strings.TrimSpace(kind) {
	case "agent.context":
		return "context"
	case "agent.plan":
		return "plan"
	case "agent.tool":
		return "tool"
	case "agent.answer":
		return "answer"
	default:
		return "fallback"
	}
}

func safeAuditValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.NewReplacer("\n", " ", "\r", " ", "`", "'").Replace(value)
	if len(value) > 120 {
		value = value[:120]
	}
	return value
}
