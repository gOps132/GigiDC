package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type UsageStatus string

const (
	UsageStatusSucceeded UsageStatus = "succeeded"
	UsageStatusFailed    UsageStatus = "failed"
)

type UsageEvent struct {
	ID               string
	RequestID        string
	GuildID          string
	ChannelID        string
	ActorUserID      string
	BillingOwnerType OwnerType
	BillingOwnerID   string
	ProviderID       ProviderID
	ModelID          string
	Purpose          Purpose
	InputTokens      int
	OutputTokens     int
	Status           UsageStatus
	ErrorClass       string
}

type UsageRecorder interface {
	RecordUsage(ctx context.Context, event UsageEvent) error
}

type UsageSummary struct {
	BillingOwnerType OwnerType
	BillingOwnerID   string
	InputTokens      int
	OutputTokens     int
	TotalEvents      int
	FailedEvents     int
}

type usageExecDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type usageQueryDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) usageRow
}

type sqlUsageQueryDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type usageRow interface {
	Scan(dest ...any) error
}

type SQLUsageRecorder struct {
	db    any
	newID func() string
}

func NewSQLUsageRecorder(db any, newID func() string) SQLUsageRecorder {
	return SQLUsageRecorder{db: db, newID: newID}
}

func (r SQLUsageRecorder) RecordUsage(ctx context.Context, event UsageEvent) error {
	if r.db == nil {
		return fmt.Errorf("usage database is required")
	}
	execDB, ok := r.db.(usageExecDB)
	if !ok {
		return fmt.Errorf("usage exec database is required")
	}
	event = normalizeUsageEvent(event)
	if err := validateUsageEvent(event); err != nil {
		return err
	}
	id := ""
	if r.newID != nil {
		id = strings.TrimSpace(r.newID())
	}
	if id == "" {
		return fmt.Errorf("usage event ID is required")
	}
	_, err := execDB.ExecContext(ctx, `
insert into llm_usage_events (
  id,
  request_id,
  guild_id,
  channel_id,
  actor_user_id,
  billing_owner_type,
  billing_owner_id,
  provider_id,
  model_id,
  purpose,
  input_tokens,
  output_tokens,
  status,
  error_class
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
`, id,
		event.RequestID,
		nullUsageStringArg(event.GuildID),
		nullUsageStringArg(event.ChannelID),
		event.ActorUserID,
		event.BillingOwnerType,
		event.BillingOwnerID,
		event.ProviderID,
		event.ModelID,
		event.Purpose,
		event.InputTokens,
		event.OutputTokens,
		event.Status,
		event.ErrorClass,
	)
	if err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}
	return nil
}

func (r SQLUsageRecorder) GuildUsageSummary(ctx context.Context, guildID string) (UsageSummary, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return UsageSummary{}, fmt.Errorf("guild ID is required")
	}
	summary := UsageSummary{BillingOwnerType: OwnerGuild, BillingOwnerID: guildID}
	row, err := r.queryRow(ctx, `
select
  coalesce(sum(input_tokens), 0),
  coalesce(sum(output_tokens), 0),
  count(*),
  coalesce(sum(case when status = 'failed' then 1 else 0 end), 0)
from llm_usage_events
where guild_id = $1
`, guildID)
	if err != nil {
		return UsageSummary{}, err
	}
	if err := row.Scan(
		&summary.InputTokens,
		&summary.OutputTokens,
		&summary.TotalEvents,
		&summary.FailedEvents,
	); err != nil {
		if err == sql.ErrNoRows {
			return UsageSummary{BillingOwnerType: OwnerGuild, BillingOwnerID: guildID}, nil
		}
		return UsageSummary{}, fmt.Errorf("query guild usage summary: %w", err)
	}
	return summary, nil
}

func (r SQLUsageRecorder) queryRow(ctx context.Context, query string, args ...any) (usageRow, error) {
	if r.db == nil {
		return nil, fmt.Errorf("usage query database is required")
	}
	if queryDB, ok := r.db.(usageQueryDB); ok {
		return queryDB.QueryRowContext(ctx, query, args...), nil
	}
	if queryDB, ok := r.db.(sqlUsageQueryDB); ok {
		return queryDB.QueryRowContext(ctx, query, args...), nil
	}
	return nil, fmt.Errorf("usage query database is required")
}

func normalizeUsageEvent(event UsageEvent) UsageEvent {
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.GuildID = strings.TrimSpace(event.GuildID)
	event.ChannelID = strings.TrimSpace(event.ChannelID)
	event.ActorUserID = strings.TrimSpace(event.ActorUserID)
	event.BillingOwnerType = OwnerType(strings.TrimSpace(string(event.BillingOwnerType)))
	event.BillingOwnerID = strings.TrimSpace(event.BillingOwnerID)
	event.ProviderID = ProviderID(strings.TrimSpace(string(event.ProviderID)))
	event.ModelID = strings.TrimSpace(event.ModelID)
	event.Purpose = Purpose(strings.TrimSpace(string(event.Purpose)))
	event.Status = UsageStatus(strings.TrimSpace(string(event.Status)))
	event.ErrorClass = strings.TrimSpace(event.ErrorClass)
	return event
}

func validateUsageEvent(event UsageEvent) error {
	if event.RequestID == "" {
		return fmt.Errorf("request ID is required")
	}
	if event.ActorUserID == "" {
		return fmt.Errorf("actor user ID is required")
	}
	if err := ValidateOwnerType(event.BillingOwnerType); err != nil {
		return err
	}
	if event.BillingOwnerID == "" {
		return fmt.Errorf("billing owner ID is required")
	}
	if err := ValidateProvider(event.ProviderID); err != nil {
		return err
	}
	modelID, err := ValidateModelID(event.ModelID)
	if err != nil {
		return err
	}
	event.ModelID = modelID
	if err := ValidatePurpose(event.Purpose); err != nil {
		return err
	}
	if !SupportsPurpose(event.ProviderID, event.Purpose) {
		return fmt.Errorf("provider does not support purpose")
	}
	if event.InputTokens < 0 || event.OutputTokens < 0 {
		return fmt.Errorf("usage tokens must be nonnegative")
	}
	switch event.Status {
	case UsageStatusSucceeded, UsageStatusFailed:
		return nil
	default:
		return fmt.Errorf("unknown usage status")
	}
}

func nullUsageStringArg(value string) any {
	if value == "" {
		return nil
	}
	return value
}
