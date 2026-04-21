package devdash

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestStoreStoredRequestsCRUD(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	ctx := context.Background()
	created, err := store.CreateStoredRequest(ctx, StoredRequest{
		AppID:  "app-test",
		Title:  "Initial",
		RPC:    "Config",
		Svc:    "tenants",
		Shared: true,
		Data: StoredRequestData{
			Method:     "GET",
			PathParams: json.RawMessage(`{"tenantID":"123"}`),
			Payload:    json.RawMessage(`{"ok":true}`),
		},
	})
	if err != nil {
		t.Fatalf("create stored request: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected stored request id")
	}

	list, err := store.ListStoredRequests(ctx, "app-test")
	if err != nil {
		t.Fatalf("list stored requests: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 stored request, got %d", len(list))
	}
	if got := list[0].Data.PathParams; string(got) != `{"tenantID":"123"}` {
		t.Fatalf("unexpected path params: %s", got)
	}

	updated, err := store.UpdateStoredRequest(ctx, StoredRequest{
		ID:     created.ID,
		AppID:  "app-test",
		Title:  "Updated",
		RPC:    "Config",
		Svc:    "tenants",
		Shared: false,
		Data: StoredRequestData{
			Method:     "POST",
			PathParams: json.RawMessage(`{"tenantID":"456"}`),
			Payload:    json.RawMessage(`{"ok":false}`),
		},
	})
	if err != nil {
		t.Fatalf("update stored request: %v", err)
	}
	if updated.Title != "Updated" {
		t.Fatalf("unexpected updated title: %q", updated.Title)
	}

	list, err = store.ListStoredRequests(ctx, "app-test")
	if err != nil {
		t.Fatalf("list after update: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 stored request after update, got %d", len(list))
	}
	if list[0].Shared {
		t.Fatal("expected shared=false after update")
	}
	if got := list[0].Data.Payload; string(got) != `{"ok":false}` {
		t.Fatalf("unexpected payload after update: %s", got)
	}

	if err := store.DeleteStoredRequest(ctx, "app-test", created.ID); err != nil {
		t.Fatalf("delete stored request: %v", err)
	}
	list, err = store.ListStoredRequests(ctx, "app-test")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 stored requests after delete, got %d", len(list))
	}
}

func TestStorePubSubSnapshot(t *testing.T) {
	t.Parallel()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	ctx := context.Background()
	empty, err := store.GetPubSubSnapshot(ctx, "app-test")
	if err != nil {
		t.Fatalf("get empty pubsub snapshot: %v", err)
	}
	if got := string(empty.Topics); got != `[]` {
		t.Fatalf("empty topics = %s, want []", got)
	}

	if err := store.UpsertPubSubSnapshot(ctx, PubSubSnapshot{
		AppID:  "app-test",
		Topics: json.RawMessage(`[{"name":"events","pending":2}]`),
	}); err != nil {
		t.Fatalf("upsert pubsub snapshot: %v", err)
	}
	got, err := store.GetPubSubSnapshot(ctx, "app-test")
	if err != nil {
		t.Fatalf("get pubsub snapshot: %v", err)
	}
	if string(got.Topics) != `[{"name":"events","pending":2}]` {
		t.Fatalf("topics = %s", got.Topics)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected updated timestamp")
	}
	history, err := store.ListPubSubSnapshots(ctx, "app-test", time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("list pubsub history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if string(history[0].Topics) != `[{"name":"events","pending":2}]` {
		t.Fatalf("history topics = %s", history[0].Topics)
	}
}
