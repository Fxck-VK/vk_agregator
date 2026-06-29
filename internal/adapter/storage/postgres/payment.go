package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// PaymentRepository is the PostgreSQL implementation of domain.PaymentRepository.
type PaymentRepository struct {
	db Querier
}

// NewPaymentRepository builds a PaymentRepository over the given querier.
func NewPaymentRepository(db Querier) *PaymentRepository {
	return &PaymentRepository{db: db}
}

var _ domain.PaymentRepository = (*PaymentRepository)(nil)

const paymentProductColumns = `id, code, title, amount, currency, credits,
	price_version, vat_code, payment_subject, payment_mode, is_active,
	created_at, updated_at`

const paymentIntentColumns = `id, user_id, product_id, status, amount, currency,
	credits, price_version, receipt_description, vat_code, payment_subject,
	payment_mode, provider, provider_payment_id, confirmation_url,
	idempotency_key, receipt_email, receipt_phone, metadata, created_at,
	updated_at, expires_at`

const paymentEventColumns = `id, provider, event_type, provider_payment_id,
	provider_refund_id, dedup_key, payload, processed_at, received_at,
	updated_at`

const paymentRefundColumns = `id, intent_id, provider_refund_id, amount, status,
	idempotency_key, reason, created_at, updated_at`

// ListActiveProducts lists active product catalog entries by amount.
func (r *PaymentRepository) ListActiveProducts(ctx context.Context) ([]*domain.PaymentProduct, error) {
	const q = `SELECT ` + paymentProductColumns + `
		FROM payment_products
		WHERE is_active = true
		ORDER BY amount ASC, code ASC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	products := []*domain.PaymentProduct{}
	for rows.Next() {
		var product domain.PaymentProduct
		if err := scanPaymentProduct(rows, &product); err != nil {
			return nil, mapError(err)
		}
		products = append(products, &product)
	}
	return products, mapError(rows.Err())
}

// ListProducts lists product catalog entries for protected operator endpoints.
func (r *PaymentRepository) ListProducts(ctx context.Context, filter domain.PaymentProductFilter, limit, offset int) ([]*domain.PaymentProduct, error) {
	args := []any{}
	where := []string{"1=1"}
	if filter.Active != nil {
		args = append(args, *filter.Active)
		where = append(where, "is_active = $"+strconv.Itoa(len(args)))
	}
	args = append(args, limit, offset)
	query := `SELECT ` + paymentProductColumns + `
		FROM payment_products
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC, code ASC
		LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanPaymentProducts(rows)
}

// GetActiveProductByCode fetches an active product by code.
func (r *PaymentRepository) GetActiveProductByCode(ctx context.Context, code string) (*domain.PaymentProduct, error) {
	const q = `SELECT ` + paymentProductColumns + `
		FROM payment_products
		WHERE code = $1 AND is_active = true`
	var product domain.PaymentProduct
	if err := mapError(scanPaymentProduct(r.db.QueryRow(ctx, q, strings.TrimSpace(code)), &product)); err != nil {
		return nil, err
	}
	return &product, nil
}

// GetProductByID fetches a product by id.
func (r *PaymentRepository) GetProductByID(ctx context.Context, id uuid.UUID) (*domain.PaymentProduct, error) {
	const q = `SELECT ` + paymentProductColumns + ` FROM payment_products WHERE id = $1`
	var product domain.PaymentProduct
	if err := mapError(scanPaymentProduct(r.db.QueryRow(ctx, q, id), &product)); err != nil {
		return nil, err
	}
	return &product, nil
}

// CreateProduct inserts one product catalog entry.
func (r *PaymentRepository) CreateProduct(ctx context.Context, product *domain.PaymentProduct) error {
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	if product.PriceVersion == 0 {
		product.PriceVersion = 1
	}
	const q = `
		INSERT INTO payment_products (
			id, code, title, amount, currency, credits, price_version,
			vat_code, payment_subject, payment_mode, is_active
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING ` + paymentProductColumns
	return mapError(scanPaymentProduct(r.db.QueryRow(ctx, q,
		product.ID,
		strings.TrimSpace(product.Code),
		strings.TrimSpace(product.Title),
		product.Amount,
		product.Currency,
		product.Credits,
		product.PriceVersion,
		product.VATCode,
		strings.TrimSpace(product.PaymentSubject),
		strings.TrimSpace(product.PaymentMode),
		product.IsActive,
	), product))
}

// UpdateProduct persists an existing product catalog entry.
func (r *PaymentRepository) UpdateProduct(ctx context.Context, product *domain.PaymentProduct) error {
	const q = `
		UPDATE payment_products
		SET code = $2,
		    title = $3,
		    amount = $4,
		    currency = $5,
		    credits = $6,
		    price_version = $7,
		    vat_code = $8,
		    payment_subject = $9,
		    payment_mode = $10,
		    is_active = $11,
		    updated_at = now()
		WHERE id = $1
		RETURNING ` + paymentProductColumns
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	return mapError(scanPaymentProduct(r.db.QueryRow(ctx, q,
		product.ID,
		strings.TrimSpace(product.Code),
		strings.TrimSpace(product.Title),
		product.Amount,
		product.Currency,
		product.Credits,
		product.PriceVersion,
		product.VATCode,
		strings.TrimSpace(product.PaymentSubject),
		strings.TrimSpace(product.PaymentMode),
		product.IsActive,
	), product))
}

// CreateIntent inserts a new local payment intent.
func (r *PaymentRepository) CreateIntent(ctx context.Context, intent *domain.PaymentIntent) error {
	if intent.ID == uuid.Nil {
		intent.ID = uuid.New()
	}
	if intent.Status == "" {
		intent.Status = domain.PaymentIntentCreated
	}
	if intent.Currency == "" {
		intent.Currency = domain.CurrencyRUB
	}
	if len(intent.Metadata) == 0 {
		intent.Metadata = json.RawMessage(`{}`)
	}
	const q = `
		INSERT INTO payment_intents (
			id, user_id, product_id, status, amount, currency, credits,
			price_version, receipt_description, vat_code, payment_subject,
			payment_mode, provider, provider_payment_id, confirmation_url,
			idempotency_key, receipt_email, receipt_phone, metadata, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, COALESCE($19::jsonb, '{}'::jsonb), $20)
		RETURNING ` + paymentIntentColumns
	return mapError(scanPaymentIntent(r.db.QueryRow(ctx, q,
		intent.ID,
		intent.UserID,
		intent.ProductID,
		intent.Status,
		intent.Amount,
		intent.Currency,
		intent.Credits,
		intent.PriceVersion,
		strings.TrimSpace(intent.ReceiptDescription),
		intent.VATCode,
		strings.TrimSpace(intent.PaymentSubject),
		strings.TrimSpace(intent.PaymentMode),
		intent.Provider,
		nullEmptyString(intent.ProviderPaymentID),
		intent.ConfirmationURL,
		intent.IdempotencyKey,
		nullEmptyString(intent.ReceiptEmail),
		nullEmptyString(intent.ReceiptPhone),
		rawOrNil(intent.Metadata),
		intent.ExpiresAt,
	), intent))
}

// GetIntentByID fetches one intent by id.
func (r *PaymentRepository) GetIntentByID(ctx context.Context, id uuid.UUID) (*domain.PaymentIntent, error) {
	const q = `SELECT ` + paymentIntentColumns + ` FROM payment_intents WHERE id = $1`
	var intent domain.PaymentIntent
	if err := mapError(scanPaymentIntent(r.db.QueryRow(ctx, q, id), &intent)); err != nil {
		return nil, err
	}
	return &intent, nil
}

// GetIntentByIdempotencyKey fetches one intent by idempotency key.
func (r *PaymentRepository) GetIntentByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentIntent, error) {
	const q = `SELECT ` + paymentIntentColumns + ` FROM payment_intents WHERE idempotency_key = $1`
	var intent domain.PaymentIntent
	if err := mapError(scanPaymentIntent(r.db.QueryRow(ctx, q, strings.TrimSpace(key)), &intent)); err != nil {
		return nil, err
	}
	return &intent, nil
}

// SetIntentProviderState stores provider payment fields after CreatePayment.
func (r *PaymentRepository) SetIntentProviderState(ctx context.Context, id uuid.UUID, status domain.PaymentIntentStatus, providerPaymentID, confirmationURL string) error {
	const q = `
		UPDATE payment_intents
		SET status = $2,
		    provider_payment_id = $3,
		    confirmation_url = $4,
		    updated_at = now()
		WHERE id = $1
		RETURNING ` + paymentIntentColumns
	var intent domain.PaymentIntent
	return mapError(scanPaymentIntent(r.db.QueryRow(ctx, q,
		id,
		status,
		nullEmptyString(providerPaymentID),
		confirmationURL,
	), &intent))
}

// GetIntentByProviderPaymentID fetches one intent by provider payment id.
func (r *PaymentRepository) GetIntentByProviderPaymentID(ctx context.Context, provider domain.PaymentProviderCode, providerPaymentID string) (*domain.PaymentIntent, error) {
	const q = `SELECT ` + paymentIntentColumns + `
		FROM payment_intents
		WHERE provider = $1 AND provider_payment_id = $2`
	var intent domain.PaymentIntent
	if err := mapError(scanPaymentIntent(r.db.QueryRow(ctx, q, provider, strings.TrimSpace(providerPaymentID)), &intent)); err != nil {
		return nil, err
	}
	return &intent, nil
}

// UpdateIntentStatus updates an intent status using optimistic state matching.
func (r *PaymentRepository) UpdateIntentStatus(ctx context.Context, id uuid.UUID, from, to domain.PaymentIntentStatus) error {
	const q = `
		UPDATE payment_intents
		SET status = $3,
		    updated_at = now()
		WHERE id = $1 AND status = $2
		RETURNING ` + paymentIntentColumns
	var intent domain.PaymentIntent
	if err := mapError(scanPaymentIntent(r.db.QueryRow(ctx, q, id, from, to), &intent)); err != nil {
		if err == domain.ErrNotFound {
			return domain.ErrConflict
		}
		return err
	}
	return nil
}

// UpdateIntentMetadata replaces one intent metadata document.
func (r *PaymentRepository) UpdateIntentMetadata(ctx context.Context, id uuid.UUID, metadata json.RawMessage) error {
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	const q = `
		UPDATE payment_intents
		SET metadata = COALESCE($2::jsonb, '{}'::jsonb),
		    updated_at = now()
		WHERE id = $1
		RETURNING ` + paymentIntentColumns
	var intent domain.PaymentIntent
	return mapError(scanPaymentIntent(r.db.QueryRow(ctx, q, id, rawOrNil(metadata)), &intent))
}

// ListIntentsByUser lists intents for one user.
func (r *PaymentRepository) ListIntentsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.PaymentIntent, error) {
	const q = `SELECT ` + paymentIntentColumns + `
		FROM payment_intents
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanPaymentIntents(rows)
}

// ListIntents lists intents for protected operator endpoints.
func (r *PaymentRepository) ListIntents(ctx context.Context, filter domain.PaymentIntentFilter, limit, offset int) ([]*domain.PaymentIntent, error) {
	args := []any{}
	where := []string{"1=1"}
	if filter.IntentID != nil {
		args = append(args, *filter.IntentID)
		where = append(where, "id = $"+strconv.Itoa(len(args)))
	}
	if filter.UserID != nil {
		args = append(args, *filter.UserID)
		where = append(where, "user_id = $"+strconv.Itoa(len(args)))
	}
	statuses := paymentIntentStatuses(filter)
	if len(statuses) > 0 {
		args = append(args, statuses)
		where = append(where, "status = ANY($"+strconv.Itoa(len(args))+")")
	}
	if filter.Provider != "" {
		args = append(args, filter.Provider)
		where = append(where, "provider = $"+strconv.Itoa(len(args)))
	}
	if providerPaymentID := strings.TrimSpace(filter.ProviderPaymentID); providerPaymentID != "" {
		args = append(args, providerPaymentID)
		where = append(where, "provider_payment_id = $"+strconv.Itoa(len(args)))
	}
	if source := strings.TrimSpace(filter.Source); source != "" {
		args = append(args, source)
		where = append(where, "metadata->>'source' = $"+strconv.Itoa(len(args)))
	}
	if filter.UpdatedBefore != nil && !filter.UpdatedBefore.IsZero() {
		args = append(args, *filter.UpdatedBefore)
		where = append(where, "updated_at <= $"+strconv.Itoa(len(args)))
	}
	args = append(args, limit, offset)
	query := `SELECT ` + paymentIntentColumns + `
		FROM payment_intents
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC
		LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanPaymentIntents(rows)
}

func paymentIntentStatuses(filter domain.PaymentIntentFilter) []string {
	seen := map[string]bool{}
	statuses := []string{}
	if filter.Status != "" {
		value := string(filter.Status)
		seen[value] = true
		statuses = append(statuses, value)
	}
	for _, status := range filter.Statuses {
		if status == "" {
			continue
		}
		value := string(status)
		if seen[value] {
			continue
		}
		seen[value] = true
		statuses = append(statuses, value)
	}
	return statuses
}

// ListIntentsForReconciliation lists stale provider-backed intents.
func (r *PaymentRepository) ListIntentsForReconciliation(ctx context.Context, filter domain.PaymentReconciliationFilter, limit int) ([]*domain.PaymentIntent, error) {
	statuses := make([]string, 0, len(filter.Statuses))
	for _, status := range filter.Statuses {
		statuses = append(statuses, string(status))
	}
	if len(statuses) == 0 {
		statuses = []string{string(domain.PaymentIntentProviderPending), string(domain.PaymentIntentWaitingForUser)}
	}
	updatedBefore := filter.UpdatedBefore
	if updatedBefore.IsZero() {
		updatedBefore = time.Now()
	}
	const q = `SELECT ` + paymentIntentColumns + `
		FROM payment_intents
		WHERE provider = $1
		  AND status = ANY($2)
		  AND provider_payment_id IS NOT NULL
		  AND btrim(provider_payment_id) <> ''
		  AND updated_at <= $3
		ORDER BY updated_at ASC
		LIMIT $4`
	rows, err := r.db.Query(ctx, q, filter.Provider, statuses, updatedBefore, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanPaymentIntents(rows)
}

// CreateEvent inserts a raw provider webhook event into the inbox.
func (r *PaymentRepository) CreateEvent(ctx context.Context, event *domain.PaymentEvent) (bool, error) {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	const q = `
		INSERT INTO payment_events (
			id, provider, event_type, provider_payment_id, provider_refund_id,
			dedup_key, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7::jsonb, '{}'::jsonb))
		ON CONFLICT (dedup_key) DO NOTHING
		RETURNING ` + paymentEventColumns
	err := mapError(scanPaymentEvent(r.db.QueryRow(ctx, q,
		event.ID,
		event.Provider,
		event.EventType,
		nullEmptyString(event.ProviderPaymentID),
		nullEmptyString(event.ProviderRefundID),
		strings.TrimSpace(event.DedupKey),
		rawOrNil(event.Payload),
	), event))
	if err != nil {
		if err == domain.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetEventByID fetches one provider webhook event.
func (r *PaymentRepository) GetEventByID(ctx context.Context, id uuid.UUID) (*domain.PaymentEvent, error) {
	const q = `SELECT ` + paymentEventColumns + ` FROM payment_events WHERE id = $1`
	var event domain.PaymentEvent
	if err := mapError(scanPaymentEvent(r.db.QueryRow(ctx, q, id), &event)); err != nil {
		return nil, err
	}
	return &event, nil
}

// ListUnprocessedEvents lists provider webhook events waiting for processing.
func (r *PaymentRepository) ListUnprocessedEvents(ctx context.Context, provider domain.PaymentProviderCode, limit int) ([]*domain.PaymentEvent, error) {
	const q = `SELECT ` + paymentEventColumns + `
		FROM payment_events
		WHERE provider = $1 AND processed_at IS NULL
		ORDER BY received_at ASC
		LIMIT $2`
	rows, err := r.db.Query(ctx, q, provider, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanPaymentEvents(rows)
}

func (r *PaymentRepository) ListEvents(ctx context.Context, filter domain.PaymentEventFilter, limit, offset int) ([]*domain.PaymentEvent, error) {
	args := []any{}
	where := []string{"1=1"}
	if filter.Provider != "" {
		args = append(args, filter.Provider)
		where = append(where, "provider = $"+strconv.Itoa(len(args)))
	}
	if providerPaymentID := strings.TrimSpace(filter.ProviderPaymentID); providerPaymentID != "" {
		args = append(args, providerPaymentID)
		where = append(where, "provider_payment_id = $"+strconv.Itoa(len(args)))
	}
	if filter.Processed != nil {
		if *filter.Processed {
			where = append(where, "processed_at IS NOT NULL")
		} else {
			where = append(where, "processed_at IS NULL")
		}
	}
	args = append(args, limit, offset)
	query := `SELECT ` + paymentEventColumns + `
		FROM payment_events
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY received_at ASC
		LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	return scanPaymentEvents(rows)
}

// WebhookInboxStats returns current unprocessed provider webhook backlog.
func (r *PaymentRepository) WebhookInboxStats(ctx context.Context, provider domain.PaymentProviderCode) (domain.PaymentWebhookInboxStats, error) {
	const q = `
		SELECT count(*)::bigint, min(received_at)
		FROM payment_events
		WHERE provider = $1 AND processed_at IS NULL`
	var count int64
	var oldest sql.NullTime
	if err := r.db.QueryRow(ctx, q, provider).Scan(&count, &oldest); err != nil {
		return domain.PaymentWebhookInboxStats{}, mapError(err)
	}
	stats := domain.PaymentWebhookInboxStats{
		Provider:          provider,
		UnprocessedEvents: count,
	}
	if oldest.Valid {
		t := oldest.Time
		stats.OldestUnprocessedReceivedAt = &t
	}
	return stats, nil
}

// MarkEventProcessed marks a provider webhook event as processed.
func (r *PaymentRepository) MarkEventProcessed(ctx context.Context, id uuid.UUID, processedAt time.Time) error {
	const q = `
		UPDATE payment_events
		SET processed_at = COALESCE($2::timestamptz, now()),
		    updated_at = now()
		WHERE id = $1 AND processed_at IS NULL`
	_, err := r.db.Exec(ctx, q, id, nullableTime(processedAt))
	return mapError(err)
}

// CreateRefund inserts a local refund row.
func (r *PaymentRepository) CreateRefund(ctx context.Context, refund *domain.PaymentRefund) error {
	if refund.ID == uuid.Nil {
		refund.ID = uuid.New()
	}
	if refund.Status == "" {
		refund.Status = domain.PaymentRefundCreated
	}
	const q = `
		INSERT INTO payment_refunds (
			id, intent_id, provider_refund_id, amount, status, idempotency_key, reason
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + paymentRefundColumns
	return mapError(scanPaymentRefund(r.db.QueryRow(ctx, q,
		refund.ID,
		refund.IntentID,
		nullEmptyString(refund.ProviderRefundID),
		refund.Amount,
		refund.Status,
		strings.TrimSpace(refund.IdempotencyKey),
		strings.TrimSpace(refund.Reason),
	), refund))
}

// GetRefundByIdempotencyKey fetches one refund by internal idempotency key.
func (r *PaymentRepository) GetRefundByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentRefund, error) {
	const q = `SELECT ` + paymentRefundColumns + `
		FROM payment_refunds
		WHERE idempotency_key = $1`
	var refund domain.PaymentRefund
	if err := mapError(scanPaymentRefund(r.db.QueryRow(ctx, q, strings.TrimSpace(key)), &refund)); err != nil {
		return nil, err
	}
	return &refund, nil
}

// ListRefunds lists protected operator refund rows.
func (r *PaymentRepository) ListRefunds(ctx context.Context, filter domain.PaymentRefundFilter, limit, offset int) ([]*domain.PaymentRefund, error) {
	args := []any{}
	where := []string{"1=1"}
	if filter.IntentID != nil {
		args = append(args, *filter.IntentID)
		where = append(where, "intent_id = $"+strconv.Itoa(len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, "status = $"+strconv.Itoa(len(args)))
	}
	args = append(args, limit, offset)
	query := `SELECT ` + paymentRefundColumns + `
		FROM payment_refunds
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY created_at DESC
		LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	var refunds []*domain.PaymentRefund
	for rows.Next() {
		var refund domain.PaymentRefund
		if err := mapError(scanPaymentRefund(rows, &refund)); err != nil {
			return nil, err
		}
		refunds = append(refunds, &refund)
	}
	return refunds, mapError(rows.Err())
}

// SetRefundProviderState stores provider refund fields.
func (r *PaymentRepository) SetRefundProviderState(ctx context.Context, id uuid.UUID, providerRefundID string, status domain.PaymentRefundStatus) error {
	const q = `
		UPDATE payment_refunds
		SET provider_refund_id = $2,
		    status = $3,
		    updated_at = now()
		WHERE id = $1
		RETURNING ` + paymentRefundColumns
	var refund domain.PaymentRefund
	return mapError(scanPaymentRefund(r.db.QueryRow(ctx, q, id, nullEmptyString(providerRefundID), status), &refund))
}

func scanPaymentProduct(row rowScanner, product *domain.PaymentProduct) error {
	return row.Scan(
		&product.ID,
		&product.Code,
		&product.Title,
		&product.Amount,
		&product.Currency,
		&product.Credits,
		&product.PriceVersion,
		&product.VATCode,
		&product.PaymentSubject,
		&product.PaymentMode,
		&product.IsActive,
		&product.CreatedAt,
		&product.UpdatedAt,
	)
}

func scanPaymentIntent(row rowScanner, intent *domain.PaymentIntent) error {
	var productID *uuid.UUID
	var providerPaymentID, receiptEmail, receiptPhone *string
	var metadata []byte
	if err := row.Scan(
		&intent.ID,
		&intent.UserID,
		&productID,
		&intent.Status,
		&intent.Amount,
		&intent.Currency,
		&intent.Credits,
		&intent.PriceVersion,
		&intent.ReceiptDescription,
		&intent.VATCode,
		&intent.PaymentSubject,
		&intent.PaymentMode,
		&intent.Provider,
		&providerPaymentID,
		&intent.ConfirmationURL,
		&intent.IdempotencyKey,
		&receiptEmail,
		&receiptPhone,
		&metadata,
		&intent.CreatedAt,
		&intent.UpdatedAt,
		&intent.ExpiresAt,
	); err != nil {
		return err
	}
	intent.ProductID = productID
	if providerPaymentID != nil {
		intent.ProviderPaymentID = *providerPaymentID
	}
	if receiptEmail != nil {
		intent.ReceiptEmail = *receiptEmail
	}
	if receiptPhone != nil {
		intent.ReceiptPhone = *receiptPhone
	}
	if len(metadata) == 0 {
		intent.Metadata = json.RawMessage(`{}`)
	} else {
		intent.Metadata = append(json.RawMessage(nil), metadata...)
	}
	return nil
}

func scanPaymentRefund(row rowScanner, refund *domain.PaymentRefund) error {
	var providerRefundID *string
	if err := row.Scan(
		&refund.ID,
		&refund.IntentID,
		&providerRefundID,
		&refund.Amount,
		&refund.Status,
		&refund.IdempotencyKey,
		&refund.Reason,
		&refund.CreatedAt,
		&refund.UpdatedAt,
	); err != nil {
		return err
	}
	if providerRefundID != nil {
		refund.ProviderRefundID = *providerRefundID
	}
	return nil
}

func scanPaymentEvent(row rowScanner, event *domain.PaymentEvent) error {
	var providerPaymentID, providerRefundID *string
	var payload []byte
	if err := row.Scan(
		&event.ID,
		&event.Provider,
		&event.EventType,
		&providerPaymentID,
		&providerRefundID,
		&event.DedupKey,
		&payload,
		&event.ProcessedAt,
		&event.ReceivedAt,
		&event.UpdatedAt,
	); err != nil {
		return err
	}
	if providerPaymentID != nil {
		event.ProviderPaymentID = *providerPaymentID
	}
	if providerRefundID != nil {
		event.ProviderRefundID = *providerRefundID
	}
	if len(payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	} else {
		event.Payload = append(json.RawMessage(nil), payload...)
	}
	return nil
}

func scanPaymentEvents(rows rowScannerRows) ([]*domain.PaymentEvent, error) {
	var events []*domain.PaymentEvent
	for rows.Next() {
		var event domain.PaymentEvent
		if err := scanPaymentEvent(rows, &event); err != nil {
			return nil, mapError(err)
		}
		events = append(events, &event)
	}
	return events, mapError(rows.Err())
}

func scanPaymentIntents(rows rowScannerRows) ([]*domain.PaymentIntent, error) {
	var intents []*domain.PaymentIntent
	for rows.Next() {
		var intent domain.PaymentIntent
		if err := scanPaymentIntent(rows, &intent); err != nil {
			return nil, mapError(err)
		}
		intents = append(intents, &intent)
	}
	return intents, mapError(rows.Err())
}

func scanPaymentProducts(rows rowScannerRows) ([]*domain.PaymentProduct, error) {
	var products []*domain.PaymentProduct
	for rows.Next() {
		var product domain.PaymentProduct
		if err := scanPaymentProduct(rows, &product); err != nil {
			return nil, mapError(err)
		}
		products = append(products, &product)
	}
	return products, mapError(rows.Err())
}

func nullEmptyString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
