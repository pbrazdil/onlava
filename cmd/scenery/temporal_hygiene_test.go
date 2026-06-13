package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	commonpb "go.temporal.io/api/common/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeTemporalWorkflowClient struct {
	byQuery    map[string][]*workflowpb.WorkflowExecutionInfo
	terminated []string
	closed     bool
	err        error
}

func (f *fakeTemporalWorkflowClient) ListWorkflow(_ context.Context, req *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &workflowservice.ListWorkflowExecutionsResponse{Executions: f.byQuery[req.GetQuery()]}, nil
}

func (f *fakeTemporalWorkflowClient) TerminateWorkflow(_ context.Context, workflowID string, runID string, _ string, _ ...interface{}) error {
	f.terminated = append(f.terminated, workflowID+"/"+runID)
	return nil
}

func (f *fakeTemporalWorkflowClient) Close() {
	f.closed = true
}

func TestFindTemporalStaleWorkflowsFiltersByQueueAndCutoff(t *testing.T) {
	cutoff := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	client := &fakeTemporalWorkflowClient{byQuery: map[string][]*workflowpb.WorkflowExecutionInfo{
		temporalWorkflowVisibilityQuery("scenery.demo.session.worker.go"): {
			temporalWorkflowInfo("old", "run-old", "orders.Sync/v1", "scenery.demo.session.worker.go", cutoff.Add(-time.Minute)),
			temporalWorkflowInfo("new", "run-new", "orders.Sync/v1", "scenery.demo.session.worker.go", cutoff.Add(time.Minute)),
		},
		temporalWorkflowVisibilityQuery("scenery.demo.session.render.ts"): {
			temporalWorkflowInfo("older", "run-older", "orders.Render/v1", "scenery.demo.session.render.ts", cutoff.Add(-2*time.Minute)),
		},
	}}
	got, err := findTemporalStaleWorkflows(context.Background(), client, "default", []string{"scenery.demo.session.worker.go", "scenery.demo.session.render.ts"}, cutoff)
	if err != nil {
		t.Fatalf("findTemporalStaleWorkflows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("stale workflows = %#v, want 2", got)
	}
	if got[0].WorkflowID != "older" || got[1].WorkflowID != "old" {
		t.Fatalf("stale workflows sorted by start = %#v", got)
	}
	summary := temporalStaleWorkflowSummaryFor(got)
	if summary.Count != 2 || summary.SampleWorkflowID != "older" || !summary.OldestStartTime.Equal(cutoff.Add(-2*time.Minute)) {
		t.Fatalf("summary = %#v", summary)
	}
	wantQueues := []string{"scenery.demo.session.render.ts", "scenery.demo.session.worker.go"}
	if !reflect.DeepEqual(summary.TaskQueues, wantQueues) {
		t.Fatalf("summary queues = %#v, want %#v", summary.TaskQueues, wantQueues)
	}
}

func TestFindTemporalStaleWorkflowsNamesListErrors(t *testing.T) {
	client := &fakeTemporalWorkflowClient{err: errors.New("visibility down")}
	_, err := findTemporalStaleWorkflows(context.Background(), client, "default", []string{"queue-a"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "task queue queue-a") || !strings.Contains(err.Error(), "visibility down") {
		t.Fatalf("error = %v, want named queue visibility error", err)
	}
}

func TestTemporalPruneArgsRequireStaleAndYesIsExplicit(t *testing.T) {
	if _, err := parseTemporalPruneArgs(nil); err == nil || !strings.Contains(err.Error(), "requires --stale") {
		t.Fatalf("parse without --stale error = %v", err)
	}
	opts, err := parseTemporalPruneArgs([]string{"--stale", "--json"})
	if err != nil {
		t.Fatalf("parseTemporalPruneArgs: %v", err)
	}
	if !opts.Stale || opts.Yes || !opts.JSON {
		t.Fatalf("opts = %+v", opts)
	}
	opts, err = parseTemporalPruneArgs([]string{"--stale", "--yes"})
	if err != nil {
		t.Fatalf("parseTemporalPruneArgs --yes: %v", err)
	}
	if !opts.Yes {
		t.Fatalf("--yes was not recorded: %+v", opts)
	}
}

func TestTerminateTemporalStaleWorkflowsRequiresExplicitCall(t *testing.T) {
	client := &fakeTemporalWorkflowClient{}
	candidates := []temporalStaleWorkflow{
		{WorkflowID: "wf-a", RunID: "run-a"},
		{WorkflowID: "wf-b", RunID: "run-b"},
	}
	terminated, err := terminateTemporalStaleWorkflows(context.Background(), client, candidates, "session-a")
	if err != nil {
		t.Fatalf("terminateTemporalStaleWorkflows: %v", err)
	}
	if terminated != 2 || !reflect.DeepEqual(client.terminated, []string{"wf-a/run-a", "wf-b/run-b"}) {
		t.Fatalf("terminated=%d calls=%#v", terminated, client.terminated)
	}
}

func TestTemporalActivityLogCollapserSuppressesRepeatsAfterThreshold(t *testing.T) {
	collapser := newTemporalActivityLogCollapser(3)
	line := []byte(`ERROR activity failed WorkflowType=orders.Sync/v1 error="deleted temp dir"` + "\n")
	for i := 0; i < 3; i++ {
		if got := collapser.Filter(123, "stderr", line); string(got) != string(line) {
			t.Fatalf("line %d = %q, want original", i+1, got)
		}
	}
	summary := string(collapser.Filter(123, "stderr", line))
	if !strings.Contains(summary, "suppressed repeated Temporal activity error") || !strings.Contains(summary, "orders.Sync/v1") || !strings.Contains(summary, "deleted temp dir") {
		t.Fatalf("summary = %q", summary)
	}
	if got := collapser.Filter(123, "stderr", line); got != nil {
		t.Fatalf("fifth repeated line = %q, want suppressed", got)
	}
	distinct := []byte(`ERROR activity failed WorkflowType=orders.Sync/v1 error="different"` + "\n")
	if got := collapser.Filter(123, "stderr", distinct); string(got) != string(distinct) {
		t.Fatalf("distinct error = %q, want original", got)
	}
}

func temporalWorkflowInfo(id, runID, workflowType, queue string, started time.Time) *workflowpb.WorkflowExecutionInfo {
	return &workflowpb.WorkflowExecutionInfo{
		Execution: &commonpb.WorkflowExecution{WorkflowId: id, RunId: runID},
		Type:      &commonpb.WorkflowType{Name: workflowType},
		TaskQueue: queue,
		StartTime: timestamppb.New(started),
	}
}
