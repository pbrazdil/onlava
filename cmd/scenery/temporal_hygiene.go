package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	localagent "scenery.sh/internal/agent"
	"scenery.sh/internal/app"
	"scenery.sh/internal/devdash"
	"scenery.sh/internal/envpolicy"
	"scenery.sh/internal/workers"
	sceneryruntime "scenery.sh/runtime"
	scenerytemporal "scenery.sh/temporal"

	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
)

const temporalWorkflowListPageSize = 100

type temporalWorkflowClient interface {
	ListWorkflow(context.Context, *workflowservice.ListWorkflowExecutionsRequest) (*workflowservice.ListWorkflowExecutionsResponse, error)
	TerminateWorkflow(ctx context.Context, workflowID string, runID string, reason string, details ...interface{}) error
	Close()
}

var temporalWorkflowClientFactory = func(ctx context.Context, info sceneryruntime.TemporalRuntimeInfo) (temporalWorkflowClient, error) {
	return scenerytemporal.Dial(ctx, info)
}

type temporalStaleWorkflow struct {
	WorkflowID   string    `json:"workflow_id"`
	RunID        string    `json:"run_id,omitempty"`
	WorkflowType string    `json:"workflow_type,omitempty"`
	TaskQueue    string    `json:"task_queue"`
	StartTime    time.Time `json:"start_time"`
}

type temporalStaleWorkflowSummary struct {
	Count            int       `json:"count"`
	OldestStartTime  time.Time `json:"oldest_start_time,omitempty"`
	SampleWorkflowID string    `json:"sample_workflow_id,omitempty"`
	SampleRunID      string    `json:"sample_run_id,omitempty"`
	TaskQueues       []string  `json:"task_queues,omitempty"`
}

type temporalPruneOptions struct {
	AppRoot string
	Stale   bool
	Yes     bool
	JSON    bool
}

type temporalPruneResult struct {
	SchemaVersion string                  `json:"schema_version"`
	OK            bool                    `json:"ok"`
	DryRun        bool                    `json:"dry_run"`
	AppRoot       string                  `json:"app_root"`
	SessionID     string                  `json:"session_id"`
	Namespace     string                  `json:"namespace"`
	TaskQueues    []string                `json:"task_queues"`
	Cutoff        time.Time               `json:"cutoff"`
	Candidates    []temporalStaleWorkflow `json:"candidates"`
	Terminated    int                     `json:"terminated"`
}

func temporalCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: scenery worker temporal prune --stale [--yes] [--app-root <path>] [--json]")
	}
	switch args[0] {
	case "prune":
		return temporalPruneCommand(args[1:], os.Stdout)
	default:
		return fmt.Errorf("unknown temporal command %q", args[0])
	}
}

func temporalPruneCommand(args []string, stdout io.Writer) error {
	opts, err := parseTemporalPruneArgs(args)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := localagent.DefaultClient()
	if err != nil {
		return err
	}
	return runTemporalPrune(ctx, client, opts, stdout)
}

func parseTemporalPruneArgs(args []string) (temporalPruneOptions, error) {
	var opts temporalPruneOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--app-root":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("missing value for --app-root")
			}
			opts.AppRoot = args[i]
		case "--stale":
			opts.Stale = true
		case "--yes":
			opts.Yes = true
		case "--json":
			opts.JSON = true
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if !opts.Stale {
		return opts, fmt.Errorf("scenery worker temporal prune requires --stale; workflow termination is never implicit")
	}
	return opts, nil
}

func runTemporalPrune(ctx context.Context, agentClient *localagent.Client, opts temporalPruneOptions, stdout io.Writer) error {
	root, err := resolveAppRoot(opts.AppRoot)
	if err != nil {
		return err
	}
	root, cfg, err := app.DiscoverRoot(root)
	if err != nil {
		return err
	}
	env, err := appEnvWithDotEnv(envpolicy.Environ(), root, ".env", ".env.local")
	if err != nil {
		return err
	}
	restoreEnv := applyTemporaryEnv(envListMap(env))
	defer restoreEnv()

	if !cfg.Temporal.Enabled {
		return fmt.Errorf("scenery worker temporal prune requires temporal.enabled=true")
	}
	session, err := resolveDownSession(ctx, agentClient, downOptions{AppRoot: root})
	if err != nil {
		return err
	}
	info := temporalRuntimeInfoForSession(cfg, session)
	queues, err := temporalSessionTaskQueues(root, cfg, info)
	if err != nil {
		return err
	}
	client, err := temporalWorkflowClientFactory(ctx, info)
	if err != nil {
		return err
	}
	defer client.Close()

	cutoff := session.CreatedAt
	candidates, err := findTemporalStaleWorkflows(ctx, client, info.Namespace, queues, cutoff)
	if err != nil {
		return err
	}
	result := temporalPruneResult{
		SchemaVersion: "scenery.temporal.prune.v1",
		OK:            true,
		DryRun:        !opts.Yes,
		AppRoot:       root,
		SessionID:     session.SessionID,
		Namespace:     info.Namespace,
		TaskQueues:    queues,
		Cutoff:        cutoff,
		Candidates:    candidates,
	}
	if opts.Yes {
		terminated, err := terminateTemporalStaleWorkflows(ctx, client, candidates, session.SessionID)
		if err != nil {
			return err
		}
		result.Terminated = terminated
	}
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	if len(candidates) == 0 {
		fmt.Fprintf(stdout, "no stale Temporal workflows found for session %s\n", session.SessionID)
		return nil
	}
	if !opts.Yes {
		fmt.Fprintf(stdout, "found %d stale Temporal workflows for session %s; rerun with --yes to terminate them\n", len(candidates), session.SessionID)
		return nil
	}
	fmt.Fprintf(stdout, "terminated %d stale Temporal workflows for session %s\n", result.Terminated, session.SessionID)
	return nil
}

func terminateTemporalStaleWorkflows(ctx context.Context, client temporalWorkflowClient, candidates []temporalStaleWorkflow, sessionID string) (int, error) {
	if client == nil {
		return 0, fmt.Errorf("missing Temporal client")
	}
	terminated := 0
	for _, wf := range candidates {
		if err := client.TerminateWorkflow(ctx, wf.WorkflowID, wf.RunID, "scenery worker temporal prune --stale", "session_id", sessionID); err != nil {
			return terminated, fmt.Errorf("terminate stale Temporal workflow %s/%s: %w", wf.WorkflowID, wf.RunID, err)
		}
		terminated++
	}
	return terminated, nil
}

func temporalRuntimeInfoForSession(cfg app.Config, session localagent.Session) sceneryruntime.TemporalRuntimeInfo {
	info := sceneryruntime.ResolveTemporalConfig(cfg.Name, temporalRuntimeConfigFromApp(cfg.Temporal))
	sessionID := strings.TrimSpace(session.SessionID)
	if sessionID == "" {
		return info
	}
	baseAppID := strings.TrimSpace(session.BaseAppID)
	if baseAppID == "" {
		baseAppID = cfg.AppID()
	}
	prefix := "scenery." + baseAppID + "." + sessionID
	info.TaskQueuePrefix = prefix
	info.TaskQueueEnvSet = true
	info.SessionID = sessionID
	info.SessionIDEnvSet = true
	info.DeploymentName = sceneryruntime.TemporalDeploymentName(sceneryruntime.TemporalRuntimeInfo{DeploymentName: prefix})
	return info
}

func temporalSessionTaskQueues(appRoot string, cfg app.Config, info sceneryruntime.TemporalRuntimeInfo) ([]string, error) {
	if !cfg.Temporal.Enabled {
		return nil, nil
	}
	appModel, err := cachedInspectAppModel(appRoot, cfg.Name)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	add := func(queue string) {
		queue = strings.TrimSpace(queue)
		if queue != "" {
			seen[queue] = struct{}{}
		}
	}
	add(defaultTemporalWorkerTaskQueue(info.TaskQueuePrefix))
	for _, decl := range temporalDeclarations(appRoot, appModel, info) {
		add(decl.TaskQueue)
	}
	for _, activity := range workers.DiscoverTypeScriptActivities(appRoot).Activities {
		add(sceneryruntime.SessionScopedTemporalTaskQueue(info, activity.TaskQueue))
	}
	queues := make([]string, 0, len(seen))
	for queue := range seen {
		queues = append(queues, queue)
	}
	sort.Strings(queues)
	return queues, nil
}

func findTemporalStaleWorkflows(ctx context.Context, client temporalWorkflowClient, namespace string, queues []string, cutoff time.Time) ([]temporalStaleWorkflow, error) {
	if client == nil || cutoff.IsZero() {
		return nil, nil
	}
	var out []temporalStaleWorkflow
	for _, queue := range queues {
		token := []byte(nil)
		for {
			resp, err := client.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
				Namespace:     namespace,
				PageSize:      temporalWorkflowListPageSize,
				NextPageToken: token,
				Query:         temporalWorkflowVisibilityQuery(queue),
			})
			if err != nil {
				return nil, fmt.Errorf("list open Temporal workflows for task queue %s: %w", queue, err)
			}
			for _, wf := range resp.GetExecutions() {
				candidate, ok := staleWorkflowFromExecution(wf, queue, cutoff)
				if ok {
					out = append(out, candidate)
				}
			}
			token = resp.GetNextPageToken()
			if len(token) == 0 {
				break
			}
		}
	}
	sortTemporalStaleWorkflows(out)
	return out, nil
}

func temporalWorkflowVisibilityQuery(queue string) string {
	return fmt.Sprintf("TaskQueue = '%s' AND CloseTime is null", strings.ReplaceAll(queue, "'", "''"))
}

func staleWorkflowFromExecution(wf *workflowpb.WorkflowExecutionInfo, fallbackQueue string, cutoff time.Time) (temporalStaleWorkflow, bool) {
	if wf == nil || wf.GetExecution() == nil || wf.GetStartTime() == nil {
		return temporalStaleWorkflow{}, false
	}
	started := wf.GetStartTime().AsTime()
	if !started.Before(cutoff) {
		return temporalStaleWorkflow{}, false
	}
	queue := strings.TrimSpace(wf.GetTaskQueue())
	if queue == "" {
		queue = fallbackQueue
	}
	workflowType := ""
	if wf.GetType() != nil {
		workflowType = wf.GetType().GetName()
	}
	return temporalStaleWorkflow{
		WorkflowID:   wf.GetExecution().GetWorkflowId(),
		RunID:        wf.GetExecution().GetRunId(),
		WorkflowType: workflowType,
		TaskQueue:    queue,
		StartTime:    started,
	}, true
}

func temporalStaleWorkflowSummaryFor(workflows []temporalStaleWorkflow) temporalStaleWorkflowSummary {
	summary := temporalStaleWorkflowSummary{Count: len(workflows)}
	seenQueues := map[string]struct{}{}
	for _, wf := range workflows {
		if summary.SampleWorkflowID == "" {
			summary.SampleWorkflowID = wf.WorkflowID
			summary.SampleRunID = wf.RunID
		}
		if summary.OldestStartTime.IsZero() || wf.StartTime.Before(summary.OldestStartTime) {
			summary.OldestStartTime = wf.StartTime
		}
		if wf.TaskQueue != "" {
			seenQueues[wf.TaskQueue] = struct{}{}
		}
	}
	for queue := range seenQueues {
		summary.TaskQueues = append(summary.TaskQueues, queue)
	}
	sort.Strings(summary.TaskQueues)
	return summary
}

func sortTemporalStaleWorkflows(workflows []temporalStaleWorkflow) {
	sort.Slice(workflows, func(i, j int) bool {
		if !workflows[i].StartTime.Equal(workflows[j].StartTime) {
			return workflows[i].StartTime.Before(workflows[j].StartTime)
		}
		if workflows[i].TaskQueue != workflows[j].TaskQueue {
			return workflows[i].TaskQueue < workflows[j].TaskQueue
		}
		if workflows[i].WorkflowID != workflows[j].WorkflowID {
			return workflows[i].WorkflowID < workflows[j].WorkflowID
		}
		return workflows[i].RunID < workflows[j].RunID
	})
}

func (s *devSupervisor) emitTemporalStaleWorkflowWarning(ctx context.Context) {
	if s == nil || s.temporal == nil || !s.cfg.Temporal.Enabled {
		return
	}
	session := s.currentAgentSession()
	if session == nil || session.CreatedAt.IsZero() {
		return
	}
	info := temporalRuntimeInfoForSession(s.cfg, *session)
	info.Address = s.temporal.info.Address
	info.Namespace = s.temporal.info.Namespace
	queues, err := temporalSessionTaskQueues(s.root, s.cfg, info)
	if err != nil {
		s.emitTemporalHygieneScanError(ctx, err)
		return
	}
	scanCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	client, err := temporalWorkflowClientFactory(scanCtx, info)
	if err != nil {
		s.emitTemporalHygieneScanError(ctx, err)
		return
	}
	defer client.Close()
	workflows, err := findTemporalStaleWorkflows(scanCtx, client, info.Namespace, queues, session.CreatedAt)
	if err != nil {
		s.emitTemporalHygieneScanError(ctx, err)
		return
	}
	if len(workflows) == 0 {
		return
	}
	summary := temporalStaleWorkflowSummaryFor(workflows)
	fields := map[string]any{
		"count":              summary.Count,
		"oldest_start_time":  summary.OldestStartTime.Format(time.RFC3339Nano),
		"sample_workflow_id": summary.SampleWorkflowID,
		"task_queues":        summary.TaskQueues,
		"prune_command":      "scenery worker temporal prune --stale",
	}
	if summary.SampleRunID != "" {
		fields["sample_run_id"] = summary.SampleRunID
	}
	s.eventSink().Emit(ctx, devdash.DevSource{ID: "temporal", Kind: "substrate", Name: "temporal", Role: "workflow-server", Status: "warning", URL: s.temporal.URL()}, "warn", "Temporal has stale open workflows on this dev session's task queues", fields)
}

func (s *devSupervisor) emitTemporalHygieneScanError(ctx context.Context, err error) {
	if s == nil || err == nil {
		return
	}
	s.eventSink().Emit(ctx, devdash.DevSource{ID: "temporal", Kind: "substrate", Name: "temporal", Role: "workflow-server", Status: "warning"}, "warn", "Temporal stale workflow scan skipped", map[string]any{
		"error": err.Error(),
	})
}
