package runtime

import (
	"context"
	"fmt"
)

type TemporalRuntimeStarter func(context.Context, AppConfig) (func(context.Context) error, error)
type TemporalWorkerStarter func(context.Context, AppConfig) (func(context.Context) error, error)
type TemporalCronStarter func(context.Context, AppConfig, []*CronJob) (func(context.Context) error, error)

var temporalRuntimeStarter TemporalRuntimeStarter
var temporalWorkerStarter TemporalWorkerStarter
var temporalCronStarter TemporalCronStarter

func RegisterTemporalRuntimeStarter(starter TemporalRuntimeStarter) {
	temporalRuntimeStarter = starter
}

func RegisterTemporalWorkerStarter(starter TemporalWorkerStarter) {
	temporalWorkerStarter = starter
}

func RegisterTemporalCronStarter(starter TemporalCronStarter) {
	temporalCronStarter = starter
}

func StartTemporalRuntime(ctx context.Context, cfg AppConfig) (func(context.Context) error, error) {
	info := ResolveTemporalConfig(cfg.Name, cfg.Temporal)
	if !info.Enabled {
		return func(context.Context) error { return nil }, nil
	}
	if temporalRuntimeStarter == nil {
		return nil, fmt.Errorf("runtime: temporal.enabled requires github.com/pbrazdil/onlava/temporal runtime registration")
	}
	return temporalRuntimeStarter(ctx, cfg)
}

func startTemporalWorkerRuntime(ctx context.Context, cfg AppConfig) (func(context.Context) error, error) {
	if temporalWorkerStarter == nil {
		return func(context.Context) error { return nil }, nil
	}
	return temporalWorkerStarter(ctx, cfg)
}

func startTemporalCronScheduler(ctx context.Context, cfg AppConfig, jobs []*CronJob) (*cronScheduler, error) {
	if temporalCronStarter == nil {
		return nil, fmt.Errorf("runtime: temporal cron jobs require github.com/pbrazdil/onlava/temporal runtime registration")
	}
	stop, err := temporalCronStarter(ctx, cfg, jobs)
	if err != nil {
		return nil, err
	}
	done := make(chan struct{})
	close(done)
	return &cronScheduler{done: done, stop: stop}, nil
}
