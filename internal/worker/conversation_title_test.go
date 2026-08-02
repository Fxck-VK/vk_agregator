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

func TestConversationTitleEngineUsesDedicatedStreamAndShortReclaim(t *testing.T) {
	engine := NewConversationTitleEngine(&engineReaderFake{}, func(context.Context, queue.Task) error { return nil }, nil)
	if len(engine.streams) != 1 || engine.streams[0] != redisqueue.StreamConversationTitle {
		t.Fatalf("title engine streams = %#v", engine.streams)
	}
	if engine.minIdle != 2*time.Second {
		t.Fatalf("title engine min idle = %s, want 2s", engine.minIdle)
	}
	if got := streamPhase(redisqueue.StreamConversationTitle); got != "conversation_title" {
		t.Fatalf("title stream phase = %q", got)
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
