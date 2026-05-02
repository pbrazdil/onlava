package cron

import (
	"context"
	"fmt"
	"reflect"
	"time"

	onlavaruntime "onlava.com/runtime"
)

type JobConfig struct {
	Title    string
	Endpoint any
	Every    Duration
	Schedule string
}

type Job struct {
	ID       string
	Title    string
	Every    Duration
	Schedule string
	Endpoint any
}

type Duration int64

const (
	Minute Duration = 60
	Hour   Duration = 60 * Minute
)

func NewJob(id string, cfg JobConfig) *Job {
	job := &Job{
		ID:       id,
		Title:    cfg.Title,
		Every:    cfg.Every,
		Schedule: cfg.Schedule,
		Endpoint: cfg.Endpoint,
	}
	if job.Title == "" {
		job.Title = job.ID
	}

	invoke, err := makeInvoker(cfg.Endpoint)
	if err != nil {
		panic(err)
	}
	onlavaruntime.RegisterCronJob(&onlavaruntime.CronJob{
		ID:       job.ID,
		Title:    job.Title,
		Every:    time.Duration(job.Every) * time.Second,
		Schedule: job.Schedule,
		Invoke:   invoke,
	})
	return job
}

func makeInvoker(endpoint any) (func(context.Context) error, error) {
	value := reflect.ValueOf(endpoint)
	if !value.IsValid() || value.Kind() != reflect.Func {
		return nil, fmt.Errorf("cron: endpoint must be a function")
	}

	typ := value.Type()
	if typ.NumIn() != 1 || !isContextType(typ.In(0)) {
		return nil, fmt.Errorf("cron: endpoint must have signature func(context.Context) error or func(context.Context) (T, error)")
	}
	if typ.NumOut() != 1 && typ.NumOut() != 2 {
		return nil, fmt.Errorf("cron: endpoint must have signature func(context.Context) error or func(context.Context) (T, error)")
	}
	if !isErrorType(typ.Out(typ.NumOut() - 1)) {
		return nil, fmt.Errorf("cron: endpoint must have signature func(context.Context) error or func(context.Context) (T, error)")
	}

	return func(ctx context.Context) error {
		results := value.Call([]reflect.Value{reflect.ValueOf(ctx)})
		errValue := results[len(results)-1]
		if errValue.IsNil() {
			return nil
		}
		return errValue.Interface().(error)
	}, nil
}

func isContextType(t reflect.Type) bool {
	return t == reflect.TypeFor[context.Context]()
}

func isErrorType(t reflect.Type) bool {
	return t.Implements(reflect.TypeFor[error]())
}
