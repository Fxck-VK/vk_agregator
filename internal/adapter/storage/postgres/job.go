package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// JobRepository is the PostgreSQL implementation of domain.JobRepository.
type JobRepository struct {
	db Querier
}

// NewJobRepository builds a JobRepository over the given querier.
func NewJobRepository(db Querier) *JobRepository {
	return &JobRepository{db: db}
}

var _ domain.JobRepository = (*JobRepository)(nil)

const jobColumns = `id, user_id, account_id, source, channel, recipient_ref, thread_ref, result_mode,
	target_channel, target_recipient_ref, target_thread_ref, vk_peer_id, command_id, operation_type, modality,
	provider_id, model_id, status, priority, idempotency_key, correlation_id,
	input_artifact_ids, output_artifact_ids, params, pricing_snapshot, cost_estimate, cost_reserved,
	cost_captured, error_code, error_message, created_at, updated_at, expires_at`

// Create inserts a new job.
func (r *JobRepository) Create(ctx context.Context, job *domain.Job) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if len(job.Params) == 0 {
		job.Params = []byte("{}")
	}
	job.Source = strings.TrimSpace(job.Source)
	if job.Source == "" {
		job.Source = "unknown"
	}
	if job.ResultMode == "" {
		job.ResultMode = domain.ResultModeLegacyUnknown
	}
	channel, recipientRef, threadRef := channelContextValues(job.ChannelContext)
	targetChannel, targetRecipientRef, targetThreadRef := deliveryTargetValues(job.DeliveryTarget)
	// command_id is nullable; pass nil when the job has no associated command
	// (e.g. jobs created directly through the Mini App BFF).
	var commandID *uuid.UUID
	if job.CommandID != uuid.Nil {
		commandID = &job.CommandID
	}
	const q = `
		INSERT INTO jobs (
			id, user_id, account_id, source, channel, recipient_ref, thread_ref, result_mode,
			target_channel, target_recipient_ref, target_thread_ref, vk_peer_id, command_id, operation_type, modality,
			provider_id, model_id, status, priority, idempotency_key, correlation_id,
			input_artifact_ids, output_artifact_ids, params, pricing_snapshot, cost_estimate, cost_reserved,
			cost_captured, error_code, error_message, expires_at
		) VALUES (
			$1, $2, COALESCE($3::uuid, (SELECT account_id FROM users WHERE id = $2)), $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26, $27,
			$28, $29, $30, $31
		)
		RETURNING ` + jobColumns
	row := r.db.QueryRow(ctx, q,
		job.ID, nullableUUID(job.UserID), nullableUUID(job.AccountID), job.Source, channel, recipientRef, threadRef, job.ResultMode,
		targetChannel, targetRecipientRef, targetThreadRef, nullableInt64(job.VKPeerID), commandID, job.OperationType, job.Modality,
		job.ProviderID, job.ModelID, job.Status, job.Priority, job.IdempotencyKey, job.CorrelationID,
		uuidArray(job.InputArtifactIDs), uuidArray(job.OutputArtifactIDs), []byte(job.Params), nullableJSON(job.PricingSnapshot), job.CostEstimate, job.CostReserved,
		job.CostCaptured, job.ErrorCode, job.ErrorMessage, job.ExpiresAt,
	)
	return mapError(scanJob(row, job))
}

// GetByID fetches a job by id.
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	const q = `SELECT ` + jobColumns + ` FROM jobs WHERE id = $1`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, q, id), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByIDForAccount fetches a job only for its exact canonical account owner.
func (r *JobRepository) GetByIDForAccount(ctx context.Context, accountID, id uuid.UUID) (*domain.Job, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	const q = `SELECT ` + jobColumns + ` FROM jobs WHERE id = $1 AND account_id = $2`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, q, id, accountID), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByIdempotencyKey fetches a job by its idempotency key.
func (r *JobRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Job, error) {
	const q = `SELECT ` + jobColumns + ` FROM jobs WHERE idempotency_key = $1`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, q, key), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}

// GetByIdempotencyKeyForAccount fetches a job key only for its exact canonical
// owner; a foreign key is deliberately indistinguishable from a missing key.
func (r *JobRepository) GetByIdempotencyKeyForAccount(ctx context.Context, accountID uuid.UUID, key string) (*domain.Job, error) {
	if accountID == uuid.Nil {
		return nil, domain.ErrNotFound
	}
	const q = `SELECT ` + jobColumns + ` FROM jobs WHERE idempotency_key = $1 AND account_id = $2`
	var job domain.Job
	if err := mapError(scanJob(r.db.QueryRow(ctx, q, key, accountID), &job)); err != nil {
		return nil, err
	}
	return &job, nil
}

// LockAccountForCapacity locks the exact canonical owner row until the caller's
// transaction completes. Activation takes this lock before reading active video
// work, making the read-check-transition sequence strict across API processes.
func (r *JobRepository) LockAccountForCapacity(ctx context.Context, accountID uuid.UUID) error {
	if accountID == uuid.Nil {
		return domain.ErrNotFound
	}
	const q = `SELECT id FROM accounts WHERE id = $1 FOR UPDATE`
	var locked uuid.UUID
	return mapError(r.db.QueryRow(ctx, q, accountID).Scan(&locked))
}

// UpdateStatus applies an explicit state-machine transition. The WHERE clause
// pins the previous status so a concurrent transition cannot be lost; a missing
// match is reported as ErrConflict.
func (r *JobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, from, to domain.JobStatus, errCode, errMessage string) error {
	const q = `
		UPDATE jobs
		SET status = $3, error_code = $4, error_message = $5, updated_at = now()
		WHERE id = $1 AND status = $2`
	tag, err := r.db.Exec(ctx, q, id, from, to, errCode, errMessage)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return nil
}

// Update persists mutable, non-status fields of a job.
func (r *JobRepository) Update(ctx context.Context, job *domain.Job) error {
	if len(job.Params) == 0 {
		job.Params = []byte("{}")
	}
	job.Source = strings.TrimSpace(job.Source)
	if job.Source == "" {
		job.Source = "unknown"
	}
	if job.ResultMode == "" {
		job.ResultMode = domain.ResultModeLegacyUnknown
	}
	channel, recipientRef, threadRef := channelContextValues(job.ChannelContext)
	targetChannel, targetRecipientRef, targetThreadRef := deliveryTargetValues(job.DeliveryTarget)
	const q = `
		UPDATE jobs
		SET source = $2, channel = $3, recipient_ref = $4, thread_ref = $5, result_mode = $6,
		    target_channel = $7, target_recipient_ref = $8, target_thread_ref = $9,
		    provider_id = $10, model_id = $11, priority = $12, correlation_id = $13,
		    input_artifact_ids = $14, output_artifact_ids = $15, params = $16,
		    pricing_snapshot = $17, cost_estimate = $18, cost_reserved = $19, cost_captured = $20,
		    error_code = $21, error_message = $22, expires_at = $23,
		    account_id = COALESCE($24::uuid, account_id, (SELECT account_id FROM users WHERE id = jobs.user_id)),
		    updated_at = now()
		WHERE id = $1
		RETURNING ` + jobColumns
	row := r.db.QueryRow(ctx, q,
		job.ID, job.Source, channel, recipientRef, threadRef, job.ResultMode,
		targetChannel, targetRecipientRef, targetThreadRef,
		job.ProviderID, job.ModelID, job.Priority, job.CorrelationID,
		uuidArray(job.InputArtifactIDs), uuidArray(job.OutputArtifactIDs), []byte(job.Params),
		nullableJSON(job.PricingSnapshot),
		job.CostEstimate, job.CostReserved, job.CostCaptured,
		job.ErrorCode, job.ErrorMessage, job.ExpiresAt, nullableUUID(job.AccountID),
	)
	return mapError(scanJob(row, job))
}

// ListByUser returns the most recent jobs for a user, newest first.
func (r *JobRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	jobs, err := r.listByOwnerColumn(ctx, "account_id", userID, limit, offset)
	if err != nil {
		return nil, err
	}
	if len(jobs) > 0 {
		return jobs, nil
	}
	return r.listByOwnerColumn(ctx, "user_id", userID, limit, offset)
}

// ListByAccount returns jobs only for their exact canonical account owner.
func (r *JobRepository) ListByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	if accountID == uuid.Nil {
		return []*domain.Job{}, nil
	}
	return r.listByOwnerColumn(ctx, "account_id", accountID, limit, offset)
}

func (r *JobRepository) listByOwnerColumn(ctx context.Context, column string, ownerID uuid.UUID, limit, offset int) ([]*domain.Job, error) {
	q := `SELECT ` + jobColumns + `
		FROM jobs WHERE ` + column + ` = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, ownerID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		var job domain.Job
		if err := scanJob(rows, &job); err != nil {
			return nil, mapError(err)
		}
		jobs = append(jobs, &job)
	}
	return jobs, mapError(rows.Err())
}

// List returns jobs matching the filter, newest first. The WHERE clause is
// built dynamically from the non-zero filter fields.
func (r *JobRepository) List(ctx context.Context, filter domain.JobFilter, limit, offset int) ([]*domain.Job, error) {
	q := `SELECT ` + jobColumns + ` FROM jobs`
	var (
		conds []string
		args  []any
	)
	addJobFilterConds(filter, &conds, &args)
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))
	args = append(args, offset)
	q += fmt.Sprintf(" OFFSET $%d", len(args))

	return r.queryJobs(ctx, q, args...)
}

// ListCursor returns jobs matching the filter using keyset pagination.
func (r *JobRepository) ListCursor(ctx context.Context, filter domain.JobFilter, limit int, after *domain.JobCursor) ([]*domain.Job, error) {
	q := `SELECT ` + jobColumns + ` FROM jobs`
	var (
		conds []string
		args  []any
	)
	addJobFilterConds(filter, &conds, &args)
	if after != nil {
		args = append(args, after.CreatedAt)
		createdAtParam := len(args)
		args = append(args, after.ID)
		idParam := len(args)
		conds = append(conds, fmt.Sprintf("(created_at, id) < ($%d, $%d)", createdAtParam, idParam))
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))

	return r.queryJobs(ctx, q, args...)
}

func addJobFilterConds(filter domain.JobFilter, conds *[]string, args *[]any) {
	if filter.UserID != nil {
		*args = append(*args, *filter.UserID)
		*conds = append(*conds, fmt.Sprintf("(user_id = $%d OR account_id = $%d)", len(*args), len(*args)))
	}
	if filter.AccountID != nil {
		*args = append(*args, *filter.AccountID)
		*conds = append(*conds, fmt.Sprintf("account_id = $%d", len(*args)))
	}
	if filter.Source != "" {
		*args = append(*args, filter.Source)
		*conds = append(*conds, fmt.Sprintf("source = $%d", len(*args)))
	}
	if filter.Status != "" {
		*args = append(*args, filter.Status)
		*conds = append(*conds, fmt.Sprintf("status = $%d", len(*args)))
	}
	if filter.Operation != "" {
		*args = append(*args, filter.Operation)
		*conds = append(*conds, fmt.Sprintf("operation_type = $%d", len(*args)))
	}
	if filter.Modality != "" {
		*args = append(*args, filter.Modality)
		*conds = append(*conds, fmt.Sprintf("modality = $%d", len(*args)))
	}
	if filter.ErrorCode != "" {
		*args = append(*args, filter.ErrorCode)
		*conds = append(*conds, fmt.Sprintf("error_code = $%d", len(*args)))
	}
	if filter.Provider != "" {
		*args = append(*args, filter.Provider)
		*conds = append(*conds, fmt.Sprintf("EXISTS (SELECT 1 FROM provider_tasks pt WHERE pt.job_id = jobs.id AND pt.provider = $%d)", len(*args)))
	}
	if filter.CorrelationID != "" {
		*args = append(*args, filter.CorrelationID)
		*conds = append(*conds, fmt.Sprintf("correlation_id = $%d", len(*args)))
	}
	if filter.CreatedFrom != nil {
		*args = append(*args, *filter.CreatedFrom)
		*conds = append(*conds, fmt.Sprintf("created_at >= $%d", len(*args)))
	}
	if filter.CreatedTo != nil {
		*args = append(*args, *filter.CreatedTo)
		*conds = append(*conds, fmt.Sprintf("created_at < $%d", len(*args)))
	}
}

func (r *JobRepository) queryJobs(ctx context.Context, q string, args ...any) ([]*domain.Job, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		var job domain.Job
		if err := scanJob(rows, &job); err != nil {
			return nil, mapError(err)
		}
		jobs = append(jobs, &job)
	}
	return jobs, mapError(rows.Err())
}

func (r *JobRepository) CountActiveByUserOperation(ctx context.Context, userID uuid.UUID, operation domain.OperationType) (int, error) {
	statuses := domain.ActiveWorkJobStatuses()
	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusValues = append(statusValues, string(status))
	}
	count, err := r.countActiveByOwnerColumn(ctx, "account_id", userID, operation, statusValues)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return count, nil
	}
	return r.countActiveByOwnerColumn(ctx, "user_id", userID, operation, statusValues)
}

// CountActiveByAccountOperation counts active jobs for one exact canonical
// account. Account-native flows must not use the legacy user-id fallback.
func (r *JobRepository) CountActiveByAccountOperation(ctx context.Context, accountID uuid.UUID, operation domain.OperationType) (int, error) {
	statuses := domain.ActiveWorkJobStatuses()
	statusValues := make([]string, 0, len(statuses))
	for _, status := range statuses {
		statusValues = append(statusValues, string(status))
	}
	return r.countActiveByOwnerColumn(ctx, "account_id", accountID, operation, statusValues)
}

// CountUnexpiredPreparedByAccountOperation counts exact account-owned prepared
// jobs that remain activatable for a trusted source and product operation.
// Rows created before confirmation expiries existed retain their conservative
// zero-expiry behaviour and still count until they leave prepared.
func (r *JobRepository) CountUnexpiredPreparedByAccountOperation(ctx context.Context, accountID uuid.UUID, source string, operation domain.OperationType, modality domain.Modality, now time.Time) (int, error) {
	if accountID == uuid.Nil {
		return 0, nil
	}
	const q = `
		SELECT count(*)
		FROM jobs
		WHERE account_id = $1
		  AND source = $2
		  AND operation_type = $3
		  AND modality = $4
		  AND status = $5
		  AND (expires_at IS NULL OR expires_at > $6)`
	var count int
	if err := r.db.QueryRow(ctx, q, accountID, strings.TrimSpace(source), operation, modality, domain.JobStatusPrepared, now).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

func (r *JobRepository) countActiveByOwnerColumn(ctx context.Context, column string, ownerID uuid.UUID, operation domain.OperationType, statusValues []string) (int, error) {
	q := `
		SELECT count(*)
		FROM jobs
		WHERE ` + column + ` = $1
		  AND operation_type = $2
		  AND status = ANY($3::text[])`
	var count int
	if err := r.db.QueryRow(ctx, q, ownerID, operation, statusValues).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

// CountSucceededByUser returns completed successful jobs for one user.
func (r *JobRepository) CountSucceededByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	count, err := r.countSucceededByOwnerColumn(ctx, "account_id", userID)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		return count, nil
	}
	return r.countSucceededByOwnerColumn(ctx, "user_id", userID)
}

func (r *JobRepository) countSucceededByOwnerColumn(ctx context.Context, column string, ownerID uuid.UUID) (int, error) {
	q := `
		SELECT count(*)
		FROM jobs
		WHERE ` + column + ` = $1 AND status = $2`
	var count int
	if err := r.db.QueryRow(ctx, q, ownerID, domain.JobStatusSucceeded).Scan(&count); err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

func scanJob(row rowScanner, job *domain.Job) error {
	var legacyUserID *uuid.UUID
	var legacyPeerID *int64
	var commandID *uuid.UUID
	var accountID *uuid.UUID
	var channel, recipientRef, threadRef *string
	var targetChannel, targetRecipientRef, targetThreadRef *string
	var pricingSnapshot []byte
	if err := row.Scan(
		&job.ID, &legacyUserID, &accountID, &job.Source, &channel, &recipientRef, &threadRef, &job.ResultMode,
		&targetChannel, &targetRecipientRef, &targetThreadRef, &legacyPeerID, &commandID, &job.OperationType, &job.Modality,
		&job.ProviderID, &job.ModelID, &job.Status, &job.Priority, &job.IdempotencyKey, &job.CorrelationID,
		&job.InputArtifactIDs, &job.OutputArtifactIDs, &job.Params, &pricingSnapshot, &job.CostEstimate, &job.CostReserved,
		&job.CostCaptured, &job.ErrorCode, &job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &job.ExpiresAt,
	); err != nil {
		return err
	}
	job.AccountID = uuid.Nil
	if accountID != nil {
		job.AccountID = *accountID
	}
	job.UserID = uuid.Nil
	if legacyUserID != nil {
		job.UserID = *legacyUserID
	}
	job.VKPeerID = 0
	if legacyPeerID != nil {
		job.VKPeerID = *legacyPeerID
	}
	if commandID != nil {
		job.CommandID = *commandID
	} else {
		job.CommandID = uuid.Nil
	}
	job.ChannelContext = channelContextFromColumns(channel, recipientRef, threadRef)
	job.DeliveryTarget = deliveryTargetFromColumns(targetChannel, targetRecipientRef, targetThreadRef)
	if len(pricingSnapshot) > 0 {
		job.PricingSnapshot = append(job.PricingSnapshot[:0], pricingSnapshot...)
	} else {
		job.PricingSnapshot = nil
	}
	return nil
}

func channelContextValues(context *domain.ChannelContext) (*string, *string, *string) {
	if context == nil {
		return nil, nil, nil
	}
	return nullableString(string(context.Channel)), nullableString(context.RecipientRef), nullableString(context.ThreadRef)
}

func deliveryTargetValues(target *domain.DeliveryTarget) (*string, *string, *string) {
	if target == nil {
		return nil, nil, nil
	}
	return nullableString(string(target.Channel)), nullableString(target.RecipientRef), nullableString(target.ThreadRef)
}

func channelContextFromColumns(channel, recipientRef, threadRef *string) *domain.ChannelContext {
	if channel == nil && recipientRef == nil && threadRef == nil {
		return nil
	}
	context := &domain.ChannelContext{}
	if channel != nil {
		context.Channel = domain.Channel(*channel)
	}
	if recipientRef != nil {
		context.RecipientRef = *recipientRef
	}
	if threadRef != nil {
		context.ThreadRef = *threadRef
	}
	return context
}

func deliveryTargetFromColumns(channel, recipientRef, threadRef *string) *domain.DeliveryTarget {
	if channel == nil && recipientRef == nil && threadRef == nil {
		return nil
	}
	target := &domain.DeliveryTarget{}
	if channel != nil {
		target.Channel = domain.Channel(*channel)
	}
	if recipientRef != nil {
		target.RecipientRef = *recipientRef
	}
	if threadRef != nil {
		target.ThreadRef = *threadRef
	}
	return target
}

func nullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}
