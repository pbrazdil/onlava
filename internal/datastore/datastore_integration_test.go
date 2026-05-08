package datastore

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresVerticalSlice(t *testing.T) {
	dsn := os.Getenv("ONLAVA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ONLAVA_TEST_DATABASE_URL to run PostgreSQL data platform integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	store, err := Open(ctx, pool, Options{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	tenantKey := fmt.Sprintf("tenant_%d", time.Now().UnixNano())
	actor := Actor{ID: "tester"}

	obj, err := store.CreateObject(ctx, actor, CreateObjectRequest{
		TenantKey:    tenantKey,
		TenantName:   "Tenant",
		NameSingular: "company",
		NamePlural:   "companies",
	})
	if err != nil {
		t.Fatalf("CreateObject: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `drop table if exists `+qualifiedIdent(RecordsSchema, obj.TableName))
	}()

	if _, err := store.CreateField(ctx, actor, "company", CreateFieldRequest{TenantKey: tenantKey, Name: "name", Type: FieldText}); err != nil {
		t.Fatalf("CreateField(name): %v", err)
	}
	if _, err := store.CreateField(ctx, actor, "company", CreateFieldRequest{
		TenantKey: tenantKey,
		Name:      "stage",
		Type:      FieldSelect,
		Options: []FieldOptionRequest{
			{Value: "lead", Label: "Lead"},
			{Value: "won", Label: "Won"},
		},
	}); err != nil {
		t.Fatalf("CreateField(stage): %v", err)
	}
	if _, err := store.CreateField(ctx, actor, "company", CreateFieldRequest{TenantKey: tenantKey, Name: "arr", Type: FieldNumeric}); err != nil {
		t.Fatalf("CreateField(arr): %v", err)
	}
	if _, err := store.CreateField(ctx, actor, "company", CreateFieldRequest{TenantKey: tenantKey, Name: "full_name", Type: FieldFullName}); err != nil {
		t.Fatalf("CreateField(full_name): %v", err)
	}

	created, err := store.CreateRecord(ctx, actor, "company", CreateRecordRequest{
		TenantKey: tenantKey,
		Values: Record{
			"name":      "Acme",
			"stage":     "lead",
			"arr":       42.5,
			"full_name": Record{"first_name": "Ada", "last_name": "Lovelace"},
		},
	})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	recordID, _ := created.Record["id"].(string)
	if recordID == "" {
		t.Fatalf("created record missing id: %#v", created.Record)
	}
	if created.Event == nil || created.Event.Seq == 0 {
		t.Fatalf("created event missing seq: %#v", created.Event)
	}

	page, err := store.QueryRecords(ctx, actor, "company", QueryRecordsRequest{
		TenantKey: tenantKey,
		Query: Query{
			Select: []string{"name", "stage", "arr", "full_name"},
			Filter: &Filter{Op: "contains", Field: "name", Value: "Ac"},
			Sort:   []Sort{{Field: "arr", Desc: true}},
		},
	})
	if err != nil {
		t.Fatalf("QueryRecords: %v", err)
	}
	if len(page.Records) != 1 {
		t.Fatalf("records len = %d, want 1: %#v", len(page.Records), page.Records)
	}
	if got := page.Records[0]["stage"]; got != "lead" {
		t.Fatalf("stage = %#v, want lead", got)
	}
	fullName, ok := page.Records[0]["full_name"].(Record)
	if !ok {
		if raw, ok := page.Records[0]["full_name"].(map[string]any); ok {
			fullName = Record(raw)
		}
	}
	if fullName["first_name"] != "Ada" || fullName["last_name"] != "Lovelace" {
		t.Fatalf("full_name = %#v", page.Records[0]["full_name"])
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = store.ServeEvents(r.Context(), actor, w, r)
	}))
	defer server.Close()

	filterData, _ := json.Marshal(Filter{Op: "eq", Field: "stage", Value: "won"})
	streamURL := server.URL + "/events?tenant_key=" + url.QueryEscape(tenantKey) +
		"&object=company&query_id=won-companies&fields=name,stage&filter=" + url.QueryEscape(string(filterData))
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(streamCtx, http.MethodGet, streamURL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	if eventName, _, _, err := readSSEEvent(reader); err != nil || eventName != "ready" {
		t.Fatalf("first SSE event = %q, err %v; want ready", eventName, err)
	}

	updated, err := store.UpdateRecord(ctx, actor, "company", recordID, UpdateRecordRequest{
		TenantKey: tenantKey,
		Values:    Record{"stage": "won", "name": "Acme Labs"},
	})
	if err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	if updated.Event == nil || updated.Event.Action != "updated" {
		t.Fatalf("updated event = %#v", updated.Event)
	}

	eventName, eventData, _, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read update SSE: %v", err)
	}
	if eventName != "data" {
		t.Fatalf("update SSE event name = %q, want data", eventName)
	}
	var live Event
	if err := json.Unmarshal([]byte(eventData), &live); err != nil {
		t.Fatalf("decode live event: %v\n%s", err, eventData)
	}
	if live.Seq != updated.Event.Seq || len(live.QueryIDs) != 1 || live.QueryIDs[0] != "won-companies" {
		t.Fatalf("live event = %#v, updated event = %#v", live, updated.Event)
	}
	if _, ok := live.After["arr"]; ok {
		t.Fatalf("live selected fields included arr: %#v", live.After)
	}
	cancel()

	replayURL := server.URL + "/events?tenant_key=" + url.QueryEscape(tenantKey) +
		"&object=company&query_id=replay&after_seq=" + fmt.Sprint(updated.Event.Seq-1) +
		"&filter=" + url.QueryEscape(string(filterData))
	replayCtx, replayCancel := context.WithCancel(ctx)
	defer replayCancel()
	replayReq, _ := http.NewRequestWithContext(replayCtx, http.MethodGet, replayURL, nil)
	replayResp, err := server.Client().Do(replayReq)
	if err != nil {
		t.Fatalf("open replay SSE: %v", err)
	}
	defer replayResp.Body.Close()
	replayReader := bufio.NewReader(replayResp.Body)
	eventName, eventData, _, err = readSSEEvent(replayReader)
	if err != nil {
		t.Fatalf("read replay SSE: %v", err)
	}
	if eventName != "data" {
		t.Fatalf("replay first event = %q, want data", eventName)
	}
	var replay Event
	if err := json.Unmarshal([]byte(eventData), &replay); err != nil {
		t.Fatalf("decode replay event: %v", err)
	}
	if replay.Seq != updated.Event.Seq {
		t.Fatalf("replay seq = %d, want %d", replay.Seq, updated.Event.Seq)
	}

	deleted, err := store.DeleteRecord(ctx, actor, "company", recordID, DeleteRecordRequest{TenantKey: tenantKey})
	if err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}
	if deleted.Event == nil || deleted.Event.Action != "deleted" {
		t.Fatalf("delete event = %#v", deleted.Event)
	}
}

func readSSEEvent(r *bufio.Reader) (eventName, data, id string, err error) {
	for {
		line, readErr := r.ReadString('\n')
		if readErr != nil {
			return "", "", "", readErr
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if eventName != "" || data != "" || id != "" {
				return eventName, data, id, nil
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if data != "" {
				data += "\n"
			}
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case strings.HasPrefix(line, "id:"):
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		}
	}
}
