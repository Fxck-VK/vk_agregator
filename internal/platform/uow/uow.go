// Package uow defines a small unit-of-work abstraction that lets services run
// several repository writes inside a single database transaction without
// depending on a concrete storage implementation.
package uow

import (
	"context"

	"vk-ai-aggregator/internal/domain"
)

// Repositories is the set of repositories bound to a single transaction. New
// fields can be added as more services need transactional composition.
type Repositories struct {
	// Jobs writes jobs within the transaction.
	Jobs domain.JobRepository
	// Outbox writes domain events within the same transaction as the state
	// change that produced them (transactional outbox pattern).
	Outbox domain.OutboxRepository
}

// Manager runs a unit of work inside a transaction, committing on success and
// rolling back on error.
type Manager interface {
	Within(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}
