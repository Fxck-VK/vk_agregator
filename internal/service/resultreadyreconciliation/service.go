// Package resultreadyreconciliation reconstructs missing result-ready outbox
// events after an operator has hard-drained all job workers and relays.
package resultreadyreconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/uow"
	"vk-ai-aggregator/internal/service/outboxrelay"
)

// FinalizationReadiness is the exact finalization gate shared with result
// delivery and capture.
type FinalizationReadiness interface {
	RequireCompletionReady(ctx context.Context, accountID, jobID uuid.UUID) error
}

// Result is a privacy-safe count-only summary of one bounded page.
type Result struct {
	Candidates int  `json:"candidates"`
	Eligible   int  `json:"eligible"`
	Existing   int  `json:"existing"`
	Created    int  `json:"created"`
	Blocked    int  `json:"blocked"`
	HasMore    bool `json:"has_more"`
}

// Service reconstructs missing durable finalization events. It has no
// publisher, delivery, billing, or provider dependency.
type Service struct {
	candidates domain.ResultReadyCandidateRepository
	readiness  FinalizationReadiness
	uow        uow.Manager
}

// New builds a bounded result-ready reconciliation service.
func New(
	candidates domain.ResultReadyCandidateRepository,
	readiness FinalizationReadiness,
	manager uow.Manager,
) *Service {
	return &Service{
		candidates: candidates,
		readiness:  readiness,
		uow:        manager,
	}
}

// Reconcile reads one anti-joined candidate page, applies the exact completion
// readiness gate, and inserts all eligible events in one short unit of work.
func (s *Service) Reconcile(ctx context.Context, limit int) (result Result, err error) {
	started := time.Now()
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		metrics.ObserveResultReadyReconciliationDuration(outcome, time.Since(started))
		if err == nil {
			metrics.AddResultReadyReconciliationItems("candidates", result.Candidates)
			metrics.AddResultReadyReconciliationItems("eligible", result.Eligible)
			metrics.AddResultReadyReconciliationItems("existing", result.Existing)
			metrics.AddResultReadyReconciliationItems("created", result.Created)
			metrics.AddResultReadyReconciliationItems("blocked", result.Blocked)
		}
	}()
	if s == nil || s.candidates == nil || s.readiness == nil || s.uow == nil {
		return Result{}, errors.New("resultreadyreconciliation: candidate, readiness, and unit-of-work dependencies are required")
	}
	if limit <= 0 {
		return Result{}, errors.New("resultreadyreconciliation: limit must be positive")
	}

	jobs, hasMore, err := s.candidates.ListMissingCanonicalResultReady(ctx, limit, nil)
	if err != nil {
		return Result{}, err
	}
	if len(jobs) > limit {
		return Result{}, errors.New("resultreadyreconciliation: candidate repository exceeded requested page")
	}

	result = Result{
		Candidates: len(jobs),
		HasMore:    hasMore,
	}
	events := make([]*domain.OutboxEvent, 0, len(jobs))
	for _, job := range jobs {
		if !eligible(job) {
			result.Blocked++
			continue
		}
		if err := s.readiness.RequireCompletionReady(ctx, job.AccountID, job.ID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				result.Blocked++
				continue
			}
			return Result{}, err
		}

		event, err := resultReadyEvent(job)
		if err != nil {
			return Result{}, err
		}
		result.Eligible++
		events = append(events, event)
	}
	if len(events) == 0 {
		return result, nil
	}

	err = s.uow.Within(ctx, func(ctx context.Context, repos uow.Repositories) error {
		if repos.Outbox == nil {
			return errors.New("resultreadyreconciliation: outbox repository is required")
		}
		for _, event := range events {
			created, err := repos.Outbox.AddIfAbsentByID(ctx, event)
			if err != nil {
				return err
			}
			if created {
				result.Created++
			} else {
				result.Existing++
			}
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func eligible(job *domain.Job) bool {
	if job == nil ||
		job.ID == uuid.Nil ||
		job.AccountID == uuid.Nil ||
		job.Status != domain.JobStatusResultReady ||
		!job.OperationType.Valid() ||
		!job.Modality.Valid() {
		return false
	}
	switch job.ResultMode {
	case domain.ResultModeAccountHistory, domain.ResultModeExternalPush:
	default:
		return false
	}
	return job.ValidateResultContract() == nil
}

func resultReadyEvent(job *domain.Job) (*domain.OutboxEvent, error) {
	payload, err := json.Marshal(struct {
		JobID         uuid.UUID            `json:"job_id"`
		Operation     domain.OperationType `json:"operation"`
		Modality      domain.Modality      `json:"modality"`
		CorrelationID string               `json:"correlation_id,omitempty"`
	}{
		JobID:         job.ID,
		Operation:     job.OperationType,
		Modality:      job.Modality,
		CorrelationID: job.CorrelationID,
	})
	if err != nil {
		return nil, fmt.Errorf("resultreadyreconciliation: encode event: %w", err)
	}
	return &domain.OutboxEvent{
		ID: uuid.NewSHA1(
			uuid.NameSpaceURL,
			[]byte("urn:vk-ai-aggregator:outbox:"+outboxrelay.EventJobResultReady+":"+job.ID.String()),
		),
		AggregateType: "job",
		AggregateID:   job.ID,
		EventType:     outboxrelay.EventJobResultReady,
		Payload:       payload,
		Status:        domain.OutboxPending,
	}, nil
}
