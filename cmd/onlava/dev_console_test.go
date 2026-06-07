package main

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pbrazdil/onlava/internal/devdash"
)

func TestRenderDevConsoleShowsSourcesLogsAndExpandedJSON(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 5, 31, 12, 44, 1, 223000000, time.UTC)
	event := devdash.DevEvent{
		ID:        42,
		SessionID: "feature-x",
		Source:    devdash.DevSource{ID: "worker:typescript", Kind: "worker", Name: "typescript", PID: "12351", Status: "running"},
		Level:     "error",
		Message:   "activity failed",
		Fields:    []byte(`{"activity":"SyncUser","attempt":2}`),
		Raw:       `ERROR activity failed activity=SyncUser attempt=2`,
		Parse:     devdash.DevEventParse{Format: "level-text", OK: true},
		CreatedAt: at,
	}
	snapshot := devConsoleSnapshot{
		AppName:    "billing",
		SessionID:  "feature-x",
		Selected:   "worker:typescript",
		ErrorsOnly: true,
		Expanded:   true,
		Sources: buildDevConsoleSources([]devdash.DevSource{
			{ID: "api", Kind: "app", Name: "api", PID: "12345", Status: "running"},
			{ID: "worker:typescript", Kind: "worker", Name: "typescript", PID: "12351", Status: "running"},
		}, []devdash.DevEvent{event}),
		Events: []devdash.DevEvent{event},
	}

	out := renderDevConsole(snapshot)
	for _, want := range []string{
		"onlava up session: billing / feature-x",
		"worker:typescript",
		"activity failed",
		"activity=SyncUser",
		"event json",
		`"schema_version": "onlava.dev.event.v1"`,
		"q quit",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered console missing %q:\n%s", want, out)
		}
	}
}

func TestAttachTUIFallsBackToLogsWhenNotTerminal(t *testing.T) {
	prev := runOnlavaLogsFunc
	defer func() { runOnlavaLogsFunc = prev }()
	called := false
	runOnlavaLogsFunc = func(ctx context.Context, stdout io.Writer, args []string) error {
		called = true
		got := strings.Join(args, "\x00")
		want := strings.Join([]string{"--follow", "--session", "session-123", "--limit", "200", "--stream", "all", "--source", "api"}, "\x00")
		if got != want {
			t.Fatalf("fallback logs args = %#v, want %#v", args, strings.Split(want, "\x00"))
		}
		return nil
	}
	if err := attachCommand([]string{"--tui", "--session", "session-123", "--source", "api"}); err != nil {
		t.Fatalf("attachCommand returned error: %v", err)
	}
	if !called {
		t.Fatal("expected logs fallback")
	}
}

func TestDevConsoleRefreshUsesSelectedBackend(t *testing.T) {
	t.Parallel()

	backend := &fakeDevEventBackend{
		events: []devdash.DevEvent{
			{ID: 1, AppID: "logsapp", SessionID: "session-a", Source: devdash.DevSource{ID: "api", Kind: "app"}, Level: "info", Message: "ok", CreatedAt: time.Now().UTC()},
			{ID: 2, AppID: "logsapp", SessionID: "session-a", Source: devdash.DevSource{ID: "worker:typescript", Kind: "worker"}, Level: "error", Message: "boom", CreatedAt: time.Now().UTC()},
		},
		sources: []devdash.DevSource{
			{ID: "api", Kind: "app"},
			{ID: "worker:typescript", Kind: "worker"},
		},
	}
	state := devConsoleState{
		opts:      logsOptions{Limit: 10},
		appID:     "logsapp",
		sessionID: "session-a",
		selected:  "worker:typescript",
		errors:    true,
	}

	if err := state.refresh(context.Background(), backend); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if backend.lastQuery.SourceID != "worker:typescript" || backend.lastQuery.Level != "error" {
		t.Fatalf("backend query = %+v", backend.lastQuery)
	}
	if len(state.events) != 1 || state.events[0].Message != "boom" {
		t.Fatalf("state events = %+v", state.events)
	}
}

type fakeDevEventBackend struct {
	name      string
	events    []devdash.DevEvent
	sources   []devdash.DevSource
	lastQuery devdash.DevEventQuery
}

func (b *fakeDevEventBackend) ListDevEvents(ctx context.Context, query devdash.DevEventQuery) ([]devdash.DevEvent, error) {
	b.lastQuery = query
	out := slices.Clone(b.events)
	out = slices.DeleteFunc(out, func(event devdash.DevEvent) bool {
		if query.AppID != "" && event.AppID != query.AppID {
			return true
		}
		if query.SessionID != "" && event.SessionID != query.SessionID {
			return true
		}
		if query.SourceID != "" && event.Source.ID != query.SourceID {
			return true
		}
		if query.Level != "" && event.Level != query.Level {
			return true
		}
		return false
	})
	return out, nil
}

func (b *fakeDevEventBackend) ListDevSources(ctx context.Context, appID, sessionID string) ([]devdash.DevSource, error) {
	return slices.Clone(b.sources), nil
}

func (b *fakeDevEventBackend) BackendName() string {
	if b.name == "" {
		b.name = "fake"
	}
	return b.name
}
