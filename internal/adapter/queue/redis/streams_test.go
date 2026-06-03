package redisqueue_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
)

// testClient connects to the Redis instance pointed to by TEST_REDIS_ADDR,
// skipping the test when it is not configured so the default `go test ./...`
// stays green without external infrastructure.
func testClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set; skipping Redis Streams integration test")
	}
	client := redisqueue.NewClient(addr, os.Getenv("TEST_REDIS_PASSWORD"), 0)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}
	return client
}

func TestStreamForOperation(t *testing.T) {
	cases := map[domain.OperationType]string{
		domain.OperationTextGenerate:      redisqueue.StreamText,
		domain.OperationImageGenerate:     redisqueue.StreamImage,
		domain.OperationImageEdit:         redisqueue.StreamImage,
		domain.OperationVideoGenerate:     redisqueue.StreamVideo,
		domain.OperationVideoImageToVideo: redisqueue.StreamVideo,
	}
	for op, want := range cases {
		if got := redisqueue.StreamForOperation(op); got != want {
			t.Errorf("StreamForOperation(%s) = %s, want %s", op, got, want)
		}
	}
}

func TestPublishConsumeAck(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	group := "test-group-" + uuid.NewString()
	consumer := "c1"
	con := redisqueue.NewConsumer(client, group, consumer)
	if err := con.EnsureGroups(ctx, redisqueue.StreamText); err != nil {
		t.Fatalf("ensure groups: %v", err)
	}
	t.Cleanup(func() {
		client.XGroupDestroy(context.Background(), redisqueue.StreamText, group)
	})

	pub := redisqueue.NewPublisher(client, 1000)
	task := queue.Task{
		JobID:         uuid.New(),
		Operation:     domain.OperationTextGenerate,
		Modality:      domain.ModalityText,
		CorrelationID: "corr-1",
	}
	if err := pub.Enqueue(ctx, task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deliveries, err := con.Read(ctx, redisqueue.ReadOptions{Streams: []string{redisqueue.StreamText}, Count: 10, Block: time.Second})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	got := deliveries[0]
	if got.Task.JobID != task.JobID || got.Task.CorrelationID != "corr-1" {
		t.Fatalf("unexpected task: %+v", got.Task)
	}
	if err := con.Ack(ctx, got.Stream, got.ID); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// After ack the pending entries list should be empty for this consumer.
	pending, err := client.XPending(ctx, redisqueue.StreamText, group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected no pending entries, got %d", pending.Count)
	}
}
