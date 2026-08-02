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

func TestConversationTitleEngineUsesDedicatedStreamAndSafeInFlightLease(t *testing.T) {
	engine := NewConversationTitleEngine(&engineReaderFake{}, func(context.Context, queue.Task) error { return nil }, nil)
	if len(engine.streams) != 1 || engine.streams[0] != redisqueue.StreamConversationTitle {
		t.Fatalf("title engine streams = %#v", engine.streams)
	}
	if engine.count != 1 {
		t.Fatalf("title engine read batch = %d, want 1 so one lease covers one provider call", engine.count)
	}
	if engine.minIdle <= 15*time.Second+500*time.Millisecond {
		t.Fatalf("title engine min idle = %s, want longer than the bounded initial title work", engine.minIdle)
	}
	if got := streamPhase(redisqueue.StreamConversationTitle); got != "conversation_title" {
		t.Fatalf("title stream phase = %q", got)
	}
}

func TestConversationTitleEngineDoesNotReclaimAnInFlightProviderCall(t *testing.T) {
	// This reader models an entry already being processed by a first title
	// worker. Redis will return it to a second consumer only when the configured
	// idle lease has elapsed.
	reader := &inFlightTitleReader{
		pendingFor: 15*time.Second + 500*time.Millisecond,
		delivery: redisqueue.Delivery{
			Stream: redisqueue.StreamConversationTitle,
			ID:     "title-in-flight",
			Task: queue.Task{
				JobID:     uuid.New(),
				Operation: domain.OperationTextGenerate,
				Modality:  domain.ModalityText,
			},
		},
	}
	secondConsumerCalls := 0
	engine := NewConversationTitleEngine(reader, func(context.Context, queue.Task) error {
		secondConsumerCalls++
		return nil
	}, nil)

	handled, err := engine.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll() = %v", err)
	}
	if reader.observedMinIdle <= reader.pendingFor {
		t.Fatalf("second consumer lease = %s, must outlive first provider call of %s", reader.observedMinIdle, reader.pendingFor)
	}
	if handled != 0 || secondConsumerCalls != 0 {
		t.Fatalf("second consumer reclaimed a live title call: handled=%d calls=%d", handled, secondConsumerCalls)
	}
}

func TestConversationTitleEngineAcknowledgesSuccessfulTitleWork(t *testing.T) {
	task := queue.Task{JobID: uuid.New(), Operation: domain.OperationTextGenerate, Modality: domain.ModalityText}
	reader := &engineReaderFake{read: []redisqueue.Delivery{{
		Stream: redisqueue.StreamConversationTitle,
		ID:     "title-1",
		Task:   task,
	}}}
	engine := NewConversationTitleEngine(reader, func(_ context.Context, got queue.Task) error {
		if got.JobID != task.JobID {
			t.Fatalf("task job id = %s, want %s", got.JobID, task.JobID)
		}
		return nil
	}, nil)

	count, err := engine.Poll(context.Background())
	if err != nil {
		t.Fatalf("Poll() = %v", err)
	}
	if count != 1 || len(reader.acked) != 1 || reader.acked[0] != "title-1" {
		t.Fatalf("title delivery was not acknowledged: count=%d acked=%v", count, reader.acked)
	}
}

type inFlightTitleReader struct {
	pendingFor      time.Duration
	delivery        redisqueue.Delivery
	observedMinIdle time.Duration
}

func (r *inFlightTitleReader) Read(context.Context, redisqueue.ReadOptions) ([]redisqueue.Delivery, error) {
	return nil, nil
}

func (r *inFlightTitleReader) AutoClaim(_ context.Context, _ string, minIdle time.Duration, _ int64) ([]redisqueue.Delivery, error) {
	r.observedMinIdle = minIdle
	if minIdle <= r.pendingFor {
		return []redisqueue.Delivery{r.delivery}, nil
	}
	return nil, nil
}

func (r *inFlightTitleReader) Ack(context.Context, string, ...string) error { return nil }
