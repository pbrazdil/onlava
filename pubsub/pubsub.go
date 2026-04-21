package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	gruntime "runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	nserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	pulseruntime "pulse.dev/runtime"
)

const (
	NoRetries       = -2
	InfiniteRetries = -1
)

type DeliveryGuarantee int

const (
	AtLeastOnce DeliveryGuarantee = iota + 1
	ExactlyOnce
)

type TopicConfig struct {
	DeliveryGuarantee DeliveryGuarantee
	OrderingAttribute string
}

type RetryPolicy struct {
	MinBackoff time.Duration
	MaxBackoff time.Duration
	MaxRetries int
}

type TopicMeta struct {
	Name   string
	Config TopicConfig
}

type TopicPerms[T any] interface {
	Meta() TopicMeta
}

type Publisher[T any] interface {
	Publish(ctx context.Context, msg T) (id string, err error)
	Meta() TopicMeta
}

type Topic[T any] struct {
	decl *topicDecl
}

type SubscriptionConfig[T any] struct {
	Handler          func(ctx context.Context, msg T) error
	MaxConcurrency   int
	AckDeadline      time.Duration
	MessageRetention time.Duration
	RetryPolicy      *RetryPolicy
}

type SubscriptionMeta[T any] struct {
	Name   string
	Config SubscriptionConfig[T]
	Topic  TopicMeta
}

type Subscription[T any] struct {
	decl *subscriptionDecl
	meta SubscriptionMeta[T]
	cfg  SubscriptionConfig[T]
}

type LocalRuntimeConfig struct {
	AppID string
}

type dlqMessage struct {
	Topic        string          `json:"topic"`
	Subscription string          `json:"subscription"`
	Error        string          `json:"error"`
	Deliveries   int             `json:"deliveries"`
	Payload      json.RawMessage `json:"payload"`
	CreatedAt    time.Time       `json:"created_at"`
}

type topicDecl struct {
	name    string
	cfg     TopicConfig
	msgType reflect.Type
}

type subscriptionDecl struct {
	topic       *topicDecl
	name        string
	msgType     reflect.Type
	handler     func(context.Context, any) error
	cfgAny      any
	serviceName string
	maxConc     int
	ackDeadline time.Duration
	retention   time.Duration
	retry       RetryPolicy
}

type localRuntime struct {
	appID      string
	server     *nserver.Server
	conn       *nats.Conn
	js         nats.JetStreamContext
	topics     map[*topicDecl]runtimeTopic
	stats      map[string]*subscriptionStats
	published  map[string]*atomic.Int64
	dlqStream  string
	dlqSubject string
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type runtimeTopic struct {
	stream  string
	subject string
}

type subscriptionStats struct {
	topic        string
	subscription string
	stream       string
	durable      string
	workers      int
	pickedUp     atomic.Int64
	completed    atomic.Int64
	failed       atomic.Int64
	deadLettered atomic.Int64
	inFlight     atomic.Int64
	totalNanos   atomic.Int64
}

type registry struct {
	mu               sync.RWMutex
	topics           map[string]*topicDecl
	subscriptions    map[string]*subscriptionDecl
	serviceAccessors map[string]func() (any, error)
	runtime          *localRuntime
}

var global = &registry{
	topics:           make(map[string]*topicDecl),
	subscriptions:    make(map[string]*subscriptionDecl),
	serviceAccessors: make(map[string]func() (any, error)),
}

func NewTopic[T any](name string, cfg TopicConfig) *Topic[T] {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("pubsub: topic name must not be empty")
	}
	decl := &topicDecl{
		name:    name,
		cfg:     cfg,
		msgType: reflect.TypeFor[T](),
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, exists := global.topics[name]; exists {
		panic(fmt.Sprintf("pubsub: duplicate topic %q", name))
	}
	global.topics[name] = decl
	return &Topic[T]{decl: decl}
}

func (t *Topic[T]) Meta() TopicMeta {
	if t == nil || t.decl == nil {
		return TopicMeta{}
	}
	return TopicMeta{Name: t.decl.name, Config: t.decl.cfg}
}

func (t *Topic[T]) Publish(ctx context.Context, msg T) (string, error) {
	if t == nil || t.decl == nil {
		return "", errors.New("pubsub: nil topic")
	}
	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		return "", errors.New("pubsub: runtime not started")
	}
	return rt.publish(ctx, t.decl, msg)
}

func TopicRef[P TopicPerms[T], T any](topic *Topic[T]) P {
	ref, ok := any(topic).(P)
	if !ok {
		panic("pubsub: topic does not satisfy requested permissions")
	}
	return ref
}

func NewSubscription[T any](topic *Topic[T], name string, cfg SubscriptionConfig[T]) *Subscription[T] {
	if topic == nil || topic.decl == nil {
		panic("pubsub: subscription topic must not be nil")
	}
	if cfg.Handler == nil {
		panic("pubsub: subscription handler must not be nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		panic("pubsub: subscription name must not be empty")
	}
	decl := &subscriptionDecl{
		topic:       topic.decl,
		name:        name,
		msgType:     reflect.TypeFor[T](),
		cfgAny:      cfg,
		serviceName: handlerServiceName(cfg.Handler),
		maxConc:     cfg.MaxConcurrency,
		ackDeadline: normalizeAckDeadline(cfg.AckDeadline),
		retention:   normalizeRetention(cfg.MessageRetention),
		retry:       normalizeRetry(cfg.RetryPolicy),
		handler: func(ctx context.Context, msg any) error {
			typed, ok := msg.(T)
			if !ok {
				return fmt.Errorf("pubsub: unexpected message type %T for %s/%s", msg, topic.decl.name, name)
			}
			return cfg.Handler(ctx, typed)
		},
	}

	key := topic.decl.name + ":" + name
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, exists := global.subscriptions[key]; exists {
		panic(fmt.Sprintf("pubsub: duplicate subscription %q on topic %q", name, topic.decl.name))
	}
	global.subscriptions[key] = decl
	return &Subscription[T]{
		decl: decl,
		meta: SubscriptionMeta[T]{
			Name: name,
			Config: SubscriptionConfig[T]{
				Handler:          cfg.Handler,
				MaxConcurrency:   cfg.MaxConcurrency,
				AckDeadline:      decl.ackDeadline,
				MessageRetention: decl.retention,
				RetryPolicy:      cloneRetryPolicy(cfg.RetryPolicy),
			},
			Topic: topic.Meta(),
		},
		cfg: SubscriptionConfig[T]{
			Handler:          cfg.Handler,
			MaxConcurrency:   cfg.MaxConcurrency,
			AckDeadline:      decl.ackDeadline,
			MessageRetention: decl.retention,
			RetryPolicy:      cloneRetryPolicy(cfg.RetryPolicy),
		},
	}
}

func (s *Subscription[T]) Config() SubscriptionConfig[T] {
	if s == nil {
		return SubscriptionConfig[T]{}
	}
	cfg := s.cfg
	cfg.RetryPolicy = cloneRetryPolicy(cfg.RetryPolicy)
	return cfg
}

func (s *Subscription[T]) Meta() SubscriptionMeta[T] {
	if s == nil {
		return SubscriptionMeta[T]{}
	}
	meta := s.meta
	meta.Config.RetryPolicy = cloneRetryPolicy(meta.Config.RetryPolicy)
	return meta
}

func MethodHandler[T, SvcStruct any](handler func(s SvcStruct, ctx context.Context, msg T) error) func(ctx context.Context, msg T) error {
	serviceKey := serviceKeyForType(reflect.TypeFor[SvcStruct]())
	return func(ctx context.Context, msg T) error {
		global.mu.RLock()
		accessor := global.serviceAccessors[serviceKey]
		global.mu.RUnlock()
		if accessor == nil {
			return fmt.Errorf("pubsub: no service accessor registered for %s", serviceKey)
		}
		svcAny, err := accessor()
		if err != nil {
			return err
		}
		svc, ok := svcAny.(SvcStruct)
		if !ok {
			return fmt.Errorf("pubsub: service accessor returned %T, want %s", svcAny, serviceKey)
		}
		return handler(svc, ctx, msg)
	}
}

func RegisterServiceAccessorFor[T any](getter func() (any, error)) {
	if getter == nil {
		panic("pubsub: service accessor getter must not be nil")
	}
	key := serviceKeyForType(reflect.TypeFor[T]())
	global.mu.Lock()
	defer global.mu.Unlock()
	global.serviceAccessors[key] = getter
}

func StartLocalRuntime(ctx context.Context, cfg LocalRuntimeConfig) (func(context.Context) error, error) {
	topics, subs, err := snapshotDeclarations()
	if err != nil {
		return nil, err
	}
	if len(topics) == 0 && len(subs) == 0 {
		return func(context.Context) error { return nil }, nil
	}
	if strings.TrimSpace(cfg.AppID) == "" {
		return nil, errors.New("pubsub: app id must not be empty")
	}
	for _, topic := range topics {
		if topic.cfg.DeliveryGuarantee == ExactlyOnce {
			return nil, fmt.Errorf("pubsub: topic %q uses ExactlyOnce, which is not supported in Pulse v1", topic.name)
		}
	}

	global.mu.Lock()
	if global.runtime != nil {
		global.mu.Unlock()
		return nil, errors.New("pubsub: runtime already started")
	}
	global.mu.Unlock()

	storeDir, err := localStoreDir(cfg.AppID)
	if err != nil {
		return nil, err
	}
	opts := &nserver.Options{
		ServerName:      "pulse-pubsub-" + sanitizeName(cfg.AppID),
		Host:            "127.0.0.1",
		Port:            -1,
		JetStream:       true,
		StoreDir:        storeDir,
		NoSigs:          true,
		NoLog:           true,
		NoSystemAccount: true,
	}
	srv, err := nserver.NewServer(opts)
	if err != nil {
		return nil, err
	}
	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		srv.Shutdown()
		return nil, errors.New("pubsub: embedded NATS server failed to start")
	}

	conn, err := nats.Connect(srv.ClientURL(), nats.Name("pulse pubsub"))
	if err != nil {
		srv.Shutdown()
		return nil, err
	}
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		srv.Shutdown()
		return nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	rt := &localRuntime{
		appID:      cfg.AppID,
		server:     srv,
		conn:       conn,
		js:         js,
		topics:     make(map[*topicDecl]runtimeTopic, len(topics)),
		stats:      make(map[string]*subscriptionStats, len(subs)),
		published:  make(map[string]*atomic.Int64, len(topics)),
		dlqStream:  "PULSE_DLQ_" + sanitizeName(cfg.AppID),
		dlqSubject: "pulse." + sanitizeSubjectPart(cfg.AppID) + ".dlq.>",
		cancel:     cancel,
	}
	if err := rt.ensureDLQStream(); err != nil {
		cancel()
		conn.Close()
		srv.Shutdown()
		return nil, err
	}
	for _, topic := range topics {
		rTopic, err := rt.ensureTopic(topic, subs)
		if err != nil {
			cancel()
			conn.Close()
			srv.Shutdown()
			return nil, err
		}
		rt.topics[topic] = rTopic
		rt.published[topic.name] = &atomic.Int64{}
	}
	if _, err := rt.clearAll(runCtx); err != nil {
		cancel()
		conn.Close()
		srv.Shutdown()
		return nil, err
	}
	for _, sub := range subs {
		if err := rt.startSubscription(runCtx, sub); err != nil {
			cancel()
			rt.wait()
			conn.Close()
			srv.Shutdown()
			return nil, err
		}
	}

	global.mu.Lock()
	global.runtime = rt
	global.mu.Unlock()
	rt.reportPubSubSnapshot()
	rt.wg.Add(1)
	go rt.reportPubSubSnapshots(runCtx)

	return func(stopCtx context.Context) error {
		global.mu.Lock()
		if global.runtime == rt {
			global.runtime = nil
		}
		global.mu.Unlock()
		rt.cancel()
		done := make(chan struct{})
		go func() {
			rt.wait()
			close(done)
		}()
		select {
		case <-done:
		case <-stopCtx.Done():
			return stopCtx.Err()
		}
		if err := rt.conn.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
			rt.conn.Close()
			rt.server.Shutdown()
			return err
		}
		rt.conn.Close()
		rt.server.Shutdown()
		return nil
	}, nil
}

func snapshotDeclarations() ([]*topicDecl, []*subscriptionDecl, error) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	topics := make([]*topicDecl, 0, len(global.topics))
	for _, topic := range global.topics {
		topics = append(topics, topic)
	}
	subs := make([]*subscriptionDecl, 0, len(global.subscriptions))
	for _, sub := range global.subscriptions {
		subs = append(subs, sub)
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].name < topics[j].name })
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].topic.name == subs[j].topic.name {
			return subs[i].name < subs[j].name
		}
		return subs[i].topic.name < subs[j].topic.name
	})
	for _, sub := range subs {
		if _, ok := global.topics[sub.topic.name]; !ok {
			return nil, nil, fmt.Errorf("pubsub: subscription %q references unknown topic %q", sub.name, sub.topic.name)
		}
	}
	return topics, subs, nil
}

func (rt *localRuntime) ensureTopic(topic *topicDecl, subs []*subscriptionDecl) (runtimeTopic, error) {
	streamName := "PULSE_" + sanitizeName(rt.appID) + "_" + sanitizeName(topic.name)
	subject := "pulse." + sanitizeSubjectPart(rt.appID) + "." + sanitizeSubjectPart(topic.name)
	maxAge := defaultMessageRetention
	for _, sub := range subs {
		if sub.topic == topic && sub.retention > maxAge {
			maxAge = sub.retention
		}
	}
	_, err := rt.js.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{subject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    maxAge,
		Discard:   nats.DiscardOld,
	})
	if err != nil && !isAlreadyExistsError(err) {
		return runtimeTopic{}, err
	}
	if err == nil {
		return runtimeTopic{stream: streamName, subject: subject}, nil
	}
	if _, err := rt.js.UpdateStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{subject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    maxAge,
		Discard:   nats.DiscardOld,
	}); err != nil {
		return runtimeTopic{}, err
	}
	return runtimeTopic{stream: streamName, subject: subject}, nil
}

func (rt *localRuntime) ensureDLQStream() error {
	_, err := rt.js.AddStream(&nats.StreamConfig{
		Name:      rt.dlqStream,
		Subjects:  []string{rt.dlqSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
		Discard:   nats.DiscardOld,
	})
	if err != nil && !isAlreadyExistsError(err) {
		return err
	}
	if err == nil {
		return nil
	}
	_, err = rt.js.UpdateStream(&nats.StreamConfig{
		Name:      rt.dlqStream,
		Subjects:  []string{rt.dlqSubject},
		Storage:   nats.FileStorage,
		Retention: nats.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
		Discard:   nats.DiscardOld,
	})
	return err
}

func (rt *localRuntime) publish(ctx context.Context, topic *topicDecl, msg any) (string, error) {
	rTopic, ok := rt.topics[topic]
	if !ok {
		return "", fmt.Errorf("pubsub: topic %q not initialized", topic.name)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	ack, err := rt.js.PublishMsg(&nats.Msg{
		Subject: rTopic.subject,
		Data:    data,
	}, nats.Context(ctx))
	if err != nil {
		return "", err
	}
	if counter := rt.published[topic.name]; counter != nil {
		counter.Add(1)
	}
	rt.reportPubSubSnapshot()
	return fmt.Sprintf("%s:%d", ack.Stream, ack.Sequence), nil
}

func ClearLocalRuntime(ctx context.Context) ([]map[string]any, error) {
	global.mu.RLock()
	rt := global.runtime
	global.mu.RUnlock()
	if rt == nil {
		return []map[string]any{}, nil
	}
	return rt.clearAll(ctx)
}

func (rt *localRuntime) clearAll(ctx context.Context) ([]map[string]any, error) {
	streams := make(map[string]struct{}, len(rt.topics))
	for _, topic := range rt.topics {
		streams[topic.stream] = struct{}{}
	}
	if rt.dlqStream != "" {
		streams[rt.dlqStream] = struct{}{}
	}
	names := make([]string, 0, len(streams))
	for stream := range streams {
		names = append(names, stream)
	}
	sort.Strings(names)
	for _, stream := range names {
		if err := rt.js.PurgeStream(stream, nats.Context(ctx)); err != nil {
			return nil, fmt.Errorf("pubsub: clear stream %s: %w", stream, err)
		}
	}
	rt.reportPubSubSnapshot()
	return rt.pubSubSnapshot(), nil
}

func (rt *localRuntime) startSubscription(ctx context.Context, sub *subscriptionDecl) error {
	rTopic, ok := rt.topics[sub.topic]
	if !ok {
		return fmt.Errorf("pubsub: topic %q not initialized for subscription %q", sub.topic.name, sub.name)
	}
	durable := "PULSE_" + sanitizeName(rt.appID) + "_" + sanitizeName(sub.topic.name) + "_" + sanitizeName(sub.name)
	maxAckPending := 1024
	if sub.maxConc > 0 {
		maxAckPending = sub.maxConc
	}
	if err := rt.ensureConsumerConfig(rTopic.stream, durable, sub, maxAckPending); err != nil {
		return err
	}
	msgCh := make(chan *nats.Msg, max(maxAckPending, 64))
	jsSub, err := rt.js.ChanSubscribe(
		rTopic.subject,
		msgCh,
		nats.BindStream(rTopic.stream),
		nats.Durable(durable),
		nats.ManualAck(),
		nats.AckWait(sub.ackDeadline),
		nats.MaxAckPending(maxAckPending),
		nats.DeliverAll(),
		nats.MaxDeliver(-1),
	)
	if err != nil {
		return err
	}
	stats := rt.subscriptionStats(sub, rTopic.stream, durable, sub.maxConc)

	var sem chan struct{}
	if sub.maxConc > 0 {
		sem = make(chan struct{}, sub.maxConc)
	}
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		defer func() {
			_ = jsSub.Unsubscribe()
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				if msg == nil {
					continue
				}
				if sem != nil {
					select {
					case sem <- struct{}{}:
					case <-ctx.Done():
						return
					}
				}
				rt.wg.Add(1)
				go func(msg *nats.Msg) {
					defer rt.wg.Done()
					defer func() {
						if sem != nil {
							<-sem
						}
					}()
					rt.handleMessage(ctx, sub, msg, stats)
				}(msg)
			}
		}
	}()
	return nil
}

func (rt *localRuntime) ensureConsumerConfig(stream, durable string, sub *subscriptionDecl, maxAckPending int) error {
	info, err := rt.js.ConsumerInfo(stream, durable)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return err
	}
	if info == nil {
		return nil
	}
	cfg := info.Config
	if cfg.MaxAckPending == maxAckPending && cfg.AckWait == sub.ackDeadline && cfg.MaxDeliver == -1 {
		return nil
	}
	cfg.MaxAckPending = maxAckPending
	cfg.AckWait = sub.ackDeadline
	cfg.MaxDeliver = -1
	if _, err := rt.js.UpdateConsumer(stream, &cfg); err != nil {
		return fmt.Errorf("pubsub: update consumer %s/%s after config change: %w", stream, durable, err)
	}
	return nil
}

func (rt *localRuntime) handleMessage(parent context.Context, sub *subscriptionDecl, msg *nats.Msg, stats *subscriptionStats) {
	meta, _ := msg.Metadata()
	handlerCtx, cancel := context.WithTimeout(parent, sub.ackDeadline)
	defer cancel()
	started := time.Now()
	if stats != nil {
		stats.pickedUp.Add(1)
		stats.inFlight.Add(1)
		defer func() {
			stats.inFlight.Add(-1)
			stats.totalNanos.Add(time.Since(started).Nanoseconds())
			rt.reportPubSubSnapshot()
		}()
	}
	var payload any
	if err := decodeMessage(msg.Data, sub.msgType, &payload); err != nil {
		_ = rt.publishDLQ(sub, msg.Data, metaDeliveries(meta), err)
		_ = msg.Ack()
		if stats != nil {
			stats.failed.Add(1)
			stats.deadLettered.Add(1)
		}
		return
	}
	err := invokeHandler(handlerCtx, sub.handler, payload)
	if err == nil {
		_ = msg.Ack()
		if stats != nil {
			stats.completed.Add(1)
		}
		return
	}
	if stats != nil {
		stats.failed.Add(1)
	}
	deliveries := metaDeliveries(meta)
	if shouldDeadLetter(sub.retry.MaxRetries, deliveries) {
		_ = rt.publishDLQ(sub, msg.Data, deliveries, err)
		_ = msg.Ack()
		if stats != nil {
			stats.deadLettered.Add(1)
		}
		return
	}
	delay := retryDelay(sub.retry, deliveries)
	if delay > 0 {
		_ = msg.NakWithDelay(delay)
		return
	}
	_ = msg.Nak()
}

func (rt *localRuntime) publishDLQ(sub *subscriptionDecl, payload []byte, deliveries int, err error) error {
	body, marshalErr := json.Marshal(dlqMessage{
		Topic:        sub.topic.name,
		Subscription: sub.name,
		Error:        err.Error(),
		Deliveries:   deliveries,
		Payload:      append([]byte(nil), payload...),
		CreatedAt:    time.Now().UTC(),
	})
	if marshalErr != nil {
		return marshalErr
	}
	subject := "pulse." + sanitizeSubjectPart(rt.appID) + ".dlq." + sanitizeSubjectPart(sub.topic.name) + "." + sanitizeSubjectPart(sub.name)
	_, pubErr := rt.js.Publish(subject, body)
	return pubErr
}

func (rt *localRuntime) wait() {
	rt.wg.Wait()
}

func (rt *localRuntime) subscriptionStats(sub *subscriptionDecl, stream, durable string, workers int) *subscriptionStats {
	key := sub.topic.name + ":" + sub.name
	if existing := rt.stats[key]; existing != nil {
		return existing
	}
	stats := &subscriptionStats{
		topic:        sub.topic.name,
		subscription: sub.name,
		stream:       stream,
		durable:      durable,
		workers:      workers,
	}
	rt.stats[key] = stats
	return stats
}

func (rt *localRuntime) reportPubSubSnapshots(ctx context.Context) {
	defer rt.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rt.reportPubSubSnapshot()
		}
	}
}

func (rt *localRuntime) reportPubSubSnapshot() {
	if rt == nil {
		return
	}
	topics := rt.pubSubSnapshot()
	if len(topics) == 0 {
		return
	}
	pulseruntime.ReportPubSubSnapshot(topics)
}

func (rt *localRuntime) pubSubSnapshot() []map[string]any {
	topics, subs, err := snapshotDeclarations()
	if err != nil {
		return nil
	}
	items := make([]map[string]any, 0, len(topics))
	for _, topic := range topics {
		rTopic := rt.topics[topic]
		pending := int64(0)
		var subItems []map[string]any
		for _, sub := range subs {
			if sub.topic != topic {
				continue
			}
			stats := rt.stats[sub.topic.name+":"+sub.name]
			subItem := map[string]any{
				"name":                sub.name,
				"service_name":        sub.serviceName,
				"max_workers":         sub.maxConc,
				"max_ack_pending":     1024,
				"ack_deadline_ms":     sub.ackDeadline.Milliseconds(),
				"message_retention_s": int64(sub.retention.Seconds()),
				"pending":             int64(0),
				"ack_pending":         int64(0),
				"redelivered":         int64(0),
				"picked_up":           int64(0),
				"completed":           int64(0),
				"failed":              int64(0),
				"dead_lettered":       int64(0),
				"in_flight":           int64(0),
				"avg_duration_ms":     float64(0),
			}
			if stats != nil {
				pickedUp := stats.pickedUp.Load()
				totalNanos := stats.totalNanos.Load()
				avg := float64(0)
				if pickedUp > 0 {
					avg = float64(totalNanos) / float64(pickedUp) / float64(time.Millisecond)
				}
				subItem["max_workers"] = stats.workers
				if stats.workers > 0 {
					subItem["max_ack_pending"] = stats.workers
				}
				subItem["picked_up"] = pickedUp
				subItem["completed"] = stats.completed.Load()
				subItem["failed"] = stats.failed.Load()
				subItem["dead_lettered"] = stats.deadLettered.Load()
				subItem["in_flight"] = stats.inFlight.Load()
				subItem["avg_duration_ms"] = avg
				if info, err := rt.js.ConsumerInfo(stats.stream, stats.durable); err == nil && info != nil {
					subItem["pending"] = int64(info.NumPending)
					subItem["ack_pending"] = int64(info.NumAckPending)
					subItem["redelivered"] = int64(info.NumRedelivered)
					pending += int64(info.NumPending)
				}
			}
			subItems = append(subItems, subItem)
		}
		published := int64(0)
		if counter := rt.published[topic.name]; counter != nil {
			published = counter.Load()
		}
		items = append(items, map[string]any{
			"name":          topic.name,
			"stream":        rTopic.stream,
			"subject":       rTopic.subject,
			"delivery":      deliveryGuaranteeName(topic.cfg.DeliveryGuarantee),
			"ordering_key":  topic.cfg.OrderingAttribute,
			"published":     published,
			"pending":       pending,
			"subscriptions": subItems,
		})
	}
	return items
}

func deliveryGuaranteeName(value DeliveryGuarantee) string {
	switch value {
	case AtLeastOnce:
		return "at_least_once"
	case ExactlyOnce:
		return "exactly_once"
	default:
		return "unknown"
	}
}

func decodeMessage(data []byte, targetType reflect.Type, out *any) error {
	if targetType == nil {
		return errors.New("pubsub: missing message type")
	}
	value := reflect.New(targetType)
	if targetType.Kind() == reflect.Pointer {
		value = reflect.New(targetType.Elem())
	}
	if err := json.Unmarshal(data, value.Interface()); err != nil {
		return err
	}
	if targetType.Kind() == reflect.Pointer {
		*out = value.Interface()
		return nil
	}
	*out = value.Elem().Interface()
	return nil
}

func invokeHandler(ctx context.Context, handler func(context.Context, any) error, payload any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pubsub: handler panic: %v", r)
		}
	}()
	return handler(ctx, payload)
}

func cloneRetryPolicy(policy *RetryPolicy) *RetryPolicy {
	if policy == nil {
		return nil
	}
	cp := *policy
	return &cp
}

func normalizeAckDeadline(v time.Duration) time.Duration {
	if v <= 0 {
		return 30 * time.Second
	}
	if v < time.Second {
		return time.Second
	}
	return v
}

func normalizeRetention(v time.Duration) time.Duration {
	if v <= 0 {
		return defaultMessageRetention
	}
	return v
}

func normalizeRetry(policy *RetryPolicy) RetryPolicy {
	if policy == nil {
		return RetryPolicy{
			MinBackoff: 10 * time.Second,
			MaxBackoff: 10 * time.Minute,
			MaxRetries: 100,
		}
	}
	min := policy.MinBackoff
	if min <= 0 {
		min = 10 * time.Second
	}
	max := policy.MaxBackoff
	if max <= 0 {
		max = 10 * time.Minute
	}
	if max < min {
		max = min
	}
	return RetryPolicy{
		MinBackoff: min,
		MaxBackoff: max,
		MaxRetries: policy.MaxRetries,
	}
}

func metaDeliveries(meta *nats.MsgMetadata) int {
	if meta == nil {
		return 1
	}
	return int(meta.NumDelivered)
}

func shouldDeadLetter(maxRetries, deliveries int) bool {
	switch maxRetries {
	case InfiniteRetries:
		return false
	case NoRetries:
		return true
	case 0:
		return deliveries > 100
	default:
		return deliveries > maxRetries+1
	}
}

func retryDelay(policy RetryPolicy, deliveries int) time.Duration {
	if deliveries <= 0 {
		return policy.MinBackoff
	}
	delay := policy.MinBackoff
	for i := 1; i < deliveries; i++ {
		delay *= 2
		if delay >= policy.MaxBackoff {
			return policy.MaxBackoff
		}
	}
	if delay > policy.MaxBackoff {
		return policy.MaxBackoff
	}
	return delay
}

func localStoreDir(appID string) (string, error) {
	root := strings.TrimSpace(os.Getenv("PULSE_DEV_CACHE_DIR"))
	if root == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		root = filepathJoin(dir, "pulse")
	}
	path := filepathJoin(root, "pubsub", sanitizeName(appID))
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "default"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('_')
			lastDash = true
		}
	}
	value := strings.Trim(b.String(), "_")
	if value == "" {
		return "default"
	}
	return value
}

func sanitizeSubjectPart(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "default"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func serviceKeyForType(t reflect.Type) string {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return ""
	}
	if pkgPath := t.PkgPath(); pkgPath != "" && t.Name() != "" {
		return pkgPath + "." + t.Name()
	}
	return t.String()
}

func handlerServiceName(handler any) string {
	name := gruntime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	if name == "" {
		return ""
	}
	name = strings.TrimSuffix(name, "-fm")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, "."); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimPrefix(name, "*")
	return name
}

func isAlreadyExistsError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "already in use")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "not found")
}

func filepathJoin(parts ...string) string {
	return strings.Join(parts, string(os.PathSeparator))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

const defaultMessageRetention = 7 * 24 * time.Hour
