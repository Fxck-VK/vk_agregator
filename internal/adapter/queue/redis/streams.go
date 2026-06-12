// Package redisqueue implements the task-queue contract on top of Redis Streams
// with consumer groups. Each modality has its own stream so slow video jobs
// never block fast text jobs, and consumer groups give at-least-once delivery
// with explicit acknowledgement and pending-entry recovery.
package redisqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/platform/tracing"
)

// Stream names, one per worker pool.
const (
	StreamText         = "stream:jobs:text"
	StreamImage        = "stream:jobs:image"
	StreamVideo        = "stream:jobs:video"
	StreamDelivery     = "stream:jobs:delivery"
	StreamProviderPoll = "stream:jobs:provider_poll"
	// StreamDLQ is the dead-letter stream for tasks that exhausted their retry
	// budget. It is not consumed by workers; entries are inspected/replayed by
	// operators.
	StreamDLQ = "stream:jobs:dlq"
)

// AllStreams lists every worker-consumed stream (the DLQ is intentionally
// excluded; nothing auto-consumes it).
var AllStreams = []string{StreamText, StreamImage, StreamVideo, StreamDelivery, StreamProviderPoll}

// AllStreamsWithDLQ lists worker streams plus the operator-inspected DLQ for
// maintenance trimming.
var AllStreamsWithDLQ = []string{StreamText, StreamImage, StreamVideo, StreamDelivery, StreamProviderPoll, StreamDLQ}

// taskField is the Redis stream entry field that carries the JSON task body.
const taskField = "task"

// QueuePressure describes bounded, non-payload queue pressure for one stream.
// It is safe for logs/metrics because it contains no task payload or user data.
type QueuePressure struct {
	Stream    string
	Group     string
	Lag       int64
	Pending   int64
	Threshold int64
}

// Total returns lag+pending, ignoring Redis' unknown lag sentinel.
func (p QueuePressure) Total() int64 {
	lag := p.Lag
	if lag < 0 {
		lag = 0
	}
	return lag + p.Pending
}

// BackpressureError reports that a Redis stream is over its configured safe
// processing threshold.
type BackpressureError struct {
	Pressure QueuePressure
}

func (e BackpressureError) Error() string {
	return fmt.Sprintf("redisqueue: stream %s/%s backlog %d over threshold %d", e.Pressure.Stream, e.Pressure.Group, e.Pressure.Total(), e.Pressure.Threshold)
}

// BackpressureGuard reads Redis consumer-group lag/pending counts to decide
// whether new expensive work should be refused before reservation/provider
// calls. It never reads task payloads.
type BackpressureGuard struct {
	client    redis.Cmdable
	group     string
	threshold int64
}

// NewBackpressureGuard builds a queue pressure guard. threshold <= 0 disables
// the guard.
func NewBackpressureGuard(client redis.Cmdable, group string, threshold int64) *BackpressureGuard {
	return &BackpressureGuard{client: client, group: strings.TrimSpace(group), threshold: threshold}
}

// Check refuses when any supplied stream's consumer-group lag+pending reaches
// the configured threshold. Missing streams/groups are treated as no pressure so
// local startup and tests are not blocked before workers create groups.
func (g *BackpressureGuard) Check(ctx context.Context, streams ...string) error {
	if g == nil || g.client == nil || g.threshold <= 0 || g.group == "" {
		return nil
	}
	for _, stream := range streams {
		stream = strings.TrimSpace(stream)
		if stream == "" {
			continue
		}
		pressure, ok, err := g.pressure(ctx, stream)
		if err != nil {
			return err
		}
		if ok && pressure.Total() >= g.threshold {
			return BackpressureError{Pressure: pressure}
		}
	}
	return nil
}

func (g *BackpressureGuard) pressure(ctx context.Context, stream string) (QueuePressure, bool, error) {
	groups, err := g.client.XInfoGroups(ctx, stream).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || isNoGroup(err) {
			return QueuePressure{}, false, nil
		}
		return QueuePressure{}, false, fmt.Errorf("redisqueue: xinfo groups %s: %w", stream, err)
	}
	for _, group := range groups {
		if group.Name != g.group {
			continue
		}
		return QueuePressure{
			Stream:    stream,
			Group:     g.group,
			Lag:       group.Lag,
			Pending:   group.Pending,
			Threshold: g.threshold,
		}, true, nil
	}
	return QueuePressure{}, false, nil
}

// StreamForOperation maps an operation to the stream its generation work runs
// on. Delivery and provider-poll are produced explicitly, not from operations.
func StreamForOperation(op domain.OperationType) string {
	switch op {
	case domain.OperationTextGenerate:
		return StreamText
	case domain.OperationImageGenerate, domain.OperationImageEdit, domain.OperationImageUpscale:
		return StreamImage
	case domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return StreamVideo
	default:
		return StreamText
	}
}

// NewClient builds a Redis client for the given address (host:port).
func NewClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})
}

// NewClientWithPool builds a Redis client with an explicit connection pool size
// (0 leaves the go-redis default) (audit SC1).
func NewClientWithPool(addr, password string, db, poolSize int) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db, PoolSize: poolSize})
}

// Publisher writes tasks to Redis Streams. It satisfies queue.Publisher by
// routing each task to the stream for its operation.
type Publisher struct {
	client redis.Cmdable
	maxLen int64
}

// NewPublisher builds a Publisher. maxLen caps each stream's length with
// approximate trimming (0 disables trimming).
func NewPublisher(client redis.Cmdable, maxLen int64) *Publisher {
	return &Publisher{client: client, maxLen: maxLen}
}

var _ queue.Publisher = (*Publisher)(nil)

// Enqueue routes the task to the stream for its operation.
func (p *Publisher) Enqueue(ctx context.Context, task queue.Task) error {
	return p.PublishTo(ctx, StreamForOperation(task.Operation), task)
}

// PublishTo appends the task to a specific stream (used for delivery and
// provider-poll work that is not derived from an operation).
func (p *Publisher) PublishTo(ctx context.Context, stream string, task queue.Task) error {
	if task.Traceparent != "" {
		ctx = tracing.ContextWithTraceparent(ctx, task.Traceparent)
	}
	ctx, span := tracing.Start(ctx, "queue.publish",
		attribute.String("messaging.system", "redis"),
		attribute.String("messaging.destination.name", stream),
		attribute.String("job.id", task.JobID.String()),
		attribute.String("operation", string(task.Operation)),
		tracing.CorrelationAttr(task.CorrelationID),
	)
	defer span.End()

	if task.Traceparent == "" {
		task.Traceparent = tracing.Traceparent(ctx)
	}
	body, err := json.Marshal(task)
	if err != nil {
		tracing.RecordError(span, err)
		return fmt.Errorf("redisqueue: marshal task: %w", err)
	}
	args := &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{taskField: body},
	}
	if p.maxLen > 0 {
		args.MaxLen = p.maxLen
		args.Approx = true
	}
	if err := p.client.XAdd(ctx, args).Err(); err != nil {
		tracing.RecordError(span, err)
		return fmt.Errorf("redisqueue: xadd %s: %w", stream, err)
	}
	return nil
}

// Delivery is a single task read from a stream under a consumer group. The ID
// must be passed to Ack once the task has been processed.
type Delivery struct {
	Stream string
	ID     string
	Task   queue.Task
}

// ReadOptions configures a consumer read.
type ReadOptions struct {
	// Streams to read from; defaults to AllStreams when empty.
	Streams []string
	// Count is the max number of entries to return per read.
	Count int64
	// Block is how long to block waiting for new entries (0 = no blocking).
	Block time.Duration
}

// Consumer reads tasks from streams as part of a consumer group, giving
// at-least-once delivery with explicit acknowledgement.
type Consumer struct {
	client   redis.Cmdable
	group    string
	consumer string
}

// NewConsumer builds a Consumer identified by group and consumer name.
func NewConsumer(client redis.Cmdable, group, consumer string) *Consumer {
	return &Consumer{client: client, group: group, consumer: consumer}
}

// EnsureGroups creates the consumer group on each stream, creating the streams
// if missing. An already-existing group is not an error.
func (c *Consumer) EnsureGroups(ctx context.Context, streams ...string) error {
	if len(streams) == 0 {
		streams = AllStreams
	}
	for _, stream := range streams {
		err := c.client.XGroupCreateMkStream(ctx, stream, c.group, "$").Err()
		if err != nil && !isBusyGroup(err) {
			return fmt.Errorf("redisqueue: create group %s/%s: %w", stream, c.group, err)
		}
	}
	return nil
}

// Read fetches new, never-delivered entries for this consumer.
func (c *Consumer) Read(ctx context.Context, opts ReadOptions) ([]Delivery, error) {
	streams := opts.Streams
	if len(streams) == 0 {
		streams = AllStreams
	}
	// XREADGROUP wants stream names followed by an ID per stream (">" = new).
	args := make([]string, 0, len(streams)*2)
	args = append(args, streams...)
	for range streams {
		args = append(args, ">")
	}
	count := opts.Count
	if count <= 0 {
		count = 10
	}

	res, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  args,
		Count:    count,
		Block:    opts.Block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redisqueue: xreadgroup: %w", err)
	}

	var out []Delivery
	for _, stream := range res {
		for _, msg := range stream.Messages {
			task, derr := decodeTask(msg)
			if derr != nil {
				// Skip and acknowledge poison messages so they do not loop.
				_ = c.Ack(ctx, stream.Stream, msg.ID)
				continue
			}
			out = append(out, Delivery{Stream: stream.Stream, ID: msg.ID, Task: task})
		}
	}
	return out, nil
}

// Ack acknowledges processed entries so they leave the pending list.
func (c *Consumer) Ack(ctx context.Context, stream string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.client.XAck(ctx, stream, c.group, ids...).Err(); err != nil {
		return fmt.Errorf("redisqueue: xack %s: %w", stream, err)
	}
	return nil
}

// AutoClaim reclaims entries that have been pending (delivered but unacked)
// longer than minIdle and reassigns them to this consumer. It is used on
// startup to recover work left behind by a crashed consumer, giving
// at-least-once processing across restarts.
func (c *Consumer) AutoClaim(ctx context.Context, stream string, minIdle time.Duration, count int64) ([]Delivery, error) {
	if count <= 0 {
		count = 100
	}
	msgs, _, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    c.group,
		Consumer: c.consumer,
		MinIdle:  minIdle,
		Start:    "0",
		Count:    count,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("redisqueue: xautoclaim %s: %w", stream, err)
	}

	var out []Delivery
	for _, msg := range msgs {
		task, derr := decodeTask(msg)
		if derr != nil {
			_ = c.Ack(ctx, stream, msg.ID)
			continue
		}
		out = append(out, Delivery{Stream: stream, ID: msg.ID, Task: task})
	}
	return out, nil
}

// TrimStreams caps Redis Stream backlogs. It is exact rather than approximate
// so operator cleanup has deterministic results.
func TrimStreams(ctx context.Context, client redis.Cmdable, maxLen int64, streams ...string) (map[string]int64, error) {
	if maxLen <= 0 {
		return map[string]int64{}, nil
	}
	if len(streams) == 0 {
		streams = AllStreamsWithDLQ
	}
	trimmed := make(map[string]int64, len(streams))
	for _, stream := range streams {
		n, err := client.XTrimMaxLen(ctx, stream, maxLen).Result()
		if err != nil {
			return trimmed, fmt.Errorf("redisqueue: trim %s: %w", stream, err)
		}
		trimmed[stream] = n
	}
	return trimmed, nil
}

// CollectMetrics observes stream depth, oldest-entry age and consumer-group
// pending counts. It uses only low-cardinality stream/group labels and never
// reads or exports task payloads.
func CollectMetrics(ctx context.Context, client redis.Cmdable, group string, streams ...string) error {
	if client == nil {
		return nil
	}
	if len(streams) == 0 {
		streams = AllStreamsWithDLQ
	}
	now := time.Now()
	var firstErr error
	for _, stream := range streams {
		length, err := client.XLen(ctx, stream).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				length = 0
			} else {
				firstErr = joinFirstErr(firstErr, fmt.Errorf("redisqueue: xlen %s: %w", stream, err))
				continue
			}
		}
		metrics.QueueDepth.WithLabelValues(stream).Set(float64(length))
		metrics.SetMediaQueueBacklog(queueClassForStream(stream), length)
		if length <= 0 {
			metrics.QueueOldestAgeSeconds.WithLabelValues(stream).Set(0)
		} else {
			msgs, err := client.XRangeN(ctx, stream, "-", "+", 1).Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				firstErr = joinFirstErr(firstErr, fmt.Errorf("redisqueue: xrange %s: %w", stream, err))
			} else if len(msgs) == 0 {
				metrics.QueueOldestAgeSeconds.WithLabelValues(stream).Set(0)
			} else {
				metrics.QueueOldestAgeSeconds.WithLabelValues(stream).Set(streamIDAgeSeconds(now, msgs[0].ID))
			}
		}
		if strings.TrimSpace(group) == "" {
			continue
		}
		pending, err := client.XPending(ctx, stream, group).Result()
		if err != nil {
			if isNoGroup(err) || errors.Is(err, redis.Nil) {
				metrics.QueueConsumerLag.WithLabelValues(stream, group).Set(0)
				continue
			}
			firstErr = joinFirstErr(firstErr, fmt.Errorf("redisqueue: xpending %s/%s: %w", stream, group, err))
			continue
		}
		metrics.QueueConsumerLag.WithLabelValues(stream, group).Set(float64(pending.Count))
	}
	return firstErr
}

func queueClassForStream(stream string) string {
	switch strings.TrimSpace(stream) {
	case StreamText:
		return "text"
	case StreamImage:
		return "image"
	case StreamVideo:
		return "video"
	case StreamDelivery:
		return "delivery"
	case StreamProviderPoll:
		return "provider_poll"
	case StreamDLQ:
		return "dlq"
	default:
		return "unknown"
	}
}

func streamIDAgeSeconds(now time.Time, id string) float64 {
	raw, _, ok := strings.Cut(id, "-")
	if !ok {
		raw = id
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || ms <= 0 {
		return 0
	}
	age := now.Sub(time.UnixMilli(ms))
	if age <= 0 {
		return 0
	}
	return age.Seconds()
}

func joinFirstErr(first error, next error) error {
	if first != nil {
		return first
	}
	return next
}

// Trimmer performs stream retention cleanup for maintenance jobs.
type Trimmer struct {
	client  redis.Cmdable
	maxLen  int64
	streams []string
}

// NewTrimmer builds a Redis stream trimmer. maxLen <= 0 disables trimming.
func NewTrimmer(client redis.Cmdable, maxLen int64, streams ...string) *Trimmer {
	return &Trimmer{client: client, maxLen: maxLen, streams: streams}
}

// Trim caps configured streams and returns per-stream entries removed.
func (t *Trimmer) Trim(ctx context.Context) (map[string]int64, error) {
	return TrimStreams(ctx, t.client, t.maxLen, t.streams...)
}

func decodeTask(msg redis.XMessage) (queue.Task, error) {
	var task queue.Task
	raw, ok := msg.Values[taskField]
	if !ok {
		return task, errors.New("redisqueue: message missing task field")
	}
	switch v := raw.(type) {
	case string:
		return task, json.Unmarshal([]byte(v), &task)
	case []byte:
		return task, json.Unmarshal(v, &task)
	default:
		return task, fmt.Errorf("redisqueue: unexpected task field type %T", raw)
	}
}

func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}

func isNoGroup(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "NOGROUP") || strings.Contains(msg, "NO SUCH KEY")
}
