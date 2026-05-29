package temporal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	temporalclient "go.temporal.io/sdk/client"
	sdktemporal "go.temporal.io/sdk/temporal"
	temporalworker "go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	onlavaruntime "github.com/pbrazdil/onlava/runtime"
)

const (
	temporalCronWorkflowName = "onlava.cron.Invoke/v1"
	testTriggerCronSchedules = "ONLAVA_TEST_TRIGGER_CRON_SCHEDULES"
)

type temporalCronInput struct {
	AppID                string
	JobID                string
	ActivityName         string
	TaskQueue            string
	ActivityStartToClose time.Duration
	ActivityRetryPolicy  onlavaruntime.CronRetryPolicy
}

type temporalCronActivityInput struct {
	AppID       string
	JobID       string
	ScheduledAt time.Time
	ExecutionID string
}

func startCronRuntime(parent context.Context, cfg onlavaruntime.AppConfig, jobs []*onlavaruntime.CronJob) (func(context.Context) error, error) {
	client, info, ok := ActiveClient()
	if !ok || client == nil {
		return nil, fmt.Errorf("runtime: cron jobs require temporal.enabled and an active Temporal client")
	}
	taskQueue := temporalCronTaskQueue(info)
	if shouldReconcileTemporalCronSchedules(cfg.Role) {
		for _, job := range jobs {
			if err := reconcileTemporalCronSchedule(parent, client, cfg, info, taskQueue, job); err != nil {
				return nil, err
			}
			slog.Info("onlava cron schedule reconciled", "id", job.ID, "title", job.Title, "schedule", cronScheduleSummary(job), "backend", "temporal", "task_queue", taskQueue)
		}
	}
	var worker temporalworker.Worker
	if shouldStartTemporalCronWorker(cfg.Role) {
		worker = temporalworker.New(client, taskQueue, TemporalWorkerOptions(info, "cron", taskQueue))
		worker.RegisterWorkflowWithOptions(temporalCronWorkflow, workflow.RegisterOptions{Name: temporalCronWorkflowName})
		for _, job := range jobs {
			job := job
			worker.RegisterActivityWithOptions(
				func(ctx context.Context, in temporalCronActivityInput) error {
					return runTemporalCronActivity(ctx, job, in)
				},
				activity.RegisterOptions{Name: temporalCronActivityName(job)},
			)
		}
		if err := worker.Start(); err != nil {
			return nil, fmt.Errorf("runtime: start temporal cron worker on %s: %w", taskQueue, err)
		}
		if onlavaruntime.ShouldAutoPromoteTemporalWorkerDeployment(info) {
			if err := EnsureWorkerDeploymentCurrentVersion(parent, client, info); err != nil {
				worker.Stop()
				return nil, err
			}
		}
	}
	return func(context.Context) error {
		if worker != nil {
			worker.Stop()
		}
		return nil
	}, nil
}

func reconcileTemporalCronSchedule(ctx context.Context, client temporalclient.Client, cfg onlavaruntime.AppConfig, info onlavaruntime.TemporalRuntimeInfo, taskQueue string, job *onlavaruntime.CronJob) error {
	options, err := temporalCronScheduleOptions(cfg, info, taskQueue, job)
	if err != nil {
		return err
	}
	schedules := client.ScheduleClient()
	if _, err := schedules.Create(ctx, options); err == nil {
		if temporalCronTestTriggerEnabled() {
			return schedules.GetHandle(ctx, options.ID).Trigger(ctx, temporalclient.ScheduleTriggerOptions{})
		}
		return nil
	} else if !isTemporalAlreadyExistsError(err) {
		return fmt.Errorf("runtime: create temporal cron schedule %s: %w", options.ID, err)
	}
	handle := schedules.GetHandle(ctx, options.ID)
	if err := handle.Update(ctx, temporalclient.ScheduleUpdateOptions{
		DoUpdate: func(temporalclient.ScheduleUpdateInput) (*temporalclient.ScheduleUpdate, error) {
			return &temporalclient.ScheduleUpdate{
				Schedule: &temporalclient.Schedule{
					Action: options.Action,
					Spec:   &options.Spec,
					Policy: &temporalclient.SchedulePolicies{
						CatchupWindow:  options.CatchupWindow,
						Overlap:        options.Overlap,
						PauseOnFailure: options.PauseOnFailure,
					},
					State: &temporalclient.ScheduleState{
						Note: options.Note,
					},
				},
			}, nil
		},
	}); err != nil {
		return err
	}
	if temporalCronTestTriggerEnabled() {
		return handle.Trigger(ctx, temporalclient.ScheduleTriggerOptions{})
	}
	return nil
}

func temporalCronTestTriggerEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(testTriggerCronSchedules))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func temporalCronScheduleOptions(cfg onlavaruntime.AppConfig, info onlavaruntime.TemporalRuntimeInfo, taskQueue string, job *onlavaruntime.CronJob) (temporalclient.ScheduleOptions, error) {
	spec, err := temporalCronScheduleSpec(job)
	if err != nil {
		return temporalclient.ScheduleOptions{}, err
	}
	activityName := temporalCronActivityName(job)
	overlap, err := temporalCronOverlapPolicy(job.OverlapPolicy)
	if err != nil {
		return temporalclient.ScheduleOptions{}, err
	}
	catchupWindow := job.CatchupWindow
	if catchupWindow == 0 {
		catchupWindow = time.Minute
	}
	activityStartToClose := job.ActivityStartToClose
	if activityStartToClose == 0 {
		activityStartToClose = time.Hour
	}
	return temporalclient.ScheduleOptions{
		ID:   temporalCronScheduleID(info, job),
		Spec: spec,
		Action: &temporalclient.ScheduleWorkflowAction{
			ID:        temporalCronWorkflowID(info, job),
			Workflow:  temporalCronWorkflowName,
			TaskQueue: taskQueue,
			Args: []interface{}{temporalCronInput{
				AppID:                cfg.Name,
				JobID:                job.ID,
				ActivityName:         activityName,
				TaskQueue:            taskQueue,
				ActivityStartToClose: activityStartToClose,
				ActivityRetryPolicy:  job.ActivityRetryPolicy,
			}},
			Memo: map[string]interface{}{
				"onlava_app": cfg.Name,
				"onlava_job": job.ID,
			},
		},
		Overlap:        overlap,
		CatchupWindow:  catchupWindow,
		PauseOnFailure: job.PauseOnFailure,
		Note:           "managed by onlava",
		Memo: map[string]interface{}{
			"onlava_app": cfg.Name,
			"onlava_job": job.ID,
		},
	}, nil
}

func temporalCronScheduleSpec(job *onlavaruntime.CronJob) (temporalclient.ScheduleSpec, error) {
	spec, err := onlavaruntime.TemporalCronScheduleSpecForJob(job)
	if err != nil {
		return temporalclient.ScheduleSpec{}, err
	}
	out := temporalclient.ScheduleSpec{}
	for _, interval := range spec.Intervals {
		out.Intervals = append(out.Intervals, temporalclient.ScheduleIntervalSpec{Every: interval})
	}
	for _, calendar := range spec.Calendars {
		out.Calendars = append(out.Calendars, temporalclient.ScheduleCalendarSpec{
			Second:     temporalScheduleRanges(calendar.Second),
			Minute:     temporalScheduleRanges(calendar.Minute),
			Hour:       temporalScheduleRanges(calendar.Hour),
			DayOfMonth: temporalScheduleRanges(calendar.DayOfMonth),
			Month:      temporalScheduleRanges(calendar.Month),
			DayOfWeek:  temporalScheduleRanges(calendar.DayOfWeek),
			Comment:    calendar.Comment,
		})
	}
	return out, nil
}

func temporalScheduleRanges(in []onlavaruntime.TemporalCronScheduleRange) []temporalclient.ScheduleRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]temporalclient.ScheduleRange, 0, len(in))
	for _, item := range in {
		out = append(out, temporalclient.ScheduleRange{
			Start: item.Start,
			End:   item.End,
			Step:  item.Step,
		})
	}
	return out
}

func temporalCronWorkflow(ctx workflow.Context, in temporalCronInput) error {
	scheduledAt := workflow.Now(ctx).UTC()
	startToClose := in.ActivityStartToClose
	if startToClose == 0 {
		startToClose = time.Hour
	}
	ao := workflow.ActivityOptions{
		TaskQueue:           in.TaskQueue,
		StartToCloseTimeout: startToClose,
		RetryPolicy:         temporalCronRetryPolicy(in.ActivityRetryPolicy),
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)
	return workflow.ExecuteActivity(actCtx, in.ActivityName, temporalCronActivityInput{
		AppID:       in.AppID,
		JobID:       in.JobID,
		ScheduledAt: scheduledAt,
		ExecutionID: stableTemporalCronExecutionID(in.AppID, in.JobID, scheduledAt),
	}).Get(actCtx, nil)
}

func runTemporalCronActivity(ctx context.Context, job *onlavaruntime.CronJob, in temporalCronActivityInput) error {
	if job == nil {
		return fmt.Errorf("runtime: missing cron job declaration")
	}
	scheduledAt := in.ScheduledAt.UTC()
	if scheduledAt.IsZero() {
		scheduledAt = time.Now().UTC()
	}
	return onlavaruntime.InvokeCronJob(ctx, job, scheduledAt, in.ExecutionID)
}

func temporalCronOverlapPolicy(policy string) (enumspb.ScheduleOverlapPolicy, error) {
	switch strings.TrimSpace(strings.ToLower(policy)) {
	case "", "skip":
		return enumspb.SCHEDULE_OVERLAP_POLICY_SKIP, nil
	case "buffer_one":
		return enumspb.SCHEDULE_OVERLAP_POLICY_BUFFER_ONE, nil
	case "buffer_all":
		return enumspb.SCHEDULE_OVERLAP_POLICY_BUFFER_ALL, nil
	case "cancel_other":
		return enumspb.SCHEDULE_OVERLAP_POLICY_CANCEL_OTHER, nil
	case "terminate_other":
		return enumspb.SCHEDULE_OVERLAP_POLICY_TERMINATE_OTHER, nil
	case "allow_all":
		return enumspb.SCHEDULE_OVERLAP_POLICY_ALLOW_ALL, nil
	default:
		return enumspb.SCHEDULE_OVERLAP_POLICY_UNSPECIFIED, fmt.Errorf("runtime: cron overlap policy %q is invalid", policy)
	}
}

func temporalCronRetryPolicy(policy onlavaruntime.CronRetryPolicy) *sdktemporal.RetryPolicy {
	if cronRetryPolicyIsZero(policy) {
		return nil
	}
	if policy.InitialInterval <= 0 {
		return nil
	}
	return &sdktemporal.RetryPolicy{
		InitialInterval:        policy.InitialInterval,
		BackoffCoefficient:     policy.BackoffCoefficient,
		MaximumInterval:        policy.MaximumInterval,
		MaximumAttempts:        policy.MaximumAttempts,
		NonRetryableErrorTypes: policy.NonRetryableErrorTypes,
	}
}

func cronRetryPolicyIsZero(policy onlavaruntime.CronRetryPolicy) bool {
	return policy.InitialInterval == 0 &&
		policy.BackoffCoefficient == 0 &&
		policy.MaximumInterval == 0 &&
		policy.MaximumAttempts == 0 &&
		len(policy.NonRetryableErrorTypes) == 0
}

func shouldReconcileTemporalCronSchedules(role string) bool {
	return strings.TrimSpace(strings.ToLower(role)) != "worker"
}

func shouldStartTemporalCronWorker(role string) bool {
	return strings.TrimSpace(strings.ToLower(role)) != "api"
}

func temporalCronTaskQueue(info onlavaruntime.TemporalRuntimeInfo) string {
	prefix := strings.TrimSpace(info.TaskQueuePrefix)
	if prefix == "" {
		prefix = "onlava"
	}
	return strings.TrimSuffix(prefix, ".") + ".cron.go"
}

func temporalCronScheduleID(info onlavaruntime.TemporalRuntimeInfo, job *onlavaruntime.CronJob) string {
	return onlavaruntime.TemporalDeploymentName(info) + ".cron." + onlavaruntime.SanitizeTemporalName(job.ID)
}

func temporalCronWorkflowID(info onlavaruntime.TemporalRuntimeInfo, job *onlavaruntime.CronJob) string {
	return temporalCronScheduleID(info, job)
}

func temporalCronActivityName(job *onlavaruntime.CronJob) string {
	if job == nil {
		return "onlava.cron.unknown/v1"
	}
	return "onlava.cron." + onlavaruntime.SanitizeTemporalName(job.ID) + "/v1"
}

func stableTemporalCronExecutionID(appID, jobID string, scheduledAt time.Time) string {
	appID = onlavaruntime.SanitizeTemporalName(appID)
	if appID == "" {
		appID = "app"
	}
	return fmt.Sprintf("%s-%s-%s", appID, onlavaruntime.SanitizeTemporalName(jobID), scheduledAt.UTC().Format("20060102T150405Z"))
}

func isTemporalAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	var alreadyExists *serviceerror.AlreadyExists
	if errors.As(err, &alreadyExists) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "already exist") || strings.Contains(message, "already registered")
}

func cronScheduleSummary(job *onlavaruntime.CronJob) string {
	if job.Every > 0 {
		return "every " + job.Every.String()
	}
	return job.Schedule
}
