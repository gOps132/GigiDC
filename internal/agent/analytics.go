package agent

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

const analyticsTopToolsLimit = 5

type AnalyticsQuery struct {
	GuildID     string
	ActorUserID string
	ChannelID   string
	Since       time.Time
	Until       time.Time
	Limit       int
}

type AnalyticsSummary struct {
	GuildID           string
	ActorUserID       string
	ChannelID         string
	TotalRuns         int
	StatusCounts      map[RunStatus]int
	TerminationCounts map[TerminationReason]int
	Duration          AnalyticsDurationSummary
	AverageDurationMS int64
	P50DurationMS     int64
	P95DurationMS     int64
	MaxDurationMS     int64
	StepCount         int
	ToolCallCount     int
	LLMCallCount      int
	StepsUsed         int64
	ToolCallsUsed     int64
	LLMCallsUsed      int64
	TopTools          []AnalyticsToolCount
}

type AnalyticsDurationSummary struct {
	AverageMS int64
	P50MS     int64
	P95MS     int64
	MaxMS     int64
}

type AnalyticsToolCount struct {
	Name      string
	Count     int
	Total     int64
	Succeeded int64
	Failed    int64
}

type AgentAnalyticsReader interface {
	Summary(context.Context, AnalyticsQuery) (AnalyticsSummary, error)
}

type analyticsRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type analyticsRow interface {
	Scan(dest ...any) error
}

type analyticsQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (analyticsRows, error)
}

type analyticsQueryRowDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) analyticsRow
}

type sqlAnalyticsQueryDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type sqlAnalyticsQueryRowDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type SQLAgentAnalyticsReader struct {
	query    func(context.Context, string, ...any) (analyticsRows, error)
	queryRow func(context.Context, string, ...any) analyticsRow
}

type SQLAnalyticsReader = SQLAgentAnalyticsReader

func NewSQLAgentAnalyticsReader(db any) SQLAgentAnalyticsReader {
	reader := SQLAgentAnalyticsReader{}
	if queryDB, ok := db.(analyticsQueryDB); ok {
		reader.query = queryDB.QueryContext
	} else if queryDB, ok := db.(sqlAnalyticsQueryDB); ok {
		reader.query = func(ctx context.Context, query string, args ...any) (analyticsRows, error) {
			return queryDB.QueryContext(ctx, query, args...)
		}
	}
	if queryDB, ok := db.(analyticsQueryRowDB); ok {
		reader.queryRow = queryDB.QueryRowContext
	} else if queryDB, ok := db.(sqlAnalyticsQueryRowDB); ok {
		reader.queryRow = func(ctx context.Context, query string, args ...any) analyticsRow {
			return queryDB.QueryRowContext(ctx, query, args...)
		}
	}
	return reader
}

func NewSQLAnalyticsReader(db any) SQLAnalyticsReader {
	return NewSQLAgentAnalyticsReader(db)
}

func (r SQLAgentAnalyticsReader) AgentAnalytics(ctx context.Context, query AnalyticsQuery) (AnalyticsSummary, error) {
	return r.Summary(ctx, query)
}

func (r SQLAgentAnalyticsReader) Summary(ctx context.Context, query AnalyticsQuery) (AnalyticsSummary, error) {
	query = normalizeAnalyticsQuery(query)
	if err := validateAnalyticsQuery(query); err != nil {
		return AnalyticsSummary{}, err
	}
	if r.query == nil {
		return AnalyticsSummary{}, fmt.Errorf("agent analytics query database is required")
	}
	summary := AnalyticsSummary{
		GuildID:           query.GuildID,
		ActorUserID:       query.ActorUserID,
		ChannelID:         query.ChannelID,
		StatusCounts:      map[RunStatus]int{},
		TerminationCounts: map[TerminationReason]int{},
	}
	filter, args := analyticsRunFilter(query)
	if err := r.scanAggregate(ctx, filter, args, &summary); err != nil {
		return AnalyticsSummary{}, err
	}
	if err := r.scanStatusCounts(ctx, filter, args, &summary); err != nil {
		return AnalyticsSummary{}, err
	}
	topLimit := analyticsTopToolsLimitFor(query.Limit)
	if err := r.scanTerminationCounts(ctx, filter, args, topLimit, &summary); err != nil {
		return AnalyticsSummary{}, err
	}
	if err := r.scanTopTools(ctx, filter, args, topLimit, &summary); err != nil {
		return AnalyticsSummary{}, err
	}
	return summary, nil
}

func (r SQLAgentAnalyticsReader) scanAggregate(ctx context.Context, filter string, args []any, summary *AnalyticsSummary) error {
	query := fmt.Sprintf(`
with filtered_runs as (
  select r.id,
         r.created_at,
         r.completed_at,
         r.steps_used,
         r.tool_calls_used,
         r.llm_calls_used
  from agent_runs r
  where %s
),
completed_durations as (
  select greatest(0, extract(epoch from (completed_at - created_at)) * 1000)::double precision as duration_ms
  from filtered_runs
  where completed_at is not null
)
select
  count(*),
  coalesce(sum(steps_used), 0),
  coalesce(sum(tool_calls_used), 0),
  coalesce(sum(llm_calls_used), 0),
  (select avg(duration_ms) from completed_durations),
  (select percentile_cont(0.50) within group (order by duration_ms) from completed_durations),
  (select percentile_cont(0.95) within group (order by duration_ms) from completed_durations),
  (select max(duration_ms) from completed_durations)
from filtered_runs
`, filter)
	if r.queryRow != nil {
		return scanAnalyticsSummaryRow(r.queryRow(ctx, query, args...), summary)
	}
	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query agent analytics summary: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := scanAnalyticsSummaryRow(rows, summary); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate agent analytics summary: %w", err)
	}
	return nil
}

func (r SQLAgentAnalyticsReader) scanStatusCounts(ctx context.Context, filter string, args []any, summary *AnalyticsSummary) error {
	rows, err := r.query(ctx, fmt.Sprintf(`
select r.status, count(*)
from agent_runs r
where %s
group by r.status
order by count(*) desc, r.status
`, filter), args...)
	if err != nil {
		return fmt.Errorf("query agent analytics statuses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return fmt.Errorf("scan agent analytics status: %w", err)
		}
		status = strings.TrimSpace(status)
		if status != "" {
			summary.StatusCounts[RunStatus(status)] = int(count)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate agent analytics statuses: %w", err)
	}
	return nil
}

func (r SQLAgentAnalyticsReader) scanTerminationCounts(ctx context.Context, filter string, args []any, topLimit int, summary *AnalyticsSummary) error {
	queryArgs := analyticsArgsWithLimit(args, topLimit)
	rows, err := r.query(ctx, fmt.Sprintf(`
select r.termination_reason, count(*)
from agent_runs r
where %s
  and coalesce(r.termination_reason, '') <> ''
  and $6::integer >= 0
group by r.termination_reason
order by count(*) desc, r.termination_reason
`, filter), queryArgs...)
	if err != nil {
		return fmt.Errorf("query agent analytics terminations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var reason string
		var count int64
		if err := rows.Scan(&reason, &count); err != nil {
			return fmt.Errorf("scan agent analytics termination: %w", err)
		}
		reason = strings.TrimSpace(reason)
		if reason != "" {
			summary.TerminationCounts[TerminationReason(reason)] = int(count)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate agent analytics terminations: %w", err)
	}
	return nil
}

func (r SQLAgentAnalyticsReader) scanTopTools(ctx context.Context, filter string, args []any, topLimit int, summary *AnalyticsSummary) error {
	queryArgs := analyticsArgsWithLimit(args, topLimit)
	rows, err := r.query(ctx, fmt.Sprintf(`
select s.observation->>'tool' as tool_name,
       count(*),
       coalesce(sum(case when s.status = 'succeeded' then 1 else 0 end), 0),
       coalesce(sum(case when s.status = 'failed' then 1 else 0 end), 0)
from agent_run_steps s
join agent_runs r on r.id = s.run_id
where %s
  and s.kind = 'agent.tool'
  and coalesce(s.observation->>'tool', '') <> ''
group by tool_name
order by count(*) desc, tool_name
limit $6
`, filter), queryArgs...)
	if err != nil {
		return fmt.Errorf("query agent analytics top tools: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tool AnalyticsToolCount
		if err := rows.Scan(&tool.Name, &tool.Total, &tool.Succeeded, &tool.Failed); err != nil {
			return fmt.Errorf("scan agent analytics top tool: %w", err)
		}
		tool.Name = strings.TrimSpace(tool.Name)
		if tool.Name != "" {
			tool.Count = int(tool.Total)
			summary.TopTools = append(summary.TopTools, tool)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate agent analytics top tools: %w", err)
	}
	return nil
}

func normalizeAnalyticsQuery(query AnalyticsQuery) AnalyticsQuery {
	query.GuildID = strings.TrimSpace(query.GuildID)
	query.ActorUserID = strings.TrimSpace(query.ActorUserID)
	query.ChannelID = strings.TrimSpace(query.ChannelID)
	return query
}

func validateAnalyticsQuery(query AnalyticsQuery) error {
	if query.GuildID == "" {
		return fmt.Errorf("guild ID is required")
	}
	if !query.Since.IsZero() && !query.Until.IsZero() && !query.Until.After(query.Since) {
		return fmt.Errorf("until must be after since")
	}
	return nil
}

func analyticsRunFilter(query AnalyticsQuery) (string, []any) {
	return `r.guild_id = $1
  and ($2::timestamptz is null or r.created_at >= $2::timestamptz)
  and ($3::timestamptz is null or r.created_at < $3::timestamptz)
  and ($4 = '' or r.channel_id = $4)
  and ($5 = '' or r.actor_user_id = $5)`, []any{
			query.GuildID,
			analyticsTimeArg(query.Since),
			analyticsTimeArg(query.Until),
			query.ChannelID,
			query.ActorUserID,
		}
}

func scanAnalyticsSummaryRow(row analyticsRow, summary *AnalyticsSummary) error {
	var totalRuns int64
	var average sql.NullFloat64
	var p50 sql.NullFloat64
	var p95 sql.NullFloat64
	var max sql.NullFloat64
	if err := row.Scan(
		&totalRuns,
		&summary.StepsUsed,
		&summary.ToolCallsUsed,
		&summary.LLMCallsUsed,
		&average,
		&p50,
		&p95,
		&max,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("scan agent analytics summary: %w", err)
	}
	summary.TotalRuns = int(totalRuns)
	summary.StepCount = int(summary.StepsUsed)
	summary.ToolCallCount = int(summary.ToolCallsUsed)
	summary.LLMCallCount = int(summary.LLMCallsUsed)
	summary.Duration = AnalyticsDurationSummary{
		AverageMS: analyticsFloatMS(average),
		P50MS:     analyticsFloatMS(p50),
		P95MS:     analyticsFloatMS(p95),
		MaxMS:     analyticsFloatMS(max),
	}
	summary.AverageDurationMS = summary.Duration.AverageMS
	summary.P50DurationMS = summary.Duration.P50MS
	summary.P95DurationMS = summary.Duration.P95MS
	summary.MaxDurationMS = summary.Duration.MaxMS
	return nil
}

func analyticsFloatMS(value sql.NullFloat64) int64 {
	if !value.Valid {
		return 0
	}
	return int64(math.Round(value.Float64))
}

func analyticsTimeArg(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func analyticsTopToolsLimitFor(limit int) int {
	if limit <= 0 {
		return analyticsTopToolsLimit
	}
	if limit > 10 {
		return 10
	}
	return limit
}

func analyticsArgsWithLimit(args []any, limit int) []any {
	withLimit := make([]any, 0, len(args)+1)
	withLimit = append(withLimit, args...)
	withLimit = append(withLimit, limit)
	return withLimit
}
