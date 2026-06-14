package memory

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// PaymentRepo is an in-memory domain.PaymentRepository.
type PaymentRepo struct {
	mu                 sync.Mutex
	productsByID       map[uuid.UUID]domain.PaymentProduct
	productIDByCode    map[string]uuid.UUID
	intentsByID        map[uuid.UUID]domain.PaymentIntent
	intentIDByKey      map[string]uuid.UUID
	intentIDByProvider map[string]uuid.UUID
	intentIDsByUser    map[uuid.UUID][]uuid.UUID
	eventsByID         map[uuid.UUID]domain.PaymentEvent
	eventIDByDedup     map[string]uuid.UUID
	eventIDs           []uuid.UUID
	refundsByID        map[uuid.UUID]domain.PaymentRefund
	refundIDByKey      map[string]uuid.UUID
	refundIDByProvider map[string]uuid.UUID
}

// NewPaymentRepo builds an empty PaymentRepo.
func NewPaymentRepo() *PaymentRepo {
	return &PaymentRepo{
		productsByID:       map[uuid.UUID]domain.PaymentProduct{},
		productIDByCode:    map[string]uuid.UUID{},
		intentsByID:        map[uuid.UUID]domain.PaymentIntent{},
		intentIDByKey:      map[string]uuid.UUID{},
		intentIDByProvider: map[string]uuid.UUID{},
		intentIDsByUser:    map[uuid.UUID][]uuid.UUID{},
		eventsByID:         map[uuid.UUID]domain.PaymentEvent{},
		eventIDByDedup:     map[string]uuid.UUID{},
		refundsByID:        map[uuid.UUID]domain.PaymentRefund{},
		refundIDByKey:      map[string]uuid.UUID{},
		refundIDByProvider: map[string]uuid.UUID{},
	}
}

var _ domain.PaymentRepository = (*PaymentRepo)(nil)

// PutProduct inserts or replaces a test/local product catalog entry.
func (r *PaymentRepo) PutProduct(product *domain.PaymentProduct) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	product.Code = strings.TrimSpace(product.Code)
	product.Title = strings.TrimSpace(product.Title)
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	if product.PriceVersion == 0 {
		product.PriceVersion = 1
	}
	now := time.Now()
	if product.CreatedAt.IsZero() {
		product.CreatedAt = now
	}
	product.UpdatedAt = now
	r.productsByID[product.ID] = copyPaymentProduct(product)
	r.productIDByCode[product.Code] = product.ID
}

func (r *PaymentRepo) ListActiveProducts(_ context.Context) ([]*domain.PaymentProduct, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	products := make([]domain.PaymentProduct, 0, len(r.productsByID))
	for _, product := range r.productsByID {
		if product.IsActive {
			products = append(products, product)
		}
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].Amount != products[j].Amount {
			return products[i].Amount < products[j].Amount
		}
		return products[i].Code < products[j].Code
	})
	out := make([]*domain.PaymentProduct, 0, len(products))
	for i := range products {
		product := products[i]
		out = append(out, copyPaymentProductPtr(product))
	}
	return out, nil
}

func (r *PaymentRepo) ListProducts(_ context.Context, filter domain.PaymentProductFilter, limit, offset int) ([]*domain.PaymentProduct, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	products := make([]domain.PaymentProduct, 0, len(r.productsByID))
	for _, product := range r.productsByID {
		if filter.Active != nil && product.IsActive != *filter.Active {
			continue
		}
		products = append(products, copyPaymentProduct(&product))
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].CreatedAt.Equal(products[j].CreatedAt) {
			return products[i].Code < products[j].Code
		}
		return products[i].CreatedAt.After(products[j].CreatedAt)
	})
	out := make([]*domain.PaymentProduct, 0, len(products))
	for i := offset; i < len(products) && len(out) < limit; i++ {
		product := products[i]
		out = append(out, copyPaymentProductPtr(product))
	}
	return out, nil
}

func (r *PaymentRepo) GetActiveProductByCode(_ context.Context, code string) (*domain.PaymentProduct, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.productIDByCode[strings.TrimSpace(code)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	product := r.productsByID[id]
	if !product.IsActive {
		return nil, domain.ErrNotFound
	}
	return copyPaymentProductPtr(product), nil
}

func (r *PaymentRepo) GetProductByID(_ context.Context, id uuid.UUID) (*domain.PaymentProduct, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	product, ok := r.productsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return copyPaymentProductPtr(product), nil
}

func (r *PaymentRepo) CreateProduct(_ context.Context, product *domain.PaymentProduct) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	product.Code = strings.TrimSpace(product.Code)
	product.Title = strings.TrimSpace(product.Title)
	if _, ok := r.productIDByCode[product.Code]; ok {
		return domain.ErrConflict
	}
	if product.ID == uuid.Nil {
		product.ID = uuid.New()
	}
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	if product.PriceVersion == 0 {
		product.PriceVersion = 1
	}
	now := time.Now()
	product.CreatedAt, product.UpdatedAt = now, now
	r.productsByID[product.ID] = copyPaymentProduct(product)
	r.productIDByCode[product.Code] = product.ID
	return nil
}

func (r *PaymentRepo) UpdateProduct(_ context.Context, product *domain.PaymentProduct) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.productsByID[product.ID]
	if !ok {
		return domain.ErrNotFound
	}
	product.Code = strings.TrimSpace(product.Code)
	product.Title = strings.TrimSpace(product.Title)
	if product.Code == "" {
		product.Code = existing.Code
	}
	if existingID, ok := r.productIDByCode[product.Code]; ok && existingID != product.ID {
		return domain.ErrConflict
	}
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	product.CreatedAt = existing.CreatedAt
	product.UpdatedAt = time.Now()
	if existing.Code != product.Code {
		delete(r.productIDByCode, existing.Code)
	}
	r.productsByID[product.ID] = copyPaymentProduct(product)
	r.productIDByCode[product.Code] = product.ID
	return nil
}

func (r *PaymentRepo) CreateIntent(_ context.Context, intent *domain.PaymentIntent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.intentIDByKey[intent.IdempotencyKey]; ok {
		return domain.ErrConflict
	}
	if intent.ProviderPaymentID != "" {
		if _, ok := r.intentIDByProvider[intent.ProviderPaymentID]; ok {
			return domain.ErrConflict
		}
	}
	if intent.ID == uuid.Nil {
		intent.ID = uuid.New()
	}
	if intent.Status == "" {
		intent.Status = domain.PaymentIntentCreated
	}
	if intent.Currency == "" {
		intent.Currency = domain.CurrencyRUB
	}
	if intent.Provider == "" {
		intent.Provider = domain.PaymentProviderMock
	}
	if len(intent.Metadata) == 0 {
		intent.Metadata = json.RawMessage(`{}`)
	}
	now := time.Now()
	intent.CreatedAt, intent.UpdatedAt = now, now
	r.intentsByID[intent.ID] = copyPaymentIntent(intent)
	r.intentIDByKey[intent.IdempotencyKey] = intent.ID
	if intent.ProviderPaymentID != "" {
		r.intentIDByProvider[intent.ProviderPaymentID] = intent.ID
	}
	r.intentIDsByUser[intent.UserID] = append([]uuid.UUID{intent.ID}, r.intentIDsByUser[intent.UserID]...)
	return nil
}

func (r *PaymentRepo) GetIntentByID(_ context.Context, id uuid.UUID) (*domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	intent, ok := r.intentsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	out := copyPaymentIntentPtr(intent)
	return out, nil
}

func (r *PaymentRepo) GetIntentByIdempotencyKey(_ context.Context, key string) (*domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.intentIDByKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	intent := r.intentsByID[id]
	out := copyPaymentIntentPtr(intent)
	return out, nil
}

func (r *PaymentRepo) SetIntentProviderState(_ context.Context, id uuid.UUID, status domain.PaymentIntentStatus, providerPaymentID, confirmationURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	intent, ok := r.intentsByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if providerPaymentID != "" {
		if existingID, ok := r.intentIDByProvider[providerPaymentID]; ok && existingID != id {
			return domain.ErrConflict
		}
		r.intentIDByProvider[providerPaymentID] = id
	}
	intent.Status = status
	intent.ProviderPaymentID = providerPaymentID
	intent.ConfirmationURL = confirmationURL
	intent.UpdatedAt = time.Now()
	r.intentsByID[id] = intent
	return nil
}

func (r *PaymentRepo) GetIntentByProviderPaymentID(_ context.Context, provider domain.PaymentProviderCode, providerPaymentID string) (*domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.intentIDByProvider[providerPaymentID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	intent := r.intentsByID[id]
	if intent.Provider != provider {
		return nil, domain.ErrNotFound
	}
	return copyPaymentIntentPtr(intent), nil
}

func (r *PaymentRepo) UpdateIntentStatus(_ context.Context, id uuid.UUID, from, to domain.PaymentIntentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	intent, ok := r.intentsByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if intent.Status != from {
		return domain.ErrConflict
	}
	intent.Status = to
	intent.UpdatedAt = time.Now()
	r.intentsByID[id] = intent
	return nil
}

func (r *PaymentRepo) ListIntentsByUser(_ context.Context, userID uuid.UUID, limit, offset int) ([]*domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.intentIDsByUser[userID]
	var out []*domain.PaymentIntent
	for i := offset; i < len(ids) && len(out) < limit; i++ {
		intent := r.intentsByID[ids[i]]
		out = append(out, copyPaymentIntentPtr(intent))
	}
	return out, nil
}

func (r *PaymentRepo) ListIntents(_ context.Context, filter domain.PaymentIntentFilter, limit, offset int) ([]*domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	statuses := paymentIntentStatusSet(filter)
	source := strings.TrimSpace(filter.Source)
	matched := make([]domain.PaymentIntent, 0, len(r.intentsByID))
	if filter.UserID != nil {
		for _, id := range r.intentIDsByUser[*filter.UserID] {
			intent := r.intentsByID[id]
			if paymentIntentMatchesFilter(intent, filter, statuses, source) {
				matched = append(matched, intent)
			}
		}
	} else {
		for _, intent := range r.intentsByID {
			if paymentIntentMatchesFilter(intent, filter, statuses, source) {
				matched = append(matched, intent)
			}
		}
		sort.Slice(matched, func(i, j int) bool {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		})
	}
	var out []*domain.PaymentIntent
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		out = append(out, copyPaymentIntentPtr(matched[i]))
	}
	return out, nil
}

func paymentIntentMatchesFilter(intent domain.PaymentIntent, filter domain.PaymentIntentFilter, statuses map[domain.PaymentIntentStatus]bool, source string) bool {
	if filter.UserID != nil && intent.UserID != *filter.UserID {
		return false
	}
	if len(statuses) > 0 && !statuses[intent.Status] {
		return false
	}
	if filter.Provider != "" && intent.Provider != filter.Provider {
		return false
	}
	if source != "" && paymentIntentSource(intent) != source {
		return false
	}
	if filter.UpdatedBefore != nil && intent.UpdatedAt.After(*filter.UpdatedBefore) {
		return false
	}
	return true
}

func paymentIntentSource(intent domain.PaymentIntent) string {
	if len(intent.Metadata) == 0 {
		return ""
	}
	var metadata struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.Source)
}

func paymentIntentStatusSet(filter domain.PaymentIntentFilter) map[domain.PaymentIntentStatus]bool {
	statuses := map[domain.PaymentIntentStatus]bool{}
	if filter.Status != "" {
		statuses[filter.Status] = true
	}
	for _, status := range filter.Statuses {
		if status != "" {
			statuses[status] = true
		}
	}
	if len(statuses) == 0 {
		return nil
	}
	return statuses
}

func (r *PaymentRepo) ListIntentsForReconciliation(_ context.Context, filter domain.PaymentReconciliationFilter, limit int) ([]*domain.PaymentIntent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	statuses := map[domain.PaymentIntentStatus]bool{}
	for _, status := range filter.Statuses {
		statuses[status] = true
	}
	if len(statuses) == 0 {
		statuses[domain.PaymentIntentProviderPending] = true
		statuses[domain.PaymentIntentWaitingForUser] = true
	}
	updatedBefore := filter.UpdatedBefore
	if updatedBefore.IsZero() {
		updatedBefore = time.Now()
	}
	matched := make([]domain.PaymentIntent, 0, len(r.intentsByID))
	for _, intent := range r.intentsByID {
		if intent.Provider != filter.Provider ||
			!statuses[intent.Status] ||
			intent.ProviderPaymentID == "" ||
			intent.UpdatedAt.After(updatedBefore) {
			continue
		}
		matched = append(matched, intent)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].UpdatedAt.Before(matched[j].UpdatedAt)
	})
	var out []*domain.PaymentIntent
	for i := 0; i < len(matched) && len(out) < limit; i++ {
		out = append(out, copyPaymentIntentPtr(matched[i]))
	}
	return out, nil
}

func (r *PaymentRepo) CreateEvent(_ context.Context, event *domain.PaymentEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.eventIDByDedup[event.DedupKey]; ok {
		return false, nil
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	now := time.Now()
	event.ReceivedAt = now
	event.UpdatedAt = now
	r.eventsByID[event.ID] = copyPaymentEvent(event)
	r.eventIDByDedup[event.DedupKey] = event.ID
	r.eventIDs = append(r.eventIDs, event.ID)
	return true, nil
}

func (r *PaymentRepo) GetEventByID(_ context.Context, id uuid.UUID) (*domain.PaymentEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.eventsByID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return copyPaymentEventPtr(event), nil
}

func (r *PaymentRepo) ListUnprocessedEvents(_ context.Context, provider domain.PaymentProviderCode, limit int) ([]*domain.PaymentEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.PaymentEvent
	for _, id := range r.eventIDs {
		event := r.eventsByID[id]
		if event.Provider != provider || event.ProcessedAt != nil {
			continue
		}
		out = append(out, copyPaymentEventPtr(event))
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *PaymentRepo) ListEvents(_ context.Context, filter domain.PaymentEventFilter, limit, offset int) ([]*domain.PaymentEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := make([]domain.PaymentEvent, 0, len(r.eventIDs))
	for _, id := range r.eventIDs {
		event := r.eventsByID[id]
		if filter.Provider != "" && event.Provider != filter.Provider {
			continue
		}
		if filter.Processed != nil {
			processed := event.ProcessedAt != nil
			if processed != *filter.Processed {
				continue
			}
		}
		matched = append(matched, event)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ReceivedAt.Before(matched[j].ReceivedAt)
	})
	var out []*domain.PaymentEvent
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		out = append(out, copyPaymentEventPtr(matched[i]))
	}
	return out, nil
}

func (r *PaymentRepo) WebhookInboxStats(_ context.Context, provider domain.PaymentProviderCode) (domain.PaymentWebhookInboxStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stats := domain.PaymentWebhookInboxStats{Provider: provider}
	for _, id := range r.eventIDs {
		event := r.eventsByID[id]
		if event.Provider != provider || event.ProcessedAt != nil {
			continue
		}
		stats.UnprocessedEvents++
		if stats.OldestUnprocessedReceivedAt == nil || event.ReceivedAt.Before(*stats.OldestUnprocessedReceivedAt) {
			receivedAt := event.ReceivedAt
			stats.OldestUnprocessedReceivedAt = &receivedAt
		}
	}
	return stats, nil
}

func (r *PaymentRepo) MarkEventProcessed(_ context.Context, id uuid.UUID, processedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.eventsByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if event.ProcessedAt != nil {
		return nil
	}
	if processedAt.IsZero() {
		processedAt = time.Now()
	}
	event.ProcessedAt = &processedAt
	event.UpdatedAt = processedAt
	r.eventsByID[id] = event
	return nil
}

func (r *PaymentRepo) CreateRefund(_ context.Context, refund *domain.PaymentRefund) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.refundIDByKey[refund.IdempotencyKey]; ok {
		return domain.ErrConflict
	}
	if refund.ProviderRefundID != "" {
		if _, ok := r.refundIDByProvider[refund.ProviderRefundID]; ok {
			return domain.ErrConflict
		}
	}
	if refund.ID == uuid.Nil {
		refund.ID = uuid.New()
	}
	if refund.Status == "" {
		refund.Status = domain.PaymentRefundCreated
	}
	now := time.Now()
	refund.CreatedAt, refund.UpdatedAt = now, now
	r.refundsByID[refund.ID] = copyPaymentRefund(refund)
	r.refundIDByKey[refund.IdempotencyKey] = refund.ID
	if refund.ProviderRefundID != "" {
		r.refundIDByProvider[refund.ProviderRefundID] = refund.ID
	}
	return nil
}

func (r *PaymentRepo) GetRefundByIdempotencyKey(_ context.Context, key string) (*domain.PaymentRefund, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.refundIDByKey[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	refund := r.refundsByID[id]
	return copyPaymentRefundPtr(refund), nil
}

func (r *PaymentRepo) ListRefunds(_ context.Context, filter domain.PaymentRefundFilter, limit, offset int) ([]*domain.PaymentRefund, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	matched := make([]domain.PaymentRefund, 0, len(r.refundsByID))
	for _, refund := range r.refundsByID {
		if filter.IntentID != nil && refund.IntentID != *filter.IntentID {
			continue
		}
		if filter.Status != "" && refund.Status != filter.Status {
			continue
		}
		matched = append(matched, refund)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})
	var out []*domain.PaymentRefund
	for i := offset; i < len(matched) && len(out) < limit; i++ {
		out = append(out, copyPaymentRefundPtr(matched[i]))
	}
	return out, nil
}

func (r *PaymentRepo) SetRefundProviderState(_ context.Context, id uuid.UUID, providerRefundID string, status domain.PaymentRefundStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	refund, ok := r.refundsByID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if providerRefundID != "" {
		if existingID, ok := r.refundIDByProvider[providerRefundID]; ok && existingID != id {
			return domain.ErrConflict
		}
		r.refundIDByProvider[providerRefundID] = id
	}
	refund.ProviderRefundID = providerRefundID
	refund.Status = status
	refund.UpdatedAt = time.Now()
	r.refundsByID[id] = refund
	return nil
}

func copyPaymentIntent(intent *domain.PaymentIntent) domain.PaymentIntent {
	out := *intent
	if intent.ProductID != nil {
		productID := *intent.ProductID
		out.ProductID = &productID
	}
	if intent.VATCode != nil {
		vatCode := *intent.VATCode
		out.VATCode = &vatCode
	}
	if len(intent.Metadata) > 0 {
		out.Metadata = append(json.RawMessage(nil), intent.Metadata...)
	}
	return out
}

func copyPaymentProduct(product *domain.PaymentProduct) domain.PaymentProduct {
	out := *product
	if product.VATCode != nil {
		vatCode := *product.VATCode
		out.VATCode = &vatCode
	}
	return out
}

func copyPaymentProductPtr(product domain.PaymentProduct) *domain.PaymentProduct {
	out := copyPaymentProduct(&product)
	return &out
}

func copyPaymentRefund(refund *domain.PaymentRefund) domain.PaymentRefund {
	return *refund
}

func copyPaymentRefundPtr(refund domain.PaymentRefund) *domain.PaymentRefund {
	out := refund
	return &out
}

func copyPaymentEvent(event *domain.PaymentEvent) domain.PaymentEvent {
	out := *event
	if len(event.Payload) > 0 {
		out.Payload = append(json.RawMessage(nil), event.Payload...)
	}
	if event.ProcessedAt != nil {
		processedAt := *event.ProcessedAt
		out.ProcessedAt = &processedAt
	}
	return out
}

func copyPaymentEventPtr(event domain.PaymentEvent) *domain.PaymentEvent {
	out := event
	if len(event.Payload) > 0 {
		out.Payload = append(json.RawMessage(nil), event.Payload...)
	}
	if event.ProcessedAt != nil {
		processedAt := *event.ProcessedAt
		out.ProcessedAt = &processedAt
	}
	return &out
}

func copyPaymentIntentPtr(intent domain.PaymentIntent) *domain.PaymentIntent {
	out := intent
	if intent.ProductID != nil {
		productID := *intent.ProductID
		out.ProductID = &productID
	}
	if intent.VATCode != nil {
		vatCode := *intent.VATCode
		out.VATCode = &vatCode
	}
	if len(intent.Metadata) > 0 {
		out.Metadata = append(json.RawMessage(nil), intent.Metadata...)
	}
	return &out
}
