package main

import (
	"context"
	"errors"
	"time"

	"github.com/pbrazdil/onlava/internal/devdash"
)

type devEventBackend interface {
	ListDevEvents(ctx context.Context, query devdash.DevEventQuery) ([]devdash.DevEvent, error)
	ListDevSources(ctx context.Context, appID, sessionID string) ([]devdash.DevSource, error)
	BackendName() string
}

type sqliteDevEventBackend struct {
	store *devdash.Store
}

func (b sqliteDevEventBackend) ListDevEvents(ctx context.Context, query devdash.DevEventQuery) ([]devdash.DevEvent, error) {
	return b.store.ListDevEvents(ctx, query)
}

func (b sqliteDevEventBackend) ListDevSources(ctx context.Context, appID, sessionID string) ([]devdash.DevSource, error) {
	return b.store.ListDevSources(ctx, appID, sessionID)
}

func (b sqliteDevEventBackend) BackendName() string {
	return logsBackendSQLite
}

type victoriaDevEventBackend struct {
	stack *victoriaStack
}

func (b victoriaDevEventBackend) ListDevEvents(ctx context.Context, query devdash.DevEventQuery) ([]devdash.DevEvent, error) {
	return b.stack.ListDevEvents(ctx, query)
}

func (b victoriaDevEventBackend) ListDevSources(ctx context.Context, appID, sessionID string) ([]devdash.DevSource, error) {
	return b.stack.ListDevSources(ctx, appID, sessionID)
}

func (b victoriaDevEventBackend) BackendName() string {
	return logsBackendVictoria
}

func selectDevEventBackend(ctx context.Context, store *devdash.Store, opts logsOptions) (devEventBackend, error) {
	switch normalizeLogsBackend(opts.Backend) {
	case logsBackendSQLite:
		return sqliteDevEventBackend{store: store}, nil
	case logsBackendVictoria:
		victoria := resolveLogsVictoriaStack(ctx, true)
		if victoria == nil {
			return nil, errors.New("VictoriaLogs is unavailable")
		}
		return victoriaDevEventBackend{stack: victoria}, nil
	case logsBackendAuto:
		if victoria := resolveLogsVictoriaStack(ctx, false); victoria != nil {
			return victoriaDevEventBackend{stack: victoria}, nil
		}
		return sqliteDevEventBackend{store: store}, nil
	default:
		return nil, errors.New("invalid dev event backend")
	}
}

func followDevEventBackend(ctx context.Context, stdout devEventWriter, backend devEventBackend, appID, appRoot, sessionID string, opts logsOptions, items []devdash.DevEvent) error {
	lastID := int64(0)
	for _, item := range items {
		if item.ID > lastID {
			lastID = item.ID
		}
		if err := writeDevEventOutput(stdout, appID, appRoot, item, opts.JSONL); err != nil {
			return err
		}
	}
	if !opts.Follow {
		return nil
	}
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			query := logsDevEventQuery(opts, appID, sessionID)
			query.AfterID = lastID
			items, err := backend.ListDevEvents(ctx, query)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.ID > lastID {
					lastID = item.ID
				}
				if err := writeDevEventOutput(stdout, appID, appRoot, item, opts.JSONL); err != nil {
					return err
				}
			}
		}
	}
}

type devEventWriter interface {
	Write([]byte) (int, error)
}
