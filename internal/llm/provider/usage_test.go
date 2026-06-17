package provider

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSQLUsageRecorderRecordsValidUsageEvent(t *testing.T) {
	db := &fakeUsageExecDB{}
	recorder := NewSQLUsageRecorder(db, func() string { return "usage-id" })

	err := recorder.RecordUsage(context.Background(), validUsageEvent())
	if err != nil {
		t.Fatalf("RecordUsage returned error: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("exec calls = %d, want 1", db.calls)
	}
	for _, want := range []string{
		"insert into llm_usage_events",
		"id",
		"request_id",
		"actor_user_id",
		"billing_owner_type",
		"billing_owner_id",
		"provider_id",
		"model_id",
		"input_tokens",
		"output_tokens",
		"error_class",
	} {
		if !strings.Contains(db.query, want) {
			t.Fatalf("query = %q, want %q", db.query, want)
		}
	}
	if strings.Contains(db.query, "prompt") || strings.Contains(db.query, "completion") {
		t.Fatalf("query = %q, must not include prompt/completion fields", db.query)
	}
	wantArgs := []any{
		"usage-id",
		"request-id",
		"guild-id",
		"channel-id",
		"actor-id",
		OwnerGuild,
		"guild-id",
		ProviderOpenAI,
		"gpt-4o-mini",
		PurposeChat,
		12,
		34,
		UsageStatusSucceeded,
		"",
	}
	if !reflect.DeepEqual(db.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", db.args, wantArgs)
	}
}

func TestSQLUsageRecorderRejectsMissingActorUserID(t *testing.T) {
	db := &fakeUsageExecDB{}
	recorder := NewSQLUsageRecorder(db, func() string { return "usage-id" })
	event := validUsageEvent()
	event.ActorUserID = ""

	err := recorder.RecordUsage(context.Background(), event)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if db.calls != 0 {
		t.Fatalf("exec calls = %d, want 0", db.calls)
	}
}

func TestSQLUsageRecorderRejectsInvalidBillingOwner(t *testing.T) {
	db := &fakeUsageExecDB{}
	recorder := NewSQLUsageRecorder(db, func() string { return "usage-id" })
	event := validUsageEvent()
	event.BillingOwnerType = "workspace"

	err := recorder.RecordUsage(context.Background(), event)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if db.calls != 0 {
		t.Fatalf("exec calls = %d, want 0", db.calls)
	}
}

func TestSQLUsageRecorderRejectsNegativeTokens(t *testing.T) {
	for _, tt := range []struct {
		name  string
		event UsageEvent
	}{
		{name: "input", event: usageEventWithTokens(-1, 0)},
		{name: "output", event: usageEventWithTokens(0, -1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := &fakeUsageExecDB{}
			recorder := NewSQLUsageRecorder(db, func() string { return "usage-id" })

			err := recorder.RecordUsage(context.Background(), tt.event)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if db.calls != 0 {
				t.Fatalf("exec calls = %d, want 0", db.calls)
			}
		})
	}
}

func TestSQLUsageRecorderRejectsMissingGeneratedID(t *testing.T) {
	for _, tt := range []struct {
		name  string
		newID func() string
	}{
		{name: "missing generator", newID: nil},
		{name: "empty generated id", newID: func() string { return "" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := &fakeUsageExecDB{}
			recorder := NewSQLUsageRecorder(db, tt.newID)

			err := recorder.RecordUsage(context.Background(), validUsageEvent())
			if err == nil {
				t.Fatal("expected generated ID error")
			}
			if db.calls != 0 {
				t.Fatalf("exec calls = %d, want 0", db.calls)
			}
		})
	}
}

func TestSQLUsageRecorderRejectsMissingDB(t *testing.T) {
	recorder := NewSQLUsageRecorder(nil, func() string { return "usage-id" })

	err := recorder.RecordUsage(context.Background(), validUsageEvent())
	if err == nil {
		t.Fatal("expected missing database error")
	}
	if !strings.Contains(err.Error(), "usage database is required") {
		t.Fatalf("error = %q, want missing database error", err.Error())
	}
}

func TestSQLUsageRecorderRejectsUnsupportedProvider(t *testing.T) {
	db := &fakeUsageExecDB{}
	recorder := NewSQLUsageRecorder(db, func() string { return "usage-id" })
	event := validUsageEvent()
	event.ProviderID = "ollama"

	err := recorder.RecordUsage(context.Background(), event)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if db.calls != 0 {
		t.Fatalf("exec calls = %d, want 0", db.calls)
	}
}

func TestSQLUsageRecorderRecordsFailedStatusWithErrorClass(t *testing.T) {
	db := &fakeUsageExecDB{}
	recorder := NewSQLUsageRecorder(db, func() string { return "usage-id" })
	event := validUsageEvent()
	event.Status = UsageStatusFailed
	event.ErrorClass = "provider_timeout"

	err := recorder.RecordUsage(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordUsage returned error: %v", err)
	}
	if db.calls != 1 {
		t.Fatalf("exec calls = %d, want 1", db.calls)
	}
	if got := db.args[12]; got != UsageStatusFailed {
		t.Fatalf("status arg = %#v, want failed", got)
	}
	if got := db.args[13]; got != "provider_timeout" {
		t.Fatalf("error class arg = %#v, want provider_timeout", got)
	}
}

func TestSQLUsageRecorderSummarizesGuildUsage(t *testing.T) {
	db := &fakeUsageExecDB{row: fakeUsageRow{scan: func(dest ...any) error {
		*(dest[0].(*int)) = 12
		*(dest[1].(*int)) = 34
		*(dest[2].(*int)) = 3
		*(dest[3].(*int)) = 1
		return nil
	}}}
	recorder := NewSQLUsageRecorder(db, nil)

	got, err := recorder.GuildUsageSummary(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildUsageSummary returned error: %v", err)
	}
	if got.BillingOwnerType != OwnerGuild || got.BillingOwnerID != "guild-id" || got.InputTokens != 12 || got.OutputTokens != 34 || got.TotalEvents != 3 || got.FailedEvents != 1 {
		t.Fatalf("summary = %+v, want guild totals", got)
	}
	if !strings.Contains(db.query, "llm_usage_events") || !strings.Contains(db.query, "count(*) filter") || strings.Contains(db.query, "group by") {
		t.Fatalf("query = %q, want usage summary query", db.query)
	}
}

func TestSQLUsageRecorderGuildUsageSummaryReturnsZeroForNoRows(t *testing.T) {
	db := &fakeUsageExecDB{row: fakeUsageRow{err: sql.ErrNoRows}}
	recorder := NewSQLUsageRecorder(db, nil)

	got, err := recorder.GuildUsageSummary(context.Background(), "guild-id")
	if err != nil {
		t.Fatalf("GuildUsageSummary returned error: %v", err)
	}
	if got.BillingOwnerType != OwnerGuild || got.BillingOwnerID != "guild-id" || got.InputTokens != 0 || got.TotalEvents != 0 {
		t.Fatalf("summary = %+v, want zero guild summary", got)
	}
}

func TestSQLUsageRecorderGuildUsageSummaryRejectsMissingGuild(t *testing.T) {
	recorder := NewSQLUsageRecorder(&fakeUsageExecDB{}, nil)

	_, err := recorder.GuildUsageSummary(context.Background(), " ")
	if err == nil || !strings.Contains(err.Error(), "guild ID is required") {
		t.Fatalf("error = %v, want guild ID validation", err)
	}
}

func validUsageEvent() UsageEvent {
	return UsageEvent{
		RequestID:        "request-id",
		GuildID:          "guild-id",
		ChannelID:        "channel-id",
		ActorUserID:      "actor-id",
		BillingOwnerType: OwnerGuild,
		BillingOwnerID:   "guild-id",
		ProviderID:       ProviderOpenAI,
		ModelID:          "gpt-4o-mini",
		Purpose:          PurposeChat,
		InputTokens:      12,
		OutputTokens:     34,
		Status:           UsageStatusSucceeded,
	}
}

func usageEventWithTokens(inputTokens, outputTokens int) UsageEvent {
	event := validUsageEvent()
	event.InputTokens = inputTokens
	event.OutputTokens = outputTokens
	return event
}

type fakeUsageExecDB struct {
	query string
	args  []any
	calls int
	row   fakeUsageRow
}

func (db *fakeUsageExecDB) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	db.calls++
	db.query = query
	db.args = args
	return fakeUsageResult(1), nil
}

func (db *fakeUsageExecDB) QueryRowContext(_ context.Context, query string, args ...any) usageRow {
	db.query = query
	db.args = args
	return db.row
}

type fakeUsageRow struct {
	scan func(...any) error
	err  error
}

func (r fakeUsageRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if r.scan == nil {
		return errors.New("missing scan")
	}
	return r.scan(dest...)
}

type fakeUsageResult int64

func (r fakeUsageResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r fakeUsageResult) RowsAffected() (int64, error) {
	return int64(r), nil
}
