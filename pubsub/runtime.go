package pubsub

import (
	"context"

	"pulse.dev/runtime"
)

func init() {
	runtime.RegisterLocalPubSubStarter(func(ctx context.Context, cfg runtime.AppConfig) (func(context.Context) error, error) {
		return StartLocalRuntime(ctx, LocalRuntimeConfig{AppID: cfg.Name})
	})
	runtime.RegisterLocalPubSubClearer(func(ctx context.Context) (any, error) {
		return ClearLocalRuntime(ctx)
	})
}
