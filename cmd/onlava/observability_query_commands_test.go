package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseLogsQueryArgsDefaultsAndRejectsLogQL(t *testing.T) {
	opts, err := parseLogsQueryArgs([]string{"--query", "error", "--fields", "_time,message"})
	if err != nil {
		t.Fatalf("parseLogsQueryArgs: %v", err)
	}
	if opts.Session != "current" || opts.Since != 15*time.Minute || opts.Limit != 200 || opts.Timeout != 3*time.Second {
		t.Fatalf("unexpected defaults: %+v", opts)
	}
	if strings.Join(opts.Fields, ",") != "_time,message" {
		t.Fatalf("fields = %+v", opts.Fields)
	}
	if _, err := parseLogsQueryArgs([]string{"--logql", `{app="demo"} |= "error"`}); err == nil || !strings.Contains(err.Error(), "LogsQL") {
		t.Fatalf("logql error = %v", err)
	}
	if _, err := parseLogsQueryArgs([]string{"--query", "error", "--since", "nope"}); err == nil || !strings.Contains(err.Error(), "invalid since duration") {
		t.Fatalf("since error = %v", err)
	}
}

func TestParseMetricsQueryArgsDefaults(t *testing.T) {
	opts, err := parseMetricsQueryArgs([]string{"--promql", "up", "--instant", "--limit", "7"})
	if err != nil {
		t.Fatalf("parseMetricsQueryArgs: %v", err)
	}
	if opts.Session != "current" || opts.Since != 15*time.Minute || opts.Step != 5*time.Second || opts.Timeout != 3*time.Second || !opts.Instant || opts.Limit != 7 {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if _, err := parseMetricsQueryArgs([]string{"--instant"}); err == nil || !strings.Contains(err.Error(), "missing required --promql") {
		t.Fatalf("promql error = %v", err)
	}
}

func TestParseMetricsCatalogArgs(t *testing.T) {
	opts, err := parseMetricsCatalogArgs([]string{"--match", "onlava_request_duration_seconds", "--since", "30m"}, true)
	if err != nil {
		t.Fatalf("parseMetricsCatalogArgs: %v", err)
	}
	if opts.Session != "current" || opts.Since != 30*time.Minute || opts.Match != "onlava_request_duration_seconds" || opts.Limit != 1000 {
		t.Fatalf("unexpected options: %+v", opts)
	}
	if _, err := parseMetricsCatalogArgs(nil, true); err == nil || !strings.Contains(err.Error(), "missing required --match") {
		t.Fatalf("match error = %v", err)
	}
}
