package devdash

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

type TraceQuery struct {
	AppID            string
	TraceID          string
	ServiceName      string
	EndpointName     string
	Status           string
	Since            time.Time
	MinDurationNanos uint64
	Limit            int
}

type LogLevelCount struct {
	Level string `json:"level"`
	Count int64  `json:"count"`
}

func (s *Store) QueryTraceSummaries(ctx context.Context, query TraceQuery) ([]*TraceSummary, error) {
	if query.Limit <= 0 {
		query.Limit = 100
	}
	stmt := `
		select summary_json
		from trace_summaries
		where app_id = ? and is_root = 1
	`
	args := []any{query.AppID}
	if query.TraceID != "" {
		stmt += ` and trace_id = ?`
		args = append(args, query.TraceID)
	}
	if query.ServiceName != "" {
		stmt += ` and service_name = ?`
		args = append(args, query.ServiceName)
	}
	if query.EndpointName != "" {
		stmt += ` and endpoint_name = ?`
		args = append(args, query.EndpointName)
	}
	switch query.Status {
	case "ok":
		stmt += ` and is_error = 0`
	case "error":
		stmt += ` and is_error = 1`
	}
	if !query.Since.IsZero() {
		stmt += ` and started_at >= ?`
		args = append(args, query.Since.UTC().Format(time.RFC3339Nano))
	}
	if query.MinDurationNanos > 0 {
		stmt += ` and duration_nanos >= ?`
		args = append(args, query.MinDurationNanos)
	}
	stmt += ` order by started_at desc limit ?`
	args = append(args, query.Limit)

	rows, err := s.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTraceSummaries(rows, query.AppID)
}

func (s *Store) QueryTraceMetrics(ctx context.Context, query TraceQuery) ([]*TraceSummary, error) {
	if query.Limit <= 0 {
		query.Limit = 10000
	}
	return s.QueryTraceSummaries(ctx, query)
}

func (s *Store) CountTraceEvents(ctx context.Context, appID string, since time.Time) (int64, error) {
	stmt := `select count(*) from trace_events where app_id = ?`
	args := []any{appID}
	if !since.IsZero() {
		stmt += ` and event_time >= ?`
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}
	var count int64
	if err := s.db.QueryRowContext(ctx, stmt, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) CountLogsByLevel(ctx context.Context, appID string, since time.Time) ([]LogLevelCount, error) {
	stmt := `
		select level, count(*)
		from log_events
		where app_id = ?
	`
	args := []any{appID}
	if !since.IsZero() {
		stmt += ` and created_at >= ?`
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}
	stmt += ` group by level order by count(*) desc, level asc`

	rows, err := s.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []LogLevelCount
	for rows.Next() {
		var item LogLevelCount
		if err := rows.Scan(&item.Level, &item.Count); err != nil {
			return nil, err
		}
		counts = append(counts, item)
	}
	return counts, rows.Err()
}

func scanTraceSummaries(rows *sql.Rows, appID string) ([]*TraceSummary, error) {
	var list []*TraceSummary
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var summary TraceSummary
		if err := json.Unmarshal([]byte(data), &summary); err != nil {
			return nil, err
		}
		summary.AppID = appID
		list = append(list, &summary)
	}
	return list, rows.Err()
}

func SortTraceSummariesByDuration(items []*TraceSummary) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].DurationNanos == items[j].DurationNanos {
			return items[i].StartedAt.After(items[j].StartedAt)
		}
		return items[i].DurationNanos > items[j].DurationNanos
	})
}
