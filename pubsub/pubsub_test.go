package pubsub

import (
	"context"
	"strings"
	"testing"
	"time"
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
