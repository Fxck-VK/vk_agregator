package domain

import (
	"time"

	"github.com/google/uuid"
)

// OperatorAuditEntry is a sanitized audit record for protected operator/admin
// actions. It must never contain raw tokens, request bodies, URLs, prompts,
// provider/payment payloads, idempotency keys or raw user identifiers.
type OperatorAuditEntry struct {
	ID       uuid.UUID `json:"id"`
	ActorRef string    `json:"actor_ref"`
	Action   string    `json:"action"`
	// TargetType is a bounded resource class such as jobs, referrals or billing.
	TargetType string `json:"target_type"`
	// TargetRef is a display-safe hash/ref, never the raw route id.
	TargetRef  string    `json:"target_ref,omitempty"`
	Result     string    `json:"result"`
	RequestRef string    `json:"request_ref,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// OperatorAuditFilter narrows protected operator audit listings.
type OperatorAuditFilter struct {
	Action     string
	TargetType string
	Result     string
}
