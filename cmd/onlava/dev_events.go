package main

import (
	"context"
	"time"

	"github.com/pbrazdil/onlava/internal/devdash"
)

type devEventSink struct {
	supervisor *devSupervisor
}

func newDevEventSink(supervisor *devSupervisor) *devEventSink {
	return &devEventSink{supervisor: supervisor}
}

func (e *devEventSink) Emit(ctx context.Context, source devdash.DevSource, level, message string, fields map[string]any) {
	if e == nil || e.supervisor == nil {
		return
	}
	s := e.supervisor
	event := assignDevEventID(devdash.NewDevEvent(s.activeAppID(), s.currentSessionID(), source, level, message, fields, time.Now().UTC()))
	e.ExportVictoriaDevEvent(event)
}

func (e *devEventSink) Output(ctx context.Context, source devdash.DevSource, plain []byte) {
	if e == nil || e.supervisor == nil || len(plain) == 0 {
		return
	}
	s := e.supervisor
	output := devdash.ProcessOutput{
		AppID:     s.activeAppID(),
		SessionID: s.currentSessionID(),
		PID:       source.PID,
		Stream:    source.Stream,
		Output:    plain,
		CreatedAt: time.Now().UTC(),
	}
	event := assignDevEventID(devdash.DevEventFromOutput(s.activeAppID(), s.currentSessionID(), source, plain, output.CreatedAt))
	e.ExportVictoriaDevEvent(event)
	if s.dashboard != nil {
		s.dashboard.notify(&devdash.Notification{
			Method: "process/output",
			Params: map[string]any{
				"appID":      s.activeAppID(),
				"pid":        source.PID,
				"stream":     source.Stream,
				"source":     source,
				"output":     output.Output,
				"created_at": output.CreatedAt.Format(time.RFC3339Nano),
			},
		})
	}
	if s.console != nil {
		s.console.Event("process.output", map[string]any{
			"pid":        source.PID,
			"stream":     source.Stream,
			"source":     source.ID,
			"output":     string(output.Output),
			"created_at": output.CreatedAt.Format(time.RFC3339Nano),
		})
	}
}

func (e *devEventSink) ExportVictoriaDevEvent(event devdash.DevEvent) {
	if e == nil || e.supervisor == nil {
		return
	}
	s := e.supervisor
	s.mu.RLock()
	victoria := s.victoria
	s.mu.RUnlock()
	if victoria == nil {
		s.mu.Lock()
		if s.victoria == nil {
			if !s.victoriaStarted {
				s.pendingDevEvents = append(s.pendingDevEvents, event)
			}
			s.mu.Unlock()
			return
		}
		victoria = s.victoria
		s.mu.Unlock()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = victoria.ExportDevEvent(ctx, event)
	}()
}
