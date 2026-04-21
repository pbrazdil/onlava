package runtime

import "context"

type LocalPubSubStarter func(context.Context, AppConfig) (func(context.Context) error, error)
type LocalPubSubClearer func(context.Context) (any, error)

var localPubSubStarter LocalPubSubStarter
var localPubSubClearer LocalPubSubClearer

func RegisterLocalPubSubStarter(starter LocalPubSubStarter) {
	localPubSubStarter = starter
}

func RegisterLocalPubSubClearer(clearer LocalPubSubClearer) {
	localPubSubClearer = clearer
}

func startLocalPubSubRuntime(ctx context.Context, cfg AppConfig) (func(context.Context) error, error) {
	if localPubSubStarter == nil {
		return func(context.Context) error { return nil }, nil
	}
	return localPubSubStarter(ctx, cfg)
}

func clearLocalPubSubRuntime(ctx context.Context) (any, error) {
	if localPubSubClearer == nil {
		return []any{}, nil
	}
	return localPubSubClearer(ctx)
}
