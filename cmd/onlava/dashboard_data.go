package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pbrazdil/onlava/internal/datainspect"
	"github.com/pbrazdil/onlava/internal/objectstore"
)

type dataInspectRPCRequest struct {
	AppID     string `json:"app_id"`
	TenantKey string `json:"tenant_key"`
	Object    string `json:"object"`
}

type dataQueryRecordsRPCRequest struct {
	AppID     string            `json:"app_id"`
	TenantKey string            `json:"tenant_key"`
	Object    string            `json:"object"`
	Query     objectstore.Query `json:"query"`
}

type dataOutboxEventsRPCRequest struct {
	AppID     string `json:"app_id"`
	TenantKey string `json:"tenant_key"`
	Object    string `json:"object"`
	AfterSeq  int64  `json:"after_seq"`
	Limit     int    `json:"limit"`
}

type dataOutboxEventRecord struct {
	Seq           int64           `json:"seq"`
	EventID       string          `json:"event_id"`
	TenantID      string          `json:"tenant_id"`
	TenantKey     string          `json:"tenant_key"`
	ObjectID      string          `json:"object_id,omitempty"`
	Object        string          `json:"object"`
	RecordID      string          `json:"record_id,omitempty"`
	Action        string          `json:"action"`
	ActorID       string          `json:"actor_id,omitempty"`
	SchemaVersion int64           `json:"schema_version"`
	ChangedFields []string        `json:"changed_fields,omitempty"`
	Before        json.RawMessage `json:"before,omitempty"`
	After         json.RawMessage `json:"after,omitempty"`
	Diff          json.RawMessage `json:"diff,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (s *dashboardServer) inspectData(ctx context.Context, params dataInspectRPCRequest) (datainspect.Response, error) {
	pool, err := s.openDataPool(ctx, params.AppID)
	if err != nil {
		return datainspect.Response{}, err
	}
	defer pool.Close()
	return datainspect.BuildFromDB(ctx, pool, datainspect.Options{
		TenantKey:  strings.TrimSpace(params.TenantKey),
		ObjectName: strings.TrimSpace(params.Object),
	})
}

func (s *dashboardServer) queryDataRecords(ctx context.Context, params dataQueryRecordsRPCRequest) (*objectstore.RecordPage, error) {
	if strings.TrimSpace(params.TenantKey) == "" {
		return nil, fmt.Errorf("data tenant is required")
	}
	if strings.TrimSpace(params.Object) == "" {
		return nil, fmt.Errorf("data object is required")
	}
	pool, err := s.openDataPool(ctx, params.AppID)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	store, err := objectstore.Open(ctx, pool, objectstore.Options{})
	if err != nil {
		return nil, err
	}
	return store.QueryRecords(ctx, objectstore.Actor{ID: "dashboard"}, params.Object, objectstore.QueryRecordsRequest{
		TenantKey: params.TenantKey,
		Query:     params.Query,
	})
}

func (s *dashboardServer) dataOutboxEvents(ctx context.Context, params dataOutboxEventsRPCRequest) ([]dataOutboxEventRecord, error) {
	pool, err := s.openDataPool(ctx, params.AppID)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 250 {
		limit = 250
	}
	rows, err := pool.Query(ctx, `
		select e.seq, e.id::text, e.tenant_id::text, t.key,
		       coalesce(e.object_id::text, ''), e.object_name, coalesce(e.record_id::text, ''),
		       e.action, e.actor_id, e.schema_version, e.changed_fields,
		       coalesce(e.before::text, ''), coalesce(e.after::text, ''), coalesce(e.diff::text, ''),
		       e.created_at
		from onlava_data.outbox_events e
		join onlava_data.tenants t on t.id = e.tenant_id
		where ($1::text = '' or t.key = $1)
		  and ($2::text = '' or e.object_name = $2)
		  and e.seq > $3
		order by e.seq desc
		limit $4
	`, strings.TrimSpace(params.TenantKey), strings.TrimSpace(params.Object), params.AfterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("data outbox events: %w", err)
	}
	defer rows.Close()
	events := []dataOutboxEventRecord{}
	for rows.Next() {
		var event dataOutboxEventRecord
		var beforeText, afterText, diffText string
		if err := rows.Scan(
			&event.Seq,
			&event.EventID,
			&event.TenantID,
			&event.TenantKey,
			&event.ObjectID,
			&event.Object,
			&event.RecordID,
			&event.Action,
			&event.ActorID,
			&event.SchemaVersion,
			&event.ChangedFields,
			&beforeText,
			&afterText,
			&diffText,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		event.Before = rawJSONOrNil(beforeText)
		event.After = rawJSONOrNil(afterText)
		event.Diff = rawJSONOrNil(diffText)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *dashboardServer) openDataPool(ctx context.Context, appID string) (*pgxpool.Pool, error) {
	status, err := s.dashboardStatusFor(ctx, firstNonEmpty(appID, s.dashboardActiveAppID()))
	if err != nil {
		return nil, err
	}
	dsn, _, err := discoverDatabaseURL(status.AppRoot)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect data database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect data database: %w", err)
	}
	return pool, nil
}

func rawJSONOrNil(value string) json.RawMessage {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return nil
	}
	return json.RawMessage(value)
}
