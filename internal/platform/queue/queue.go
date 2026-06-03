// Package queue defines the task-queue contract used to hand asynchronous work
// off to worker pools, plus a simple in-memory implementation for tests and
// local development. Different modalities map to different named queues so that,
// for example, slow video jobs never block fast text jobs.
package queue

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// Task is a unit of asynchronous work enqueued for a worker pool.
type Task struct {
	// JobID is the job the task advances.
	JobID uuid.UUID `json:"job_id"`
	// Operation is the operation the worker must perform.
	Operation domain.OperationType `json:"operation"`
	// Modality is the content kind, used to pick the queue.
	Modality domain.Modality `json:"modality"`
	// CorrelationID links the task to the originating request flow.
	CorrelationID string `json:"correlation_id"`
	// Attempt is the delivery attempt counter for this task. It is incremented
	// each time the task is re-enqueued after a retryable failure and is used to
	// enforce a hard retry budget (after which the task is dead-lettered).
	Attempt int `json:"attempt,omitempty"`
}

// QueueName returns the logical queue a task belongs to, derived from its
// operation/modality (e.g. "queue.video.generate").
func (t Task) QueueName() string {
	switch t.Operation {
	case domain.OperationTextGenerate:
		return "queue.text.generate"
	case domain.OperationImageGenerate:
		return "queue.image.generate"
	case domain.OperationImageEdit:
		return "queue.image.edit"
	case domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return "queue.video.generate"
	case domain.OperationAudioTTS, domain.OperationAudioSTT:
		return "queue.audio.generate"
	default:
		return "queue." + string(t.Modality) + ".generate"
	}
}

// Publisher enqueues tasks for asynchronous processing.
type Publisher interface {
	Enqueue(ctx context.Context, task Task) error
}

// MemoryPublisher is an in-memory Publisher that records enqueued tasks per
// queue name. It is safe for concurrent use.
type MemoryPublisher struct {
	mu     sync.Mutex
	queues map[string][]Task
}

// NewMemoryPublisher builds an empty in-memory publisher.
func NewMemoryPublisher() *MemoryPublisher {
	return &MemoryPublisher{queues: make(map[string][]Task)}
}

var _ Publisher = (*MemoryPublisher)(nil)

// Enqueue appends the task to its queue.
func (p *MemoryPublisher) Enqueue(_ context.Context, task Task) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	name := task.QueueName()
	p.queues[name] = append(p.queues[name], task)
	return nil
}

// Tasks returns a copy of the tasks enqueued on the named queue.
func (p *MemoryPublisher) Tasks(queueName string) []Task {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Task, len(p.queues[queueName]))
	copy(out, p.queues[queueName])
	return out
}

// Len returns the total number of enqueued tasks across all queues.
func (p *MemoryPublisher) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, q := range p.queues {
		total += len(q)
	}
	return total
}
