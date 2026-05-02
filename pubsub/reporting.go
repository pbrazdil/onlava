package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	gruntime "runtime"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	onlavaruntime "github.com/pbrazdil/onlava/runtime"
)

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
	onlavaruntime.ReportPubSubSnapshot(topics)
}

func (rt *localRuntime) reportQueuedMessages(topic *topicDecl, messageID string, payload []byte, insertedAt time.Time) {
	if rt == nil || topic == nil {
		return
	}
	_, subs, err := snapshotDeclarations()
	if err != nil {
		return
	}
	for _, sub := range subs {
		if sub.topic != topic {
			continue
		}
		rt.reportMessage(sub, messageID, payload, "queued", "", 0, time.Time{}, insertedAt, time.Time{}, 0, nil, 0)
	}
}

func (rt *localRuntime) reportMessage(
	sub *subscriptionDecl,
	messageID string,
	payload []byte,
	status string,
	traceID string,
	attempt int,
	pickedUpAt time.Time,
	insertedAt time.Time,
	finishedAt time.Time,
	duration time.Duration,
	err error,
	deliveries int,
) {
	if rt == nil || sub == nil || messageID == "" {
		return
	}
	var result any
	if err == nil {
		result = map[string]any{"status": status}
	} else {
		result = map[string]any{"status": status, "error": err.Error()}
	}
	onlavaruntime.ReportPubSubMessage(map[string]any{
		"message_id":        messageID,
		"topic_name":        sub.topic.name,
		"subscription_name": sub.name,
		"service_name":      sub.serviceName,
		"status":            status,
		"trace_id":          traceID,
		"attempt":           attempt,
		"payload":           json.RawMessage(append([]byte(nil), payload...)),
		"result":            result,
		"error":             errorString(err),
		"deliveries":        deliveries,
		"inserted_at":       insertedAt.UTC(),
		"picked_up_at":      optionalTimeValue(pickedUpAt),
		"finished_at":       optionalTimeValue(finishedAt),
		"duration_ms":       float64(duration) / float64(time.Millisecond),
	})
}

func messageIDFromMetadata(meta *nats.MsgMetadata, topic *topicDecl, msg *nats.Msg) string {
	if meta != nil {
		return fmt.Sprintf("%s:%d", meta.Stream, meta.Sequence.Stream)
	}
	if topic != nil {
		return sanitizeName(topic.name) + ":" + sanitizeName(msg.Subject)
	}
	return sanitizeName(msg.Subject)
}

func optionalTimeValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
	root := strings.TrimSpace(os.Getenv("ONLAVA_DEV_CACHE_DIR"))
	if root == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		root = filepathJoin(dir, "onlava")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
