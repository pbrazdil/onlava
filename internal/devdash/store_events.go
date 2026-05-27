package devdash

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) AppendTraceEvent(ctx context.Context, event *TraceEvent) error {
	if event == nil {
		return errors.New("trace event is nil")
	}
	body, err := marshalTraceEvent(event)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into trace_events (app_id, session_id, trace_id, span_id, event_id, event_time, event_json)
		values (?, ?, ?, ?, ?, ?, ?)
	`, event.AppID, event.SessionID, event.TraceID, event.SpanID, event.EventID, event.EventTime.UTC().Format(time.RFC3339Nano), string(body))
	return err
}

func (s *Store) GetTraceEvents(ctx context.Context, appID, traceID, spanID string) ([]*TraceEvent, error) {
	return s.GetTraceEventsForSession(ctx, appID, "", traceID, spanID)
}

func (s *Store) GetTraceEventsForSession(ctx context.Context, appID, sessionID, traceID, spanID string) ([]*TraceEvent, error) {
	stmt := `
		select event_json
		from trace_events
		where app_id = ? and trace_id = ? and span_id = ?
	`
	args := []any{appID, traceID, spanID}
	if sessionID != "" {
		stmt += ` and session_id = ?`
		args = append(args, sessionID)
	}
	stmt += ` order by event_id asc`
	rows, err := s.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*TraceEvent
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var event TraceEvent
		if err := unmarshalTraceEvent([]byte(data), &event); err != nil {
			return nil, err
		}
		event.AppID = appID
		if event.SessionID == "" {
			event.SessionID = sessionID
		}
		list = append(list, &event)
	}
	return list, rows.Err()
}

func (s *Store) WriteLogEvent(ctx context.Context, event *LogEvent) error {
	if event == nil {
		return errors.New("log event is nil")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	attrs, err := json.Marshal(event.Attrs)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		insert into log_events (app_id, session_id, trace_id, span_id, level, message, attrs_json, created_at)
		values (?, ?, ?, ?, ?, ?, ?, ?)
	`, event.AppID, event.SessionID, event.TraceID, event.SpanID, event.Level, event.Message, string(attrs), event.Timestamp.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ClearTraces(ctx context.Context, appID string) error {
	return s.ClearTracesForSession(ctx, appID, "")
}

func (s *Store) ClearTracesForSession(ctx context.Context, appID, sessionID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range []string{
		"trace_events",
		"trace_summaries",
		"log_events",
	} {
		stmt := `delete from ` + table + ` where app_id = ?`
		args := []any{appID}
		if sessionID != "" {
			stmt += ` and session_id = ?`
			args = append(args, sessionID)
		}
		if _, err := tx.ExecContext(ctx, stmt, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetOnboarding(ctx context.Context) (OnboardingState, error) {
	rows, err := s.db.QueryContext(ctx, `select name, set_at from onboarding`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	state := make(OnboardingState)
	for rows.Next() {
		var name, setAt string
		if err := rows.Scan(&name, &setAt); err != nil {
			return nil, err
		}
		ts, err := time.Parse(time.RFC3339Nano, setAt)
		if err != nil {
			return nil, fmt.Errorf("parse onboarding time for %s: %w", name, err)
		}
		state[name] = ts
	}
	return state, rows.Err()
}

func (s *Store) SetOnboarding(ctx context.Context, props []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, prop := range props {
		if prop == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			insert into onboarding (name, set_at) values (?, ?)
			on conflict(name) do update set set_at = excluded.set_at
		`, prop, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListStoredRequests(ctx context.Context, appID string) ([]StoredRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, title, rpc_name, svc_name, shared, data_json
		from stored_requests
		where app_id = ?
		order by updated_at desc, id asc
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []StoredRequest
	for rows.Next() {
		var req StoredRequest
		var shared int
		var data string
		if err := rows.Scan(&req.ID, &req.Title, &req.RPC, &req.Svc, &shared, &data); err != nil {
			return nil, err
		}
		req.AppID = appID
		req.Shared = shared == 1
		if err := json.Unmarshal([]byte(data), &req.Data); err != nil {
			return nil, err
		}
		list = append(list, sanitizeStoredRequest(req))
	}
	return list, rows.Err()
}

func (s *Store) CreateStoredRequest(ctx context.Context, req StoredRequest) (StoredRequest, error) {
	if req.AppID == "" {
		return StoredRequest{}, errors.New("stored request app id is required")
	}
	req = sanitizeStoredRequest(req)
	if req.ID == "" {
		id, err := newStoredRequestID()
		if err != nil {
			return StoredRequest{}, err
		}
		req.ID = id
	}
	data, err := json.Marshal(req.Data)
	if err != nil {
		return StoredRequest{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		insert into stored_requests (app_id, id, title, rpc_name, svc_name, shared, data_json, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.AppID, req.ID, req.Title, req.RPC, req.Svc, boolToInt(req.Shared), string(data), now, now)
	if err != nil {
		return StoredRequest{}, err
	}
	return req, nil
}

func (s *Store) UpdateStoredRequest(ctx context.Context, req StoredRequest) (StoredRequest, error) {
	if req.AppID == "" {
		return StoredRequest{}, errors.New("stored request app id is required")
	}
	if req.ID == "" {
		return StoredRequest{}, errors.New("stored request id is required")
	}
	req = sanitizeStoredRequest(req)
	data, err := json.Marshal(req.Data)
	if err != nil {
		return StoredRequest{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		update stored_requests
		set title = ?, rpc_name = ?, svc_name = ?, shared = ?, data_json = ?, updated_at = ?
		where app_id = ? and id = ?
	`, req.Title, req.RPC, req.Svc, boolToInt(req.Shared), string(data), time.Now().UTC().Format(time.RFC3339Nano), req.AppID, req.ID)
	if err != nil {
		return StoredRequest{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return StoredRequest{}, err
	}
	if rows == 0 {
		return StoredRequest{}, sql.ErrNoRows
	}
	return req, nil
}

func (s *Store) DeleteStoredRequest(ctx context.Context, appID, id string) error {
	if appID == "" {
		return errors.New("stored request app id is required")
	}
	if id == "" {
		return errors.New("stored request id is required")
	}
	result, err := s.db.ExecContext(ctx, `
		delete from stored_requests
		where app_id = ? and id = ?
	`, appID, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseOptionalTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func marshalTraceEvent(event *TraceEvent) ([]byte, error) {
	payload := map[string]any{
		"trace_id":      event.TraceID,
		"span_id":       event.SpanID,
		"session_id":    event.SessionID,
		"app_root_hash": event.AppRootHash,
		"branch":        event.Branch,
		"worktree":      event.Worktree,
		"event_id":      event.EventID,
		"event_time":    event.EventTime.UTC().Format(time.RFC3339Nano),
		"event":         event.Event,
	}
	return json.Marshal(payload)
}

func unmarshalTraceEvent(data []byte, dst *TraceEvent) error {
	var raw struct {
		TraceID     string         `json:"trace_id"`
		SpanID      string         `json:"span_id"`
		SessionID   string         `json:"session_id"`
		AppRootHash string         `json:"app_root_hash"`
		Branch      string         `json:"branch"`
		Worktree    string         `json:"worktree"`
		EventID     uint64         `json:"event_id"`
		EventTime   string         `json:"event_time"`
		Event       map[string]any `json:"event"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	dst.TraceID = raw.TraceID
	dst.SpanID = raw.SpanID
	dst.SessionID = raw.SessionID
	dst.AppRootHash = raw.AppRootHash
	dst.Branch = raw.Branch
	dst.Worktree = raw.Worktree
	dst.EventID = raw.EventID
	dst.Event = raw.Event
	if raw.EventTime != "" {
		dst.EventTime, _ = time.Parse(time.RFC3339Nano, raw.EventTime)
	}
	return nil
}

func sanitizeStoredRequest(req StoredRequest) StoredRequest {
	req.Data.PathParams = normalizeStoredRequestJSON(req.Data.PathParams)
	req.Data.Payload = normalizeStoredRequestJSON(req.Data.Payload)
	return req
}

func normalizeStoredRequestJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return append(json.RawMessage(nil), value...)
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return append(json.RawMessage(nil), value...)
	}
	return normalized
}

func newStoredRequestID() (string, error) {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("sr_%x", data[:]), nil
}
