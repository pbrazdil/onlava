package temporal

import (
	"context"
	"fmt"
	"testing"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	temporalclient "go.temporal.io/sdk/client"

	onlavaruntime "github.com/pbrazdil/onlava/runtime"
)

func TestTemporalCronScheduleOptionsApplyPolicy(t *testing.T) {
	job := &onlavaruntime.CronJob{
		ID:                   "tick",
		Every:                5 * time.Minute,
		OverlapPolicy:        "buffer_one",
		CatchupWindow:        10 * time.Minute,
		PauseOnFailure:       true,
		ActivityStartToClose: 2 * time.Minute,
		ActivityRetryPolicy: onlavaruntime.CronRetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
		Invoke: func(context.Context) error { return nil },
	}
	onlavaruntime.RegisterCronJob(job)
	options, err := temporalCronScheduleOptions(onlavaruntime.AppConfig{Name: "app"}, onlavaruntime.TemporalRuntimeInfo{TaskQueuePrefix: "app"}, "app.cron.go", job)
	if err != nil {
		t.Fatalf("temporalCronScheduleOptions returned error: %v", err)
	}
	if options.Overlap != enumspb.SCHEDULE_OVERLAP_POLICY_BUFFER_ONE {
		t.Fatalf("Overlap = %v, want BUFFER_ONE", options.Overlap)
	}
	if options.CatchupWindow != 10*time.Minute {
		t.Fatalf("CatchupWindow = %s, want 10m", options.CatchupWindow)
	}
	if !options.PauseOnFailure {
		t.Fatal("PauseOnFailure = false, want true")
	}
	action, ok := options.Action.(*temporalclient.ScheduleWorkflowAction)
	if !ok {
		t.Fatalf("Action = %T, want *client.ScheduleWorkflowAction", options.Action)
	}
	if len(action.Args) != 1 {
		t.Fatalf("Action.Args length = %d, want 1", len(action.Args))
	}
	input, ok := action.Args[0].(temporalCronInput)
	if !ok {
		t.Fatalf("Action.Args[0] = %T, want temporalCronInput", action.Args[0])
	}
	if input.ActivityStartToClose != 2*time.Minute {
		t.Fatalf("ActivityStartToClose = %s, want 2m", input.ActivityStartToClose)
	}
	if input.ActivityRetryPolicy.MaximumAttempts != 3 {
		t.Fatalf("ActivityRetryPolicy.MaximumAttempts = %d, want 3", input.ActivityRetryPolicy.MaximumAttempts)
	}
}

func TestStableTemporalCronExecutionIDIsDeterministic(t *testing.T) {
	scheduledAt := time.Date(2026, time.May, 26, 10, 30, 0, 0, time.UTC)
	got := stableTemporalCronExecutionID("orders-app", "nightly-sync", scheduledAt)
	if got != stableTemporalCronExecutionID("orders-app", "nightly-sync", scheduledAt) {
		t.Fatalf("stableTemporalCronExecutionID returned different values")
	}
	if got != "orders.app-nightly.sync-20260526T103000Z" {
		t.Fatalf("stableTemporalCronExecutionID = %q", got)
	}
}

func TestTemporalCronRetryPolicySkipsNonPositiveInitialInterval(t *testing.T) {
	if got := temporalCronRetryPolicy(onlavaruntime.CronRetryPolicy{MaximumAttempts: 3}); got != nil {
		t.Fatalf("temporalCronRetryPolicy = %#v, want nil", got)
	}
	if got := temporalCronRetryPolicy(onlavaruntime.CronRetryPolicy{InitialInterval: -time.Second, MaximumAttempts: 3}); got != nil {
		t.Fatalf("temporalCronRetryPolicy = %#v, want nil", got)
	}
	got := temporalCronRetryPolicy(onlavaruntime.CronRetryPolicy{InitialInterval: time.Second, MaximumAttempts: 3})
	if got == nil || got.InitialInterval != time.Second || got.MaximumAttempts != 3 {
		t.Fatalf("temporalCronRetryPolicy = %#v", got)
	}
}

func TestTemporalCronScheduleOptionsDefaultPolicy(t *testing.T) {
	job := &onlavaruntime.CronJob{
		ID:     "tickdefault",
		Every:  5 * time.Minute,
		Invoke: func(context.Context) error { return nil },
	}
	onlavaruntime.RegisterCronJob(job)
	options, err := temporalCronScheduleOptions(onlavaruntime.AppConfig{Name: "app"}, onlavaruntime.TemporalRuntimeInfo{TaskQueuePrefix: "app"}, "app.cron.go", job)
	if err != nil {
		t.Fatalf("temporalCronScheduleOptions returned error: %v", err)
	}
	if options.Overlap != enumspb.SCHEDULE_OVERLAP_POLICY_SKIP {
		t.Fatalf("Overlap = %v, want SKIP", options.Overlap)
	}
	if options.CatchupWindow != time.Minute {
		t.Fatalf("CatchupWindow = %s, want 1m", options.CatchupWindow)
	}
	action := options.Action.(*temporalclient.ScheduleWorkflowAction)
	input := action.Args[0].(temporalCronInput)
	if input.ActivityStartToClose != time.Hour {
		t.Fatalf("ActivityStartToClose = %s, want 1h", input.ActivityStartToClose)
	}
}

func TestTemporalCronRoleSplit(t *testing.T) {
	if shouldReconcileTemporalCronSchedules("worker") {
		t.Fatal("worker role should not reconcile schedules")
	}
	if !shouldReconcileTemporalCronSchedules("api") || !shouldReconcileTemporalCronSchedules("all") {
		t.Fatal("api/all roles should reconcile schedules")
	}
	if shouldStartTemporalCronWorker("api") {
		t.Fatal("api role should not start cron worker")
	}
	if !shouldStartTemporalCronWorker("worker") || !shouldStartTemporalCronWorker("all") {
		t.Fatal("worker/all roles should start cron worker")
	}
}

func TestTemporalAlreadyExistsErrorDetection(t *testing.T) {
	for _, err := range []error{
		serviceerror.NewAlreadyExists("schedule already exists"),
		fmt.Errorf("schedule with this ID is already registered"),
	} {
		if !isTemporalAlreadyExistsError(err) {
			t.Fatalf("isTemporalAlreadyExistsError(%v) = false, want true", err)
		}
	}
	if isTemporalAlreadyExistsError(fmt.Errorf("permission denied")) {
		t.Fatal("isTemporalAlreadyExistsError returned true for unrelated error")
	}
}
