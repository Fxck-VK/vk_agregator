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
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
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

// taskField is the Redis stream entry field that carries the JSON task body.
const taskField = "task"

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
	body, err := json.Marshal(task)
	if err != nil {
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
