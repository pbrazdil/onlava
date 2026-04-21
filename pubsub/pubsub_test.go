package pubsub

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type methodHandlerEvent struct {
	Value string `json:"value"`
}

type methodHandlerService struct {
	got chan string
}

func (s *methodHandlerService) Handle(ctx context.Context, msg *methodHandlerEvent) error {
	s.got <- msg.Value
	return nil
}

func TestStartLocalRuntimePublishesAndConsumes(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	type event struct {
		Value string `json:"value"`
	}

	got := make(chan string, 1)
	topic := NewTopic[*event]("events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	_ = NewSubscription(topic, "events-sub", SubscriptionConfig[*event]{
		Handler: func(ctx context.Context, msg *event) error {
			got <- msg.Value
			return nil
		},
		MaxConcurrency: 1,
	})

	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("stop() error = %v", err)
		}
	}()

	if _, err := topic.Publish(context.Background(), &event{Value: "ok"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case value := <-got:
		if value != "ok" {
			t.Fatalf("received %q, want ok", value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for subscription message")
	}
}

func TestLocalRuntimePubSubSnapshot(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	type event struct {
		Value string `json:"value"`
	}

	got := make(chan string, 1)
	topic := NewTopic[*event]("snapshot-events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	_ = NewSubscription(topic, "snapshot-sub", SubscriptionConfig[*event]{
		Handler: func(ctx context.Context, msg *event) error {
			got <- msg.Value
			return nil
		},
		MaxConcurrency: 2,
	})

	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("stop() error = %v", err)
		}
	}()

	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not started")
	}
	if _, err := topic.Publish(context.Background(), &event{Value: "ok"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for subscription message")
	}

	var topics []map[string]any
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		topics = rt.pubSubSnapshot()
		if len(topics) == 1 {
			subs, _ := topics[0]["subscriptions"].([]map[string]any)
			if len(subs) == 1 && subs[0]["picked_up"].(int64) >= 1 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(topics) != 1 {
		t.Fatalf("snapshot topics = %#v", topics)
	}
	if topics[0]["name"] != "snapshot-events" {
		t.Fatalf("topic name = %v", topics[0]["name"])
	}
	subs, _ := topics[0]["subscriptions"].([]map[string]any)
	if len(subs) != 1 {
		t.Fatalf("subscriptions = %#v", topics[0]["subscriptions"])
	}
	if subs[0]["max_workers"] != 2 {
		t.Fatalf("max_workers = %v", subs[0]["max_workers"])
	}
	if subs[0]["picked_up"].(int64) < 1 {
		t.Fatalf("picked_up = %v", subs[0]["picked_up"])
	}
	if subs[0]["completed"].(int64) < 1 {
		t.Fatalf("completed = %v", subs[0]["completed"])
	}
}

func TestClearLocalRuntimePurgesTopicStreams(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	type event struct {
		Value string `json:"value"`
	}

	topic := NewTopic[*event]("clear-events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("stop() error = %v", err)
		}
	}()

	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not started")
	}
	for i := 0; i < 3; i++ {
		if _, err := topic.Publish(context.Background(), &event{Value: "ok"}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	info, err := rt.js.StreamInfo("PULSE_testapp_clear_events")
	if err != nil {
		t.Fatalf("StreamInfo() error = %v", err)
	}
	if info.State.Msgs != 3 {
		t.Fatalf("stream messages before clear = %d, want 3", info.State.Msgs)
	}

	if _, err := ClearLocalRuntime(context.Background()); err != nil {
		t.Fatalf("ClearLocalRuntime() error = %v", err)
	}
	info, err = rt.js.StreamInfo("PULSE_testapp_clear_events")
	if err != nil {
		t.Fatalf("StreamInfo() after clear error = %v", err)
	}
	if info.State.Msgs != 0 {
		t.Fatalf("stream messages after clear = %d, want 0", info.State.Msgs)
	}
}

func TestStartLocalRuntimeClearsQueuesOnRestart(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	type event struct {
		Value string `json:"value"`
	}

	topic := NewTopic[*event]("restart-clear-events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("first StartLocalRuntime() error = %v", err)
	}

	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not started")
	}
	for i := 0; i < 4; i++ {
		if _, err := topic.Publish(context.Background(), &event{Value: "ok"}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	info, err := rt.js.StreamInfo("PULSE_testapp_restart_clear_events")
	if err != nil {
		t.Fatalf("StreamInfo() error = %v", err)
	}
	if info.State.Msgs != 4 {
		t.Fatalf("stream messages before restart = %d, want 4", info.State.Msgs)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := stop(stopCtx); err != nil {
		cancel()
		t.Fatalf("first stop() error = %v", err)
	}
	cancel()

	stop, err = StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("second StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("second stop() error = %v", err)
		}
	}()

	global.mu.RLock()
	rt = global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not restarted")
	}
	info, err = rt.js.StreamInfo("PULSE_testapp_restart_clear_events")
	if err != nil {
		t.Fatalf("StreamInfo() after restart error = %v", err)
	}
	if info.State.Msgs != 0 {
		t.Fatalf("stream messages after restart = %d, want 0", info.State.Msgs)
	}
}

func TestStartLocalRuntimeUpdatesConsumerWhenConcurrencyChanges(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	type event struct {
		Value string `json:"value"`
	}

	topic := NewTopic[*event]("reconfig-events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	_ = NewSubscription(topic, "reconfig-sub", SubscriptionConfig[*event]{
		Handler:        func(context.Context, *event) error { return nil },
		MaxConcurrency: 1,
	})

	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("first StartLocalRuntime() error = %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := stop(stopCtx); err != nil {
		cancel()
		t.Fatalf("first stop() error = %v", err)
	}
	cancel()

	global.mu.Lock()
	key := "reconfig-events:reconfig-sub"
	sub := global.subscriptions[key]
	if sub == nil {
		global.mu.Unlock()
		t.Fatal("subscription not registered")
	}
	sub.maxConc = 10
	cfg := sub.cfgAny.(SubscriptionConfig[*event])
	cfg.MaxConcurrency = 10
	sub.cfgAny = cfg
	global.mu.Unlock()

	stop, err = StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("second StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("second stop() error = %v", err)
		}
	}()

	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not started")
	}
	info, err := rt.js.ConsumerInfo("PULSE_testapp_reconfig_events", "PULSE_testapp_reconfig_events_reconfig_sub")
	if err != nil {
		t.Fatalf("ConsumerInfo() error = %v", err)
	}
	if info.Config.MaxAckPending != 10 {
		t.Fatalf("MaxAckPending = %d, want 10", info.Config.MaxAckPending)
	}
}

func TestStartLocalRuntimePreservesAckFloorWhenConcurrencyChanges(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	type event struct {
		Value string `json:"value"`
	}

	got := make(chan string, 3)
	topic := NewTopic[*event]("ack-floor-events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	_ = NewSubscription(topic, "ack-floor-sub", SubscriptionConfig[*event]{
		Handler: func(ctx context.Context, msg *event) error {
			got <- msg.Value
			return nil
		},
		MaxConcurrency: 1,
	})

	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("first StartLocalRuntime() error = %v", err)
	}

	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not started")
	}

	for i := 0; i < cap(got); i++ {
		if _, err := topic.Publish(context.Background(), &event{Value: "ok"}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}
	for i := 0; i < cap(got); i++ {
		select {
		case <-got:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for subscription message")
		}
	}
	waitForConsumerInfo(t, rt, "PULSE_testapp_ack_floor_events", "PULSE_testapp_ack_floor_events_ack_floor_sub", func(info *nats.ConsumerInfo) bool {
		return info.NumPending == 0 && info.NumAckPending == 0
	})

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := stop(stopCtx); err != nil {
		cancel()
		t.Fatalf("first stop() error = %v", err)
	}
	cancel()

	global.mu.Lock()
	key := "ack-floor-events:ack-floor-sub"
	sub := global.subscriptions[key]
	if sub == nil {
		global.mu.Unlock()
		t.Fatal("subscription not registered")
	}
	sub.maxConc = 5
	cfg := sub.cfgAny.(SubscriptionConfig[*event])
	cfg.MaxConcurrency = 5
	sub.cfgAny = cfg
	global.mu.Unlock()

	stop, err = StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("second StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("second stop() error = %v", err)
		}
	}()

	global.mu.RLock()
	rt = global.runtime
	global.mu.RUnlock()
	if rt == nil {
		t.Fatal("runtime not restarted")
	}
	info := waitForConsumerInfo(t, rt, "PULSE_testapp_ack_floor_events", "PULSE_testapp_ack_floor_events_ack_floor_sub", func(info *nats.ConsumerInfo) bool {
		return info.Config.MaxAckPending == 5
	})
	if info.NumPending != 0 {
		t.Fatalf("NumPending = %d, want 0; changing concurrency must not make acked messages queued again", info.NumPending)
	}
	if info.NumAckPending != 0 {
		t.Fatalf("NumAckPending = %d, want 0", info.NumAckPending)
	}
}

func TestMethodHandlerUsesServiceAccessor(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()
	t.Setenv("PULSE_DEV_CACHE_DIR", t.TempDir())

	received := make(chan string, 1)
	RegisterServiceAccessorFor[*methodHandlerService](func() (any, error) {
		return &methodHandlerService{got: received}, nil
	})
	topic := NewTopic[*methodHandlerEvent]("service-events", TopicConfig{DeliveryGuarantee: AtLeastOnce})
	_ = NewSubscription(topic, "service-events-sub", SubscriptionConfig[*methodHandlerEvent]{
		Handler: MethodHandler((*methodHandlerService).Handle),
	})

	stop, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err != nil {
		t.Fatalf("StartLocalRuntime() error = %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := stop(stopCtx); err != nil {
			t.Fatalf("stop() error = %v", err)
		}
	}()

	if _, err := topic.Publish(context.Background(), &methodHandlerEvent{Value: "handled"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case value := <-received:
		if value != "handled" {
			t.Fatalf("received %q, want handled", value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for method handler")
	}
}

func TestStartLocalRuntimeRejectsExactlyOnce(t *testing.T) {
	restore := resetRegistryForTest()
	defer restore()

	type event struct{}
	NewTopic[*event]("events", TopicConfig{DeliveryGuarantee: ExactlyOnce})
	_, err := StartLocalRuntime(context.Background(), LocalRuntimeConfig{AppID: "testapp"})
	if err == nil || !strings.Contains(err.Error(), "ExactlyOnce") {
		t.Fatalf("expected exactly-once error, got %v", err)
	}
}

func resetRegistryForTest() func() {
	prev := global
	global = &registry{
		topics:           make(map[string]*topicDecl),
		subscriptions:    make(map[string]*subscriptionDecl),
		serviceAccessors: make(map[string]func() (any, error)),
	}
	return func() {
		global = prev
	}
}

func waitForConsumerInfo(t *testing.T, rt *localRuntime, stream, durable string, ok func(*nats.ConsumerInfo) bool) *nats.ConsumerInfo {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last *nats.ConsumerInfo
	var lastErr error
	for time.Now().Before(deadline) {
		info, err := rt.js.ConsumerInfo(stream, durable)
		if err == nil && info != nil {
			last = info
			if ok(info) {
				return info
			}
		} else {
			lastErr = err
		}
		time.Sleep(25 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("ConsumerInfo(%q, %q) error = %v", stream, durable, lastErr)
	}
	t.Fatalf("ConsumerInfo(%q, %q) did not reach expected state; last = %#v", stream, durable, last)
	return nil
}
