package worker

import (
	"context"
	"errors"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/queue"
)

// PollWorker reconciles asynchronous provider tasks. It consumes the
// provider-poll stream and runs Provider Poll -> Update Status -> Requeue (while
// still running) -> Artifact -> Delivery Queue (on success).
type PollWorker struct {
	processor
}

// NewPollWorker builds a PollWorker from shared dependencies.
func NewPollWorker(d Deps) *PollWorker {
	return &PollWorker{processor: newProcessor(d)}
}

// Process polls the job's latest provider task once and applies the result.
// Returning nil acknowledges the poll message; while the task is still running
// a fresh poll message is enqueued, so progress continues without holding the
// original message. Returning an error leaves the message pending for recovery.
func (w *PollWorker) Process(ctx context.Context, task queue.Task) error {
	job, err := w.jobs.GetByID(ctx, task.JobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if isDone(job.Status) {
		return nil
	}

	pt, err := w.latestTask(ctx, job.ID)
	if err != nil {
		return err
	}
	if pt == nil {
		// Nothing to poll; the generation worker has not submitted yet.
		return nil
	}
	if pt.Status.IsTerminal() {
		return nil
	}

	provider, err := w.providers.ForName(pt.Provider)
	if err != nil {
		return err
	}
	return w.pollOnce(ctx, job, pt, provider, task)
}
