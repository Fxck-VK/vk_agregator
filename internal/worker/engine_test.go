package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
)

type engineReaderFake struct {
	read      []redisqueue.Delivery
	reclaimed []redisqueue.Delivery
	acked     []string
}

func (r *engineReaderFake) Read(context.Context, redisqueue.ReadOptions) ([]redisqueue.Delivery, error) {
	out := append([]redisqueue.Delivery(nil), r.read...)
	r.read = nil
	return out, nil
}

func (r *engineReaderFake) AutoClaim(context.Context, string, time.Duration, int64) ([]redisqueue.Delivery, error) {
	out := append([]redisqueue.Delivery(nil), r.reclaimed...)
	r.reclaimed = nil
	return out, nil
}

func (r *engineReaderFake) Ack(_ context.Context, _ string, ids ...string) error {
	r.acked = append(r.acked, ids...)
	return nil
}

func TestPollReclaimsIdlePendingWork(t *testing.T) {
	task := queue.Task{
		JobID:     uuid.New(),
		Operation: domain.OperationVideoGenerate,
		Modality:  domain.ModalityVideo,
	}
	reader := &engineReaderFake{
		reclaimed: []redisqueue.Delivery{{
			Stream: redisqueue.StreamDelivery,
			ID:     "pending-1",
			Task:   task,
		}},
	}
	var handled []uuid.UUID
	engine := NewEngine(reader, []string{redisqueue.StreamDelivery}, func(_ context.Context, task queue.Task) error {
		handled = append(handled, task.JobID)
		return nil
	}, WithBlock(0), WithCount(1), WithMinIdle(time.Second))

	count, err := engine.Poll(context.Background())
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if count != 1 || len(handled) != 1 || handled[0] != task.JobID {
		t.Fatalf("pending task was not reclaimed and handled, count=%d handled=%v", count, handled)
	}
	if len(reader.acked) != 1 || reader.acked[0] != "pending-1" {
		t.Fatalf("reclaimed task was not acknowledged, acked=%v", reader.acked)
	}
}
