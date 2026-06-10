package runtime

import (
	"time"

	"scenery.sh/internal/devreport"
)

type TemporalTraceReport struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Type         string
	Operation    string
	Name         string
	StartedAt    time.Time
	Err          error
}

func TemporalTracingEnabled() bool {
	return activeReporter() != nil
}

func ReportTemporalTrace(report TemporalTraceReport) {
	reporter := activeReporter()
	if reporter == nil {
		return
	}
	finished := time.Now().UTC()
	started := report.StartedAt
	if started.IsZero() || started.After(finished) {
		started = finished
	}
	duration := finished.Sub(started)
	summary := &devreport.TraceSummary{
		AppID:         reporter.appID,
		TraceID:       report.TraceID,
		SpanID:        report.SpanID,
		Type:          report.Type,
		IsRoot:        report.ParentSpanID == "",
		IsError:       report.Err != nil,
		StartedAt:     started,
		DurationNanos: uint64(duration),
		ServiceName:   "temporal",
		EndpointName:  optionalString(report.Name),
	}
	if report.ParentSpanID != "" {
		summary.ParentSpanID = optionalString(report.ParentSpanID)
	}
	reporter.enqueue(devreport.ReportEnvelope{
		Type:         "trace-summary",
		AppID:        reporter.appID,
		TraceSummary: summary,
	})
	reporter.enqueue(devreport.ReportEnvelope{
		Type:  "log",
		AppID: reporter.appID,
		LogEvent: &devreport.LogEvent{
			AppID:     reporter.appID,
			TraceID:   report.TraceID,
			SpanID:    report.SpanID,
			Level:     temporalReportLogLevel(report.Err),
			Message:   "temporal operation completed",
			Timestamp: finished,
			Attrs: map[string]any{
				"temporal":           true,
				"temporal_operation": report.Operation,
				"temporal_name":      report.Name,
				"temporal_error":     temporalReportErrorString(report.Err),
			},
		},
	})
}

func temporalReportLogLevel(err error) string {
	if err != nil {
		return "error"
	}
	return "info"
}

func temporalReportErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
