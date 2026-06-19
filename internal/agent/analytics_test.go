package agent

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSQLAnalyticsReaderSummarizesGuildRuns(t *testing.T) {
	since := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 6, 19, 0, 0, 0, 0, time.UTC)
	db := &fakeAnalyticsDB{
		row: fakeAnalyticsRow{values: []any{int64(4), int64(12), int64(5), int64(7), float64(1234.4), float64(900), float64(4200), float64(9000)}},
		rows: []analyticsRows{
			&fakeAnalyticsRows{values: [][]any{{"succeeded", int64(3)}, {"failed", int64(1)}}},
			&fakeAnalyticsRows{values: [][]any{{"completed", int64(3)}, {"planner_failed", int64(1)}}},
			&fakeAnalyticsRows{values: [][]any{{"memory.search", int64(3), int64(2), int64(1)}, {"plugins.plan", int64(2), int64(2), int64(0)}}},
		},
	}
	reader := NewSQLAnalyticsReader(db)

	got, err := reader.AgentAnalytics(context.Background(), AnalyticsQuery{
		GuildID:   " guild-id ",
		Since:     since,
		Until:     until,
		ChannelID: " channel-id ",
		Limit:     50,
	})
	if err != nil {
		t.Fatalf("AgentAnalytics returned error: %v", err)
	}
	if got.GuildID != "guild-id" || got.ChannelID != "channel-id" || got.TotalRuns != 4 || got.StepsUsed != 12 || got.ToolCallsUsed != 5 || got.LLMCallsUsed != 7 {
		t.Fatalf("summary=%+v, want normalized totals", got)
	}
	if got.Duration.AverageMS != 1234 || got.Duration.P50MS != 900 || got.Duration.P95MS != 4200 || got.Duration.MaxMS != 9000 {
		t.Fatalf("duration=%+v, want rounded latency stats", got.Duration)
	}
	if got.StatusCounts[RunStatusSucceeded] != 3 || got.StatusCounts[RunStatusFailed] != 1 {
		t.Fatalf("statuses=%+v, want succeeded/failed counts", got.StatusCounts)
	}
	if got.TerminationCounts[TerminationCompleted] != 3 || got.TerminationCounts[TerminationReason("planner_failed")] != 1 {
		t.Fatalf("terminations=%+v, want termination counts", got.TerminationCounts)
	}
	if len(got.TopTools) != 2 || got.TopTools[0].Name != "memory.search" || got.TopTools[0].Total != 3 || got.TopTools[0].Failed != 1 {
		t.Fatalf("topTools=%+v, want top tool counts", got.TopTools)
	}
	if len(db.queries) != 3 || !strings.Contains(db.rowQuery, "from agent_runs") || !strings.Contains(db.queries[2], "agent_run_steps") || strings.Contains(strings.Join(append(db.queries, db.rowQuery), "\n"), "raw_text") {
		t.Fatalf("queries=%+v row=%q, want safe analytics SQL", db.queries, db.rowQuery)
	}
	if db.args[0][0] != "guild-id" || db.args[0][1] != since || db.args[0][2] != until || db.args[0][3] != "channel-id" || db.args[3][5] != 10 {
		t.Fatalf("args=%+v, want normalized filters and clamped limit", db.args)
	}
}

func TestSQLAnalyticsReaderReturnsZeroForNoRows(t *testing.T) {
	db := &fakeAnalyticsDB{
		row: fakeAnalyticsRow{values: []any{int64(0), int64(0), int64(0), int64(0), nil, nil, nil, nil}},
		rows: []analyticsRows{
			&fakeAnalyticsRows{},
			&fakeAnalyticsRows{},
			&fakeAnalyticsRows{},
		},
	}

	got, err := NewSQLAnalyticsReader(db).AgentAnalytics(context.Background(), AnalyticsQuery{GuildID: "guild-id"})
	if err != nil {
		t.Fatalf("AgentAnalytics returned error: %v", err)
	}
	if got.TotalRuns != 0 || got.Duration.P95MS != 0 || len(got.TopTools) != 0 {
		t.Fatalf("summary=%+v, want zero summary", got)
	}
}

func TestSQLAnalyticsReaderRejectsMissingGuild(t *testing.T) {
	_, err := NewSQLAnalyticsReader(&fakeAnalyticsDB{}).AgentAnalytics(context.Background(), AnalyticsQuery{})
	if err == nil || !strings.Contains(err.Error(), "guild ID is required") {
		t.Fatalf("error=%v, want missing guild", err)
	}
}

func TestSQLAnalyticsReaderPropagatesQueryError(t *testing.T) {
	db := &fakeAnalyticsDB{row: fakeAnalyticsRow{err: errors.New("db down")}}

	_, err := NewSQLAnalyticsReader(db).AgentAnalytics(context.Background(), AnalyticsQuery{GuildID: "guild-id"})
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("error=%v, want db down", err)
	}
}

type fakeAnalyticsDB struct {
	row      analyticsRow
	rows     []analyticsRows
	queryErr error
	rowQuery string
	queries  []string
	args     [][]any
}

func (db *fakeAnalyticsDB) QueryRowContext(_ context.Context, query string, args ...any) analyticsRow {
	db.rowQuery = query
	db.args = append(db.args, args)
	if db.row != nil {
		return db.row
	}
	return fakeAnalyticsRow{}
}

func (db *fakeAnalyticsDB) QueryContext(_ context.Context, query string, args ...any) (analyticsRows, error) {
	db.queries = append(db.queries, query)
	db.args = append(db.args, args)
	if db.queryErr != nil {
		return nil, db.queryErr
	}
	if len(db.rows) == 0 {
		return &fakeAnalyticsRows{}, nil
	}
	rows := db.rows[0]
	db.rows = db.rows[1:]
	return rows, nil
}

type fakeAnalyticsRow struct {
	values []any
	err    error
}

func (r fakeAnalyticsRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanAnalyticsValues(dest, r.values)
}

type fakeAnalyticsRows struct {
	values [][]any
	index  int
	err    error
}

func (r *fakeAnalyticsRows) Close() error { return nil }
func (r *fakeAnalyticsRows) Err() error   { return r.err }
func (r *fakeAnalyticsRows) Next() bool {
	return r.index < len(r.values)
}
func (r *fakeAnalyticsRows) Scan(dest ...any) error {
	if r.index >= len(r.values) {
		return sql.ErrNoRows
	}
	values := r.values[r.index]
	r.index++
	return scanAnalyticsValues(dest, values)
}

func scanAnalyticsValues(dest []any, values []any) error {
	for i := range dest {
		if i >= len(values) {
			return sql.ErrNoRows
		}
		switch d := dest[i].(type) {
		case *string:
			if values[i] == nil {
				*d = ""
			} else {
				*d = values[i].(string)
			}
		case *int64:
			if values[i] == nil {
				*d = 0
			} else {
				*d = values[i].(int64)
			}
		case *sql.NullFloat64:
			if values[i] == nil {
				*d = sql.NullFloat64{}
			} else {
				*d = sql.NullFloat64{Float64: values[i].(float64), Valid: true}
			}
		default:
			return sql.ErrNoRows
		}
	}
	return nil
}
