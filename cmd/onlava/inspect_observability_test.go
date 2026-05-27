package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/pbrazdil/onlava/internal/devdash"
)

func TestRunOnlavaInspectTracesWithFilters(t *testing.T) {
	root := t.TempDir()
	cacheRoot := t.TempDir()
	t.Setenv("ONLAVA_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, ".onlava.json", `{"name":"obsapp","id":"obs-id"}`)

	store := openTestObservabilityStore(t, cacheRoot, root)
	endpoint := "SyncGet"
	now := time.Now().UTC()
	if err := store.AppendTraceSummary(context.Background(), &devdash.TraceSummary{
		AppID:         "obs-id",
		SessionID:     "session-a",
		AppRootHash:   "root123",
		Branch:        "feature/a",
		Worktree:      "onlv-a",
		TraceID:       "trace-fast",
		SpanID:        "span-fast",
		Type:          "RPC",
		IsRoot:        true,
		StartedAt:     now.Add(-2 * time.Minute),
		DurationNanos: uint64(10 * time.Millisecond),
		ServiceName:   "sync",
		EndpointName:  &endpoint,
	}); err != nil {
		t.Fatalf("append fast trace: %v", err)
	}
	if err := store.AppendTraceSummary(context.Background(), &devdash.TraceSummary{
		AppID:         "obs-id",
		SessionID:     "session-a",
		AppRootHash:   "root123",
		Branch:        "feature/a",
		Worktree:      "onlv-a",
		TraceID:       "trace-slow",
		SpanID:        "span-slow",
		Type:          "RPC",
		IsRoot:        true,
		StartedAt:     now.Add(-time.Minute),
		DurationNanos: uint64(2500 * time.Millisecond),
		ServiceName:   "sync",
		EndpointName:  &endpoint,
	}); err != nil {
		t.Fatalf("append slow trace: %v", err)
	}

	var out bytes.Buffer
	if err := runOnlavaInspect([]string{
		"traces",
		"--app-root", root,
		"--json",
		"--session", "session-a",
		"--endpoint", "SyncGet",
		"--min-duration-ms", "2000",
		"--slowest",
	}, &out); err != nil {
		t.Fatalf("inspect traces: %v\n%s", err, out.String())
	}

	var payload inspectTracesResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != inspectTracesSchema {
		t.Fatalf("schema = %q", payload.SchemaVersion)
	}
	if payload.Query.SessionID != "session-a" {
		t.Fatalf("query session = %q", payload.Query.SessionID)
	}
	if len(payload.Traces) != 1 {
		t.Fatalf("traces = %d, want 1: %+v", len(payload.Traces), payload.Traces)
	}
	if payload.Traces[0].TraceID != "trace-slow" || payload.Traces[0].DurationMS != 2500 || payload.Traces[0].SessionID != "session-a" || payload.Traces[0].AppRootHash != "root123" || payload.Traces[0].Branch != "feature/a" || payload.Traces[0].Worktree != "onlv-a" {
		t.Fatalf("trace = %+v", payload.Traces[0])
	}
}

func TestRunOnlavaInspectMetricsAggregatesTracesAndLogs(t *testing.T) {
	root := t.TempDir()
	cacheRoot := t.TempDir()
	t.Setenv("ONLAVA_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, ".onlava.json", `{"name":"obsapp","id":"obs-id"}`)

	store := openTestObservabilityStore(t, cacheRoot, root)
	endpoint := "Config"
	now := time.Now().UTC()
	for _, item := range []struct {
		id       string
		duration time.Duration
		err      bool
	}{
		{id: "trace-1", duration: 100 * time.Millisecond},
		{id: "trace-2", duration: 200 * time.Millisecond, err: true},
		{id: "trace-3", duration: 300 * time.Millisecond},
	} {
		if err := store.AppendTraceSummary(context.Background(), &devdash.TraceSummary{
			AppID:         "obs-id",
			SessionID:     "session-a",
			TraceID:       item.id,
			SpanID:        item.id + "-span",
			Type:          "RPC",
			IsRoot:        true,
			IsError:       item.err,
			StartedAt:     now.Add(-time.Minute),
			DurationNanos: uint64(item.duration),
			ServiceName:   "tenants",
			EndpointName:  &endpoint,
		}); err != nil {
			t.Fatalf("append trace %s: %v", item.id, err)
		}
	}
	if err := store.AppendTraceEvent(context.Background(), &devdash.TraceEvent{
		AppID:     "obs-id",
		SessionID: "session-a",
		TraceID:   "trace-1",
		SpanID:    "trace-1-span",
		EventID:   1,
		EventTime: now,
		Event:     map[string]any{"type": "request.start"},
	}); err != nil {
		t.Fatalf("append trace event: %v", err)
	}
	if err := store.WriteLogEvent(context.Background(), &devdash.LogEvent{
		AppID:     "obs-id",
		SessionID: "session-a",
		Level:     "ERR",
		Message:   "failed",
		Timestamp: now,
	}); err != nil {
		t.Fatalf("write log event: %v", err)
	}
	if err := store.AppendTraceSummary(context.Background(), &devdash.TraceSummary{
		AppID:         "obs-id",
		SessionID:     "session-b",
		TraceID:       "trace-other-session",
		SpanID:        "trace-other-session-span",
		Type:          "RPC",
		IsRoot:        true,
		StartedAt:     now.Add(-time.Minute),
		DurationNanos: uint64(5 * time.Second),
		ServiceName:   "tenants",
		EndpointName:  &endpoint,
	}); err != nil {
		t.Fatalf("append other-session trace: %v", err)
	}
	if err := store.WriteLogEvent(context.Background(), &devdash.LogEvent{
		AppID:     "obs-id",
		SessionID: "session-b",
		Level:     "INFO",
		Message:   "other session",
		Timestamp: now,
	}); err != nil {
		t.Fatalf("write other-session log event: %v", err)
	}

	var out bytes.Buffer
	if err := runOnlavaInspect([]string{
		"metrics",
		"--app-root", root,
		"--json",
		"--session", "session-a",
		"--service", "tenants",
		"--since", "1h",
	}, &out); err != nil {
		t.Fatalf("inspect metrics: %v\n%s", err, out.String())
	}

	var payload inspectMetricsResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != inspectMetricsSchema {
		t.Fatalf("schema = %q", payload.SchemaVersion)
	}
	if payload.Query.SessionID != "session-a" {
		t.Fatalf("query session = %q", payload.Query.SessionID)
	}
	if payload.Summary.TraceCount != 3 || payload.Summary.ErrorCount != 1 || payload.Summary.EventCount != 1 || payload.Summary.LogCount != 1 {
		t.Fatalf("summary = %+v", payload.Summary)
	}
	if len(payload.Services) != 1 || payload.Services[0].Service != "tenants" || payload.Services[0].Count != 3 {
		t.Fatalf("services = %+v", payload.Services)
	}
	if len(payload.Endpoints) != 1 || payload.Endpoints[0].Endpoint != "Config" {
		t.Fatalf("endpoints = %+v", payload.Endpoints)
	}
	if len(payload.Logs) != 1 || payload.Logs[0].Level != "ERR" {
		t.Fatalf("logs = %+v", payload.Logs)
	}
}

func TestRunOnlavaInspectUsesSessionAppRecordWhenLatestAppRootDiffers(t *testing.T) {
	root := t.TempDir()
	otherRoot := t.TempDir()
	cacheRoot := t.TempDir()
	t.Setenv("ONLAVA_DEV_CACHE_DIR", cacheRoot)
	writeTestAppFile(t, root, ".onlava.json", `{"name":"obsapp","id":"obs-id"}`)

	store, err := devdash.OpenStore(cacheRoot)
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	ctx := context.Background()
	for _, rec := range []devdash.AppRecord{
		{ID: "obs-id", SessionID: "session-a", Name: "obsapp", Root: root, Running: true, UpdatedAt: time.Now().UTC().Add(-time.Minute)},
		{ID: "obs-id", SessionID: "session-b", Name: "obsapp", Root: otherRoot, Running: true, UpdatedAt: time.Now().UTC()},
	} {
		if err := store.UpsertApp(ctx, rec); err != nil {
			t.Fatalf("UpsertApp() error = %v", err)
		}
	}
	if err := store.AppendTraceSummary(ctx, &devdash.TraceSummary{
		AppID:         "obs-id",
		SessionID:     "session-a",
		TraceID:       "trace-a",
		SpanID:        "span-a",
		Type:          "REQUEST",
		IsRoot:        true,
		StartedAt:     time.Now().UTC(),
		DurationNanos: uint64(time.Millisecond),
		ServiceName:   "sync",
	}); err != nil {
		t.Fatalf("AppendTraceSummary() error = %v", err)
	}

	var out bytes.Buffer
	if err := runOnlavaInspect([]string{
		"traces",
		"--app-root", root,
		"--json",
		"--session", "session-a",
	}, &out); err != nil {
		t.Fatalf("inspect traces: %v\n%s", err, out.String())
	}

	var payload inspectTracesResponse
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out.String())
	}
	if len(payload.Warnings) != 0 {
		t.Fatalf("warnings = %+v", payload.Warnings)
	}
	if len(payload.Traces) != 1 || payload.Traces[0].TraceID != "trace-a" {
		t.Fatalf("traces = %+v", payload.Traces)
	}
}

func openTestObservabilityStore(t *testing.T, cacheRoot, appRoot string) *devdash.Store {
	t.Helper()
	store, err := devdash.OpenStore(cacheRoot)
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	if err := store.UpsertApp(context.Background(), devdash.AppRecord{
		ID:   "obs-id",
		Name: "obsapp",
		Root: appRoot,
	}); err != nil {
		t.Fatalf("UpsertApp() error = %v", err)
	}
	return store
}
