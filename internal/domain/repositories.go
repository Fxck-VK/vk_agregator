package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned by repositories when a requested entity does not
// exist. Callers compare with errors.Is.
var ErrNotFound = errors.New("domain: entity not found")

// ErrConflict is returned when a write violates a uniqueness or optimistic
// concurrency constraint (e.g. a duplicate idempotency key).
var ErrConflict = errors.New("domain: conflicting write")

// ErrInsufficientCredits is returned by the billing repository when a
// reservation cannot be satisfied by the available balance.
var ErrInsufficientCredits = errors.New("domain: insufficient credits")

// ErrCostCapExceeded is returned when a job's estimated cost exceeds the
// configured per-job spend cap.
var ErrCostCapExceeded = errors.New("domain: cost cap exceeded")

// OutboxStatus is the publishing state of an outbox event.
type OutboxStatus string

const (
	// OutboxPending means the event is awaiting publication.
	OutboxPending OutboxStatus = "pending"
	// OutboxPublished means the event was published to the bus.
	OutboxPublished OutboxStatus = "published"
	// OutboxFailed means publication permanently failed.
	OutboxFailed OutboxStatus = "failed"
)

// OutboxEvent is a domain event persisted in the same transaction as the state
// change that produced it, then published asynchronously (outbox pattern).
type OutboxEvent struct {
	// ID is the internal primary key.
	ID uuid.UUID `json:"id"`
	// AggregateType is the kind of aggregate that emitted the event.
	AggregateType string `json:"aggregate_type"`
	// AggregateID is the id of the emitting aggregate.
	AggregateID uuid.UUID `json:"aggregate_id"`
	// EventType is the event name, e.g. "event.job.created".
	EventType string `json:"event_type"`
	// Payload is the serialized event body.
	Payload json.RawMessage `json:"payload"`
	// Status is the publishing state.
	Status OutboxStatus `json:"status"`
	// Attempts is how many times publication has been tried.
	Attempts int `json:"attempts"`
	// NextAttemptAt is when the next publication should be tried.
	NextAttemptAt time.Time `json:"next_attempt_at"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// PublishedAt is when the event was published, if it was.
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// IdempotencyStatus is the processing state of an idempotency key.
type IdempotencyStatus string

const (
	// IdempotencyStarted means processing began for this key.
	IdempotencyStarted IdempotencyStatus = "started"
	// IdempotencyCompleted means the operation finished successfully.
	IdempotencyCompleted IdempotencyStatus = "completed"
	// IdempotencyFailed means the operation failed and may be retried.
	IdempotencyFailed IdempotencyStatus = "failed"
)

// IdempotencyRecord guarantees that an external operation runs at most once. The
// Key encodes the scope and natural identity of the operation.
type IdempotencyRecord struct {
	// Key is the globally unique idempotency key.
	Key string `json:"key"`
	// Scope is the operation class, e.g. "inbound_event" or "provider_submit".
	Scope string `json:"scope"`
	// ResourceType is the kind of resource the operation produced.
	ResourceType string `json:"resource_type"`
	// ResourceID is the id of the produced resource, set once known.
	ResourceID *uuid.UUID `json:"resource_id,omitempty"`
	// Status is the processing state of the key.
	Status IdempotencyStatus `json:"status"`
	// CreatedAt is the row creation timestamp.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is when the key may be garbage-collected.
	ExpiresAt time.Time `json:"expires_at"`
}

// UserRepository persists and retrieves users.
type UserRepository interface {
	// Create inserts a new user.
	Create(ctx context.Context, user *User) error
	// Update persists changes to an existing user.
	Update(ctx context.Context, user *User) error
	// GetByID fetches a user by internal id, ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	// GetByVKUserID fetches a user by external VK id, ErrNotFound if missing.
	GetByVKUserID(ctx context.Context, vkUserID int64) (*User, error)
}

// JobFilter narrows a job listing. Zero-valued fields are ignored, so an empty
// filter matches all jobs. It backs the admin jobs listing.
type JobFilter struct {
	// UserID, when set, restricts results to one user.
	UserID *uuid.UUID
	// Status, when non-empty, restricts results to one job status.
	Status JobStatus
	// Operation, when non-empty, restricts results to one operation type.
	Operation OperationType
}

// JobRepository persists jobs.
type JobRepository interface {
	// Create inserts a new job.
	Create(ctx context.Context, job *Job) error
	// GetByID fetches a job by id, ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (*Job, error)
	// GetByIdempotencyKey fetches a job by its idempotency key.
	GetByIdempotencyKey(ctx context.Context, key string) (*Job, error)
	// UpdateStatus applies an explicit state-machine transition, persisting the
	// new status together with any error code/message. It returns ErrConflict if
	// the stored status does not match from (lost-update protection).
	UpdateStatus(ctx context.Context, id uuid.UUID, from, to JobStatus, errCode, errMessage string) error
	// Update persists non-status changes to a job (cost, artifacts, routing).
	Update(ctx context.Context, job *Job) error
	// ListByUser returns the most recent jobs for a user, newest first.
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Job, error)
	// List returns jobs matching the filter, newest first, for admin queries.
	List(ctx context.Context, filter JobFilter, limit, offset int) ([]*Job, error)
	// CountActiveByUserOperation returns active, capacity-consuming jobs for one
	// user and operation. It is used by abuse protection before enqueueing more
	// expensive work for the same user.
	CountActiveByUserOperation(ctx context.Context, userID uuid.UUID, operation OperationType) (int, error)
	// CountSucceededByUser returns completed successful jobs for account stats.
	CountSucceededByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

// InboundEventRepository persists raw inbound events for audit and idempotent
// reprocessing.
type InboundEventRepository interface {
	// Create inserts a new inbound event.
	Create(ctx context.Context, event *InboundEvent) error
	// GetByIdempotencyKey fetches an event by idempotency key, ErrNotFound if
	// missing. It is used to detect duplicate deliveries.
	GetByIdempotencyKey(ctx context.Context, key string) (*InboundEvent, error)
	// SetStatus updates the processing status of an event.
	SetStatus(ctx context.Context, id uuid.UUID, status InboundEventStatus) error
}

// CommandRepository persists normalized commands parsed from inbound events.
type CommandRepository interface {
	// Create inserts a new command.
	Create(ctx context.Context, command *Command) error
	// GetByID fetches a command by id, ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (*Command, error)
	// GetByIdempotencyKey fetches a command by idempotency key, used to dedup
	// repeated inbound events for the same message.
	GetByIdempotencyKey(ctx context.Context, key string) (*Command, error)
	// ListByUser returns the most recent commands for a user, newest first.
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Command, error)
}

// ConversationRepository persists compact dialog memory for text models.
type ConversationRepository interface {
	// GetActiveByUserPeer returns the current active conversation for a VK peer.
	GetActiveByUserPeer(ctx context.Context, userID uuid.UUID, vkPeerID int64) (*Conversation, error)
	// GetActiveByReference returns the current active conversation for a
	// source-specific reference.
	GetActiveByReference(ctx context.Context, ref ConversationRef) (*Conversation, error)
	// GetByIDForUser returns one conversation owned by the user.
	GetByIDForUser(ctx context.Context, userID, conversationID uuid.UUID) (*Conversation, error)
	// ListByUserSource returns conversations for a user/source, newest first.
	ListByUserSource(ctx context.Context, userID uuid.UUID, source ConversationSource, limit, offset int) ([]*Conversation, error)
	// CreateConversation inserts a new conversation.
	CreateConversation(ctx context.Context, conversation *Conversation) error
	// SetConversationTitleIfEmpty fills an empty title without overwriting an
	// existing one. This keeps retrying chat jobs idempotent.
	SetConversationTitleIfEmpty(ctx context.Context, conversationID uuid.UUID, title string) error
	// UpsertMessage inserts a user/assistant message or returns the existing
	// row for the same job+role, making worker retries idempotent.
	UpsertMessage(ctx context.Context, message *ConversationMessage) (*ConversationMessage, error)
	// ListRecentMessagesBefore returns newest messages before beforeSeq and
	// after minSeq. Results are returned oldest first for prompt rendering.
	ListRecentMessagesBefore(ctx context.Context, conversationID uuid.UUID, beforeSeq, minSeq int64, limit int) ([]*ConversationMessage, error)
	// ListMessagesAfter returns messages newer than afterSeq, oldest first.
	ListMessagesAfter(ctx context.Context, conversationID uuid.UUID, afterSeq int64, limit int) ([]*ConversationMessage, error)
	// LatestSummary returns the most recent summary for a conversation.
	LatestSummary(ctx context.Context, conversationID uuid.UUID) (*ConversationSummary, error)
	// UpsertSummary creates or replaces the latest summary state for a
	// conversation. Only the newest summary is used for prompt rendering.
	UpsertSummary(ctx context.Context, summary *ConversationSummary) error
}

// ProviderTaskRepository persists provider tasks and their lifecycle.
type ProviderTaskRepository interface {
	// Create inserts a provider task for a job.
	Create(ctx context.Context, task *ProviderTask) error
	// Update persists changes to a provider task.
	Update(ctx context.Context, task *ProviderTask) error
	// GetByID fetches a provider task by id, ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (*ProviderTask, error)
	// GetByExternalID fetches a task by provider and external id, used to
	// reconcile incoming provider webhooks.
	GetByExternalID(ctx context.Context, provider ProviderName, externalID string) (*ProviderTask, error)
	// ListByJob returns all provider tasks for a job, oldest attempt first.
	ListByJob(ctx context.Context, jobID uuid.UUID) ([]*ProviderTask, error)
}

// ArtifactRepository persists artifacts and their variants.
type ArtifactRepository interface {
	// Create inserts a new artifact.
	Create(ctx context.Context, artifact *Artifact) error
	// Update persists changes to an artifact.
	Update(ctx context.Context, artifact *Artifact) error
	// GetByID fetches an artifact by id, ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (*Artifact, error)
	// GetBySHA256 fetches an artifact by content hash for deduplication.
	GetBySHA256(ctx context.Context, ownerID uuid.UUID, sha256 string) (*Artifact, error)

	// AddVariant inserts a derived variant of an artifact.
	AddVariant(ctx context.Context, variant *ArtifactVariant) error
	// ListVariants returns all variants of an artifact.
	ListVariants(ctx context.Context, artifactID uuid.UUID) ([]*ArtifactVariant, error)
}

// DeliveryRepository persists VK delivery attempts.
type DeliveryRepository interface {
	// Create inserts a new delivery attempt.
	Create(ctx context.Context, delivery *Delivery) error
	// Update persists changes to a delivery attempt.
	Update(ctx context.Context, delivery *Delivery) error
	// GetByID fetches a delivery by id, ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (*Delivery, error)
	// GetByIdempotencyKey fetches a delivery by idempotency key for dedup.
	GetByIdempotencyKey(ctx context.Context, key string) (*Delivery, error)
	// ListByJob returns all delivery attempts for a job.
	ListByJob(ctx context.Context, jobID uuid.UUID) ([]*Delivery, error)
}

// BillingRepository persists the append-only credit ledger, accounts and
// reservations. Balance is only ever changed through ledger entries.
type BillingRepository interface {
	// GetAccount fetches an account by id, ErrNotFound if missing.
	GetAccount(ctx context.Context, id uuid.UUID) (*CreditAccount, error)
	// GetAccountByUser fetches a user's account for a currency.
	GetAccountByUser(ctx context.Context, userID uuid.UUID, currency Currency) (*CreditAccount, error)
	// CreateAccount inserts a new credit account.
	CreateAccount(ctx context.Context, account *CreditAccount) error

	// AppendEntry inserts an immutable ledger entry and updates the cached
	// balance atomically. It returns ErrConflict on a duplicate idempotency key.
	AppendEntry(ctx context.Context, entry *LedgerEntry) error
	// ListEntries returns ledger entries for an account, newest first.
	ListEntries(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*LedgerEntry, error)

	// Reserve creates a reservation and its reserve ledger entry atomically,
	// returning ErrInsufficientCredits if the balance is too low.
	Reserve(ctx context.Context, reservation *CreditReservation) error
	// Capture converts a reservation into a charge with a capture entry.
	Capture(ctx context.Context, reservationID uuid.UUID, amount int64, idempotencyKey string) error
	// Release frees a reservation with a release entry.
	Release(ctx context.Context, reservationID uuid.UUID, idempotencyKey string) error
	// GetReservation fetches a reservation by id, ErrNotFound if missing.
	GetReservation(ctx context.Context, id uuid.UUID) (*CreditReservation, error)
	// GetReservationByJob fetches the most recent reservation for a job, used by
	// workers to capture credits without threading the reservation id through
	// the queue. ErrNotFound if the job has no reservation.
	GetReservationByJob(ctx context.Context, jobID uuid.UUID) (*CreditReservation, error)
}

// PaymentIntentFilter narrows admin payment-intent listings. Zero-valued
// fields are ignored.
type PaymentIntentFilter struct {
	UserID   *uuid.UUID
	Status   PaymentIntentStatus
	Provider PaymentProviderCode
}

// PaymentReconciliationFilter narrows payment intents that should be synced
// against the provider during reconciliation.
type PaymentReconciliationFilter struct {
	Provider      PaymentProviderCode
	Statuses      []PaymentIntentStatus
	UpdatedBefore time.Time
}

// PaymentRepository persists products, payment intents and provider webhook
// inbox rows. It does not mutate billing balances directly.
type PaymentRepository interface {
	// GetActiveProductByCode fetches an active product catalog entry by code.
	GetActiveProductByCode(ctx context.Context, code string) (*PaymentProduct, error)
	// GetProductByID fetches a product by id, active or inactive.
	GetProductByID(ctx context.Context, id uuid.UUID) (*PaymentProduct, error)

	// CreateIntent inserts a local payment intent snapshot.
	CreateIntent(ctx context.Context, intent *PaymentIntent) error
	// GetIntentByID fetches one payment intent by id.
	GetIntentByID(ctx context.Context, id uuid.UUID) (*PaymentIntent, error)
	// GetIntentByIdempotencyKey fetches one payment intent by idempotency key.
	GetIntentByIdempotencyKey(ctx context.Context, key string) (*PaymentIntent, error)
	// SetIntentProviderState stores the provider-created payment id,
	// confirmation URL and normalized provider status.
	SetIntentProviderState(ctx context.Context, id uuid.UUID, status PaymentIntentStatus, providerPaymentID, confirmationURL string) error
	// GetIntentByProviderPaymentID fetches one intent by normalized provider
	// payment id.
	GetIntentByProviderPaymentID(ctx context.Context, provider PaymentProviderCode, providerPaymentID string) (*PaymentIntent, error)
	// UpdateIntentStatus updates an intent status using optimistic state
	// matching. It returns ErrConflict when the current state changed.
	UpdateIntentStatus(ctx context.Context, id uuid.UUID, from, to PaymentIntentStatus) error
	// ListIntentsByUser lists intents for one user, newest first.
	ListIntentsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*PaymentIntent, error)
	// ListIntents lists intents for protected operator endpoints, newest first.
	ListIntents(ctx context.Context, filter PaymentIntentFilter, limit, offset int) ([]*PaymentIntent, error)
	// ListIntentsForReconciliation lists stale provider-backed intents that
	// should be synced against the payment provider.
	ListIntentsForReconciliation(ctx context.Context, filter PaymentReconciliationFilter, limit int) ([]*PaymentIntent, error)

	// CreateEvent stores a raw provider webhook inbox event. It returns false
	// with nil error when the dedup key already exists.
	CreateEvent(ctx context.Context, event *PaymentEvent) (bool, error)
	// GetEventByID fetches a provider webhook inbox event by id.
	GetEventByID(ctx context.Context, id uuid.UUID) (*PaymentEvent, error)
	// ListUnprocessedEvents lists unprocessed provider webhook inbox events in
	// receive order.
	ListUnprocessedEvents(ctx context.Context, provider PaymentProviderCode, limit int) ([]*PaymentEvent, error)
	// MarkEventProcessed marks a provider webhook inbox event as processed.
	MarkEventProcessed(ctx context.Context, id uuid.UUID, processedAt time.Time) error

	// CreateRefund inserts a local manual/provider refund row.
	CreateRefund(ctx context.Context, refund *PaymentRefund) error
	// GetRefundByIdempotencyKey fetches a refund by internal idempotency key.
	GetRefundByIdempotencyKey(ctx context.Context, key string) (*PaymentRefund, error)
	// SetRefundProviderState stores provider refund id and normalized refund status.
	SetRefundProviderState(ctx context.Context, id uuid.UUID, providerRefundID string, status PaymentRefundStatus) error
}

// ReferralRepository persists single-user referral codes and referral
// relations. It is shared by VK bot and future VK Mini App referral flows.
type ReferralRepository interface {
	// GetCodeByUserID fetches the stable referral code for one user.
	GetCodeByUserID(ctx context.Context, userID uuid.UUID) (*ReferralCode, error)
	// GetCode fetches a referral code by its public code value.
	GetCode(ctx context.Context, code string) (*ReferralCode, error)
	// CreateCode inserts a new public code for one user.
	CreateCode(ctx context.Context, code *ReferralCode) error
	// CreateReferral records a referral relation. It returns ErrConflict when
	// the referred user already has a referrer.
	CreateReferral(ctx context.Context, referral *Referral) error
	// GetReferralByReferredUserID fetches the relation for the invited user.
	GetReferralByReferredUserID(ctx context.Context, userID uuid.UUID) (*Referral, error)
	// CountByReferrer counts users invited by one referrer.
	CountByReferrer(ctx context.Context, referrerUserID uuid.UUID) (int, error)
	// MarkRewardApplied marks signup referral rewards as posted to billing.
	MarkRewardApplied(ctx context.Context, referralID uuid.UUID, rewardedAt time.Time) error
}

// OutboxRepository persists and drains domain events using the outbox pattern.
type OutboxRepository interface {
	// Add inserts an outbox event. It is expected to be called inside the same
	// transaction as the state change that produced the event.
	Add(ctx context.Context, event *OutboxEvent) error
	// FetchPending returns up to limit events ready for publication.
	FetchPending(ctx context.Context, limit int) ([]*OutboxEvent, error)
	// MarkPublished marks an event as successfully published.
	MarkPublished(ctx context.Context, id uuid.UUID, publishedAt time.Time) error
	// MarkFailed records a failed publication and schedules the next attempt.
	MarkFailed(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error
}

// IdempotencyRepository guarantees at-most-once processing of external
// operations such as inbound events, provider submits and deliveries.
type IdempotencyRepository interface {
	// GetOrCreate atomically creates a record in the started state, or returns
	// the existing record. The boolean reports whether it was newly created.
	GetOrCreate(ctx context.Context, record *IdempotencyRecord) (existing *IdempotencyRecord, created bool, err error)
	// MarkCompleted records successful completion and the resource produced.
	MarkCompleted(ctx context.Context, key string, resourceID uuid.UUID) error
	// MarkFailed records a failed attempt so it may be retried.
	MarkFailed(ctx context.Context, key string) error
	// Get fetches a record by key, ErrNotFound if missing.
	Get(ctx context.Context, key string) (*IdempotencyRecord, error)
}
