// Package paymentservice owns payment-intent lifecycle rules shared by VK Bot,
// VK Mini App and protected operator endpoints.
package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

var (
	ErrInvalidInput           = errors.New("paymentservice: invalid input")
	ErrReceiptContactRequired = errors.New("paymentservice: receipt email or phone is required")
	ErrForbidden              = errors.New("paymentservice: forbidden")
)

const (
	defaultReceiptVATCode        = int16(1)
	defaultReceiptPaymentSubject = "service"
	defaultReceiptPaymentMode    = "full_prepayment"
	maxProductCodeLen            = 64
	maxProductTitleLen           = 160
	devTestPaymentProductCode    = "crystals_10_dev"
)

// Config controls payment lifecycle behavior.
type Config struct {
	ReturnURL                    string
	IncludeDevTestPaymentProduct bool
}

// Service creates and reads payment intents without mutating credit balances.
type Service struct {
	repo     domain.PaymentRepository
	provider domain.PaymentProvider
	cfg      Config
}

// New builds a payment Service.
func New(repo domain.PaymentRepository, provider domain.PaymentProvider, cfg Config) *Service {
	return &Service{repo: repo, provider: provider, cfg: cfg}
}

// ListActiveProducts returns active top-up catalog entries for user-facing
// surfaces. It does not create payment intents or mutate billing state.
func (s *Service) ListActiveProducts(ctx context.Context) ([]*domain.PaymentProduct, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	products, err := s.repo.ListActiveProducts(ctx)
	if err != nil {
		return nil, err
	}
	return s.filterUserFacingProducts(products), nil
}

// ListProducts returns product catalog entries for protected operator surfaces.
func (s *Service) ListProducts(ctx context.Context, active *bool, limit, offset int) ([]*domain.PaymentProduct, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	return s.repo.ListProducts(ctx, domain.PaymentProductFilter{Active: active}, normalizeLimit(limit), normalizeOffset(offset))
}

// GetProductAdmin fetches one product for protected operator surfaces.
func (s *Service) GetProductAdmin(ctx context.Context, productID uuid.UUID) (*domain.PaymentProduct, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	if productID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.GetProductByID(ctx, productID)
}

// CreateProductInput describes a protected operator product-catalog create.
type CreateProductInput struct {
	Code           string
	Title          string
	Amount         int64
	Currency       domain.Currency
	Credits        int64
	VATCode        *int16
	PaymentSubject string
	PaymentMode    string
	IsActive       bool
}

// UpdateProductInput describes a protected operator product-catalog update.
// Code is intentionally immutable through this path; create a new product code
// for a different public package identity.
type UpdateProductInput struct {
	ID             uuid.UUID
	Title          *string
	Amount         *int64
	Currency       *domain.Currency
	Credits        *int64
	VATCodeSet     bool
	VATCode        *int16
	PaymentSubject *string
	PaymentMode    *string
	IsActive       *bool
}

// CreateProduct inserts one active or hidden top-up product. It does not touch
// existing intents or balances.
func (s *Service) CreateProduct(ctx context.Context, in CreateProductInput) (*domain.PaymentProduct, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	product := &domain.PaymentProduct{
		Code:           strings.TrimSpace(in.Code),
		Title:          strings.TrimSpace(in.Title),
		Amount:         in.Amount,
		Currency:       in.Currency,
		Credits:        in.Credits,
		PriceVersion:   1,
		VATCode:        cloneInt16(in.VATCode),
		PaymentSubject: strings.TrimSpace(in.PaymentSubject),
		PaymentMode:    strings.TrimSpace(in.PaymentMode),
		IsActive:       in.IsActive,
	}
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	if product.VATCode == nil {
		product.VATCode = cloneInt16(ptrInt16(defaultReceiptVATCode))
	}
	if product.PaymentSubject == "" {
		product.PaymentSubject = defaultReceiptPaymentSubject
	}
	if product.PaymentMode == "" {
		product.PaymentMode = defaultReceiptPaymentMode
	}
	if err := validateProduct(product); err != nil {
		return nil, err
	}
	if err := s.repo.CreateProduct(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

// UpdateProduct applies protected operator catalog changes. Snapshot-sensitive
// changes increment PriceVersion so future intents can be audited against the
// catalog version they copied from. Existing intents remain untouched.
func (s *Service) UpdateProduct(ctx context.Context, in UpdateProductInput) (*domain.PaymentProduct, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	if in.ID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	product, err := s.repo.GetProductByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	next := *product
	next.VATCode = cloneInt16(product.VATCode)

	snapshotChanged := false
	if in.Title != nil {
		value := strings.TrimSpace(*in.Title)
		if value != next.Title {
			next.Title = value
			snapshotChanged = true
		}
	}
	if in.Amount != nil && *in.Amount != next.Amount {
		next.Amount = *in.Amount
		snapshotChanged = true
	}
	if in.Currency != nil {
		value := *in.Currency
		if value == "" {
			value = domain.CurrencyRUB
		}
		if value != next.Currency {
			next.Currency = value
			snapshotChanged = true
		}
	}
	if in.Credits != nil && *in.Credits != next.Credits {
		next.Credits = *in.Credits
		snapshotChanged = true
	}
	if in.VATCodeSet && !sameInt16(in.VATCode, next.VATCode) {
		next.VATCode = cloneInt16(in.VATCode)
		snapshotChanged = true
	}
	if in.PaymentSubject != nil {
		value := strings.TrimSpace(*in.PaymentSubject)
		if value == "" {
			value = defaultReceiptPaymentSubject
		}
		if value != next.PaymentSubject {
			next.PaymentSubject = value
			snapshotChanged = true
		}
	}
	if in.PaymentMode != nil {
		value := strings.TrimSpace(*in.PaymentMode)
		if value == "" {
			value = defaultReceiptPaymentMode
		}
		if value != next.PaymentMode {
			next.PaymentMode = value
			snapshotChanged = true
		}
	}
	if in.IsActive != nil {
		next.IsActive = *in.IsActive
	}
	if snapshotChanged {
		next.PriceVersion++
	}
	if err := validateProduct(&next); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateProduct(ctx, &next); err != nil {
		return nil, err
	}
	return &next, nil
}

// DisableProduct hides one product from user-facing product lists without
// mutating existing payment intents.
func (s *Service) DisableProduct(ctx context.Context, productID uuid.UUID) (*domain.PaymentProduct, error) {
	active := false
	return s.UpdateProduct(ctx, UpdateProductInput{ID: productID, IsActive: &active})
}

// CreateIntentInput describes a user-owned top-up intent creation request.
type CreateIntentInput struct {
	UserID         uuid.UUID
	ProductCode    string
	ReceiptEmail   string
	ReceiptPhone   string
	IdempotencyKey string
	ReturnURL      string
	Source         string
	ForceNew       bool
	Capture        *bool
}

// CreateIntentResult reports the intent and whether this call inserted the
// local row. Provider creation may still be retried for an existing local row
// that lacks provider fields.
type CreateIntentResult struct {
	Intent       *domain.PaymentIntent
	Created      bool
	ReusedActive bool
}

// AttachVKBotPaymentMessageInput links a VK bot payment message to a local
// intent so webhook/reconciliation status changes can edit that message later.
type AttachVKBotPaymentMessageInput struct {
	UserID    uuid.UUID
	IntentID  uuid.UUID
	VKPeerID  int64
	MessageID int64
}

// CreateIntent creates or resumes an idempotent payment intent. It never grants
// credits; only trusted webhook processing may later post a top-up ledger entry.
func (s *Service) CreateIntent(ctx context.Context, in CreateIntentInput) (CreateIntentResult, error) {
	if s == nil || s.repo == nil || s.provider == nil {
		return CreateIntentResult{}, errors.New("paymentservice: service is not configured")
	}
	in.ProductCode = strings.TrimSpace(in.ProductCode)
	in.ReceiptEmail = strings.TrimSpace(in.ReceiptEmail)
	in.ReceiptPhone = strings.TrimSpace(in.ReceiptPhone)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.ReturnURL = strings.TrimSpace(in.ReturnURL)
	resolvedReturnURL := defaultString(in.ReturnURL, s.cfg.ReturnURL)
	in.Source = strings.TrimSpace(in.Source)
	sourceLabel := metricLabel(in.Source)
	if in.UserID == uuid.Nil || in.ProductCode == "" || in.IdempotencyKey == "" {
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "invalid_input")
		return CreateIntentResult{}, ErrInvalidInput
	}

	if existing, err := s.repo.GetIntentByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		if existing.UserID != in.UserID {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "forbidden")
			return CreateIntentResult{}, ErrForbidden
		}
		intent, err := s.ensureProviderPayment(ctx, existing, nil, in.ReturnURL)
		if err != nil {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "provider_error")
			return CreateIntentResult{}, err
		}
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "idempotent")
		return CreateIntentResult{Intent: intent, Created: false}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "error")
		return CreateIntentResult{}, err
	}

	var product *domain.PaymentProduct
	if !in.ForceNew {
		var err error
		product, err = s.repo.GetActiveProductByCode(ctx, in.ProductCode)
		if err != nil {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "invalid_product")
			return CreateIntentResult{}, err
		}
		if !s.userFacingProductAllowed(product) {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "invalid_product")
			return CreateIntentResult{}, domain.ErrNotFound
		}
		active, err := s.ActiveWaitingIntentForSource(ctx, in.UserID, in.Source)
		if err == nil {
			if paymentIntentMatchesProduct(active, product) && paymentIntentMatchesReturnURL(active, resolvedReturnURL) {
				metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "reused_active")
				return CreateIntentResult{Intent: active, Created: false, ReusedActive: true}, nil
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "error")
			return CreateIntentResult{}, err
		}
	}

	if in.ReceiptEmail == "" && in.ReceiptPhone == "" {
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "receipt_contact_required")
		return CreateIntentResult{}, ErrReceiptContactRequired
	}

	if product == nil {
		var err error
		product, err = s.repo.GetActiveProductByCode(ctx, in.ProductCode)
		if err != nil {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "invalid_product")
			return CreateIntentResult{}, err
		}
		if !s.userFacingProductAllowed(product) {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "invalid_product")
			return CreateIntentResult{}, domain.ErrNotFound
		}
	}
	metadataFields := map[string]any{
		"product_code":  product.Code,
		"price_version": product.PriceVersion,
		"source":        in.Source,
	}
	if resolvedReturnURL != "" {
		metadataFields["return_url"] = resolvedReturnURL
	}
	if in.Capture != nil {
		metadataFields["capture"] = *in.Capture
	}
	metadata, err := json.Marshal(metadataFields)
	if err != nil {
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "error")
		return CreateIntentResult{}, err
	}
	intent := &domain.PaymentIntent{
		UserID:             in.UserID,
		ProductID:          &product.ID,
		Status:             domain.PaymentIntentCreated,
		Amount:             product.Amount,
		Currency:           product.Currency,
		Credits:            product.Credits,
		PriceVersion:       product.PriceVersion,
		ReceiptDescription: paymentDescription(product),
		VATCode:            cloneInt16(product.VATCode),
		PaymentSubject:     strings.TrimSpace(product.PaymentSubject),
		PaymentMode:        strings.TrimSpace(product.PaymentMode),
		Provider:           s.provider.Code(),
		IdempotencyKey:     in.IdempotencyKey,
		ReceiptEmail:       in.ReceiptEmail,
		ReceiptPhone:       in.ReceiptPhone,
		Metadata:           metadata,
	}
	if err := s.repo.CreateIntent(ctx, intent); err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "error")
			return CreateIntentResult{}, err
		}
		existing, getErr := s.repo.GetIntentByIdempotencyKey(ctx, in.IdempotencyKey)
		if getErr != nil {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "error")
			return CreateIntentResult{}, getErr
		}
		if existing.UserID != in.UserID {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "forbidden")
			return CreateIntentResult{}, ErrForbidden
		}
		intent, err := s.ensureProviderPayment(ctx, existing, nil, in.ReturnURL)
		if err != nil {
			metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "provider_error")
			return CreateIntentResult{}, err
		}
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "idempotent")
		return CreateIntentResult{Intent: intent, Created: false}, nil
	}

	intent, err = s.ensureProviderPayment(ctx, intent, product, in.ReturnURL)
	if err != nil {
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "provider_error")
		return CreateIntentResult{}, err
	}
	metrics.PaymentsCreated.WithLabelValues(string(intent.Provider), sourceLabel).Inc()
	metrics.ObserveProductEvent(sourceLabel, "payment", "intent_create", "top_up", "credits", "created")
	return CreateIntentResult{Intent: intent, Created: true}, nil
}

func (s *Service) filterUserFacingProducts(products []*domain.PaymentProduct) []*domain.PaymentProduct {
	if len(products) == 0 {
		return products
	}
	out := products[:0]
	for _, product := range products {
		if s.userFacingProductAllowed(product) {
			out = append(out, product)
		}
	}
	return out
}

func (s *Service) userFacingProductAllowed(product *domain.PaymentProduct) bool {
	if product == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(product.Code), devTestPaymentProductCode) {
		return s != nil && s.cfg.IncludeDevTestPaymentProduct
	}
	return true
}

// AttachVKBotPaymentMessage stores only the minimal local routing data needed
// to edit the VK bot top-up message after provider-confirmed status changes.
// It never sends this metadata back to the payment provider.
func (s *Service) AttachVKBotPaymentMessage(ctx context.Context, in AttachVKBotPaymentMessageInput) (*domain.PaymentIntent, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	if in.UserID == uuid.Nil || in.IntentID == uuid.Nil || in.VKPeerID <= 0 || in.MessageID <= 0 {
		return nil, ErrInvalidInput
	}
	intent, err := s.repo.GetIntentByID(ctx, in.IntentID)
	if err != nil {
		return nil, err
	}
	if intent.UserID != in.UserID {
		return nil, ErrForbidden
	}
	if paymentMetadataSource(intent.Metadata) != "vk_bot" {
		return nil, ErrForbidden
	}
	metadata, err := paymentMetadataMap(intent.Metadata)
	if err != nil {
		return nil, err
	}
	metadata["vk_peer_id"] = in.VKPeerID
	metadata["vk_payment_message_id"] = in.MessageID
	metadata["vk_payment_message_tracked_at"] = time.Now().UTC().Format(time.RFC3339)
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateIntentMetadata(ctx, intent.ID, raw); err != nil {
		return nil, err
	}
	return s.repo.GetIntentByID(ctx, intent.ID)
}

// CancelUserIntentInput describes a user-owned cancel request from one product
// surface. It is intentionally narrower than protected operator cancellation.
type CancelUserIntentInput struct {
	UserID   uuid.UUID
	IntentID uuid.UUID
	Source   string
}

// CancelUserWaitingIntent cancels a user-owned waiting payment. It does not
// mutate credits; successful payments must still be granted only by trusted
// webhook/reconciliation processing.
func (s *Service) CancelUserWaitingIntent(ctx context.Context, in CancelUserIntentInput) (*domain.PaymentIntent, error) {
	if s == nil || s.repo == nil || s.provider == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	in.Source = strings.TrimSpace(in.Source)
	if in.UserID == uuid.Nil || in.IntentID == uuid.Nil || in.Source == "" {
		return nil, ErrInvalidInput
	}
	intent, err := s.repo.GetIntentByID(ctx, in.IntentID)
	if err != nil {
		return nil, err
	}
	if intent.UserID != in.UserID || paymentMetadataSource(intent.Metadata) != in.Source {
		return nil, domain.ErrNotFound
	}
	switch intent.Status {
	case domain.PaymentIntentCanceled, domain.PaymentIntentExpired, domain.PaymentIntentFailed:
		return intent, nil
	case domain.PaymentIntentWaitingForUser:
		// allowed below
	default:
		return nil, domain.ErrConflict
	}
	if strings.TrimSpace(intent.ProviderPaymentID) == "" {
		return nil, ErrInvalidInput
	}
	sourceLabel := metricLabel(in.Source)
	if err := s.provider.CancelPayment(ctx, intent.ProviderPaymentID); err != nil {
		recordPaymentProviderError(s.provider.Code(), "cancel_payment", err)
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_cancel", "top_up", "credits", "provider_error")
		return nil, fmt.Errorf("paymentservice: cancel provider payment: %w", err)
	}
	providerPayment, err := s.provider.GetPayment(ctx, intent.ProviderPaymentID)
	if err != nil {
		recordPaymentProviderError(s.provider.Code(), "get_payment", err)
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_cancel", "top_up", "credits", "provider_error")
		return nil, fmt.Errorf("paymentservice: verify canceled payment: %w", err)
	}
	switch providerPayment.Status {
	case domain.PaymentIntentCanceled, domain.PaymentIntentExpired, domain.PaymentIntentFailed:
		if intent.Status != providerPayment.Status {
			if err := s.repo.UpdateIntentStatus(ctx, intent.ID, intent.Status, providerPayment.Status); err != nil && !errors.Is(err, domain.ErrConflict) {
				return nil, err
			}
		}
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_cancel", "top_up", "credits", "success")
		return s.repo.GetIntentByID(ctx, intent.ID)
	case domain.PaymentIntentWaitingForUser, domain.PaymentIntentProviderPending:
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_cancel", "top_up", "credits", "pending")
		return s.repo.GetIntentByID(ctx, intent.ID)
	default:
		metrics.ObserveProductEvent(sourceLabel, "payment", "intent_cancel", "top_up", "credits", "conflict")
		return nil, domain.ErrConflict
	}
}

// ActiveWaitingIntent returns the newest user-owned payment that still needs
// user confirmation. It does not grant credits and does not query the provider.
func (s *Service) ActiveWaitingIntent(ctx context.Context, userID uuid.UUID) (*domain.PaymentIntent, error) {
	return s.ActiveWaitingIntentForSource(ctx, userID, "")
}

// ActiveWaitingIntentForSource returns the newest user-owned payment for one
// product surface that still needs user confirmation.
func (s *Service) ActiveWaitingIntentForSource(ctx context.Context, userID uuid.UUID, source string) (*domain.PaymentIntent, error) {
	if s == nil || s.repo == nil || s.provider == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	intents, err := s.repo.ListIntents(ctx, domain.PaymentIntentFilter{
		UserID:   &userID,
		Status:   domain.PaymentIntentWaitingForUser,
		Provider: s.provider.Code(),
		Source:   strings.TrimSpace(source),
	}, 20, 0)
	if err != nil {
		return nil, err
	}
	for _, intent := range intents {
		if intent != nil && strings.TrimSpace(intent.ConfirmationURL) != "" {
			return intent, nil
		}
	}
	return nil, domain.ErrNotFound
}

// GetIntent fetches one user-owned intent.
func (s *Service) GetIntent(ctx context.Context, userID, intentID uuid.UUID) (*domain.PaymentIntent, error) {
	intent, err := s.repo.GetIntentByID(ctx, intentID)
	if err != nil {
		return nil, err
	}
	if intent.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return intent, nil
}

// GetIntentAdmin fetches one intent for a protected operator endpoint.
func (s *Service) GetIntentAdmin(ctx context.Context, intentID uuid.UUID) (*domain.PaymentIntent, error) {
	return s.repo.GetIntentByID(ctx, intentID)
}

// ListIntentsByUser returns user-owned payment history.
func (s *Service) ListIntentsByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.PaymentIntent, error) {
	return s.repo.ListIntentsByUser(ctx, userID, normalizeLimit(limit), normalizeOffset(offset))
}

// ListIntentsByUserSource returns user-owned payment history for one product
// surface.
func (s *Service) ListIntentsByUserSource(ctx context.Context, userID uuid.UUID, source string, limit, offset int) ([]*domain.PaymentIntent, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repo.ListIntents(ctx, domain.PaymentIntentFilter{
		UserID: &userID,
		Source: strings.TrimSpace(source),
	}, normalizeLimit(limit), normalizeOffset(offset))
}

// ListIntents returns protected operator payment history.
func (s *Service) ListIntents(ctx context.Context, filter domain.PaymentIntentFilter, limit, offset int) ([]*domain.PaymentIntent, error) {
	return s.repo.ListIntents(ctx, filter, normalizeLimit(limit), normalizeOffset(offset))
}

// ListEvents returns provider webhook inbox events for protected operator
// surfaces. Callers must not expose PaymentEvent.Payload.
func (s *Service) ListEvents(ctx context.Context, filter domain.PaymentEventFilter, limit, offset int) ([]*domain.PaymentEvent, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	return s.repo.ListEvents(ctx, filter, normalizeLimit(limit), normalizeOffset(offset))
}

// WebhookInboxStats returns the safe provider webhook backlog for protected
// operator health views. It never exposes raw provider payloads.
func (s *Service) WebhookInboxStats(ctx context.Context, provider domain.PaymentProviderCode) (domain.PaymentWebhookInboxStats, error) {
	if s == nil || s.repo == nil {
		return domain.PaymentWebhookInboxStats{}, errors.New("paymentservice: service is not configured")
	}
	return s.repo.WebhookInboxStats(ctx, provider)
}

// ListRefunds returns protected operator refund history. Callers must not expose
// internal idempotency keys or raw provider identifiers without masking.
func (s *Service) ListRefunds(ctx context.Context, filter domain.PaymentRefundFilter, limit, offset int) ([]*domain.PaymentRefund, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("paymentservice: service is not configured")
	}
	return s.repo.ListRefunds(ctx, filter, normalizeLimit(limit), normalizeOffset(offset))
}

func (s *Service) ensureProviderPayment(ctx context.Context, intent *domain.PaymentIntent, product *domain.PaymentProduct, returnURL string) (*domain.PaymentIntent, error) {
	if intent == nil {
		return nil, domain.ErrNotFound
	}
	if intent.ProviderPaymentID != "" || intent.ConfirmationURL != "" || intent.Status != domain.PaymentIntentCreated {
		return intent, nil
	}
	if product == nil && intent.ProductID != nil {
		if existingProduct, err := s.repo.GetProductByID(ctx, *intent.ProductID); err == nil {
			product = existingProduct
		}
	}
	createInput := domain.CreatePaymentInput{
		IntentID:       intent.ID,
		UserID:         intent.UserID,
		Amount:         intent.Amount,
		Currency:       intent.Currency,
		Credits:        intent.Credits,
		Description:    paymentIntentDescription(intent, product),
		ReturnURL:      defaultString(returnURL, s.cfg.ReturnURL),
		ReceiptEmail:   intent.ReceiptEmail,
		ReceiptPhone:   intent.ReceiptPhone,
		VATCode:        paymentIntentVATCode(intent, product),
		PaymentSubject: paymentIntentSubject(intent, product),
		PaymentMode:    paymentIntentMode(intent, product),
		Metadata:       intent.Metadata,
		IdempotencyKey: "pay:" + intent.ID.String(),
		Capture:        paymentIntentCapture(intent),
	}
	result, err := s.provider.CreatePayment(ctx, createInput)
	if err != nil {
		recordPaymentProviderError(s.provider.Code(), "create_payment", err)
		return nil, fmt.Errorf("paymentservice: create provider payment: %w", err)
	}
	if err := validateConfirmationURL(result.ConfirmationURL); err != nil {
		recordPaymentProviderError(s.provider.Code(), "create_payment", err)
		return nil, err
	}
	if err := s.repo.SetIntentProviderState(ctx, intent.ID, result.Status, result.ProviderPaymentID, result.ConfirmationURL); err != nil {
		return nil, err
	}
	updated, err := s.repo.GetIntentByID(ctx, intent.ID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func validateConfirmationURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("paymentservice: unsafe payment confirmation url")
	}
	return nil
}

func paymentDescription(product *domain.PaymentProduct) string {
	if product != nil && strings.TrimSpace(product.Title) != "" {
		return product.Title
	}
	return "NeiroHub balance top-up"
}

func paymentIntentDescription(intent *domain.PaymentIntent, product *domain.PaymentProduct) string {
	if intent != nil && strings.TrimSpace(intent.ReceiptDescription) != "" {
		return intent.ReceiptDescription
	}
	return paymentDescription(product)
}

func paymentIntentVATCode(intent *domain.PaymentIntent, product *domain.PaymentProduct) *int16 {
	if intent != nil && intent.VATCode != nil {
		return cloneInt16(intent.VATCode)
	}
	if product != nil {
		return cloneInt16(product.VATCode)
	}
	return nil
}

func paymentIntentSubject(intent *domain.PaymentIntent, product *domain.PaymentProduct) string {
	if intent != nil && strings.TrimSpace(intent.PaymentSubject) != "" {
		return intent.PaymentSubject
	}
	if product != nil {
		return product.PaymentSubject
	}
	return ""
}

func paymentIntentMode(intent *domain.PaymentIntent, product *domain.PaymentProduct) string {
	if intent != nil && strings.TrimSpace(intent.PaymentMode) != "" {
		return intent.PaymentMode
	}
	if product != nil {
		return product.PaymentMode
	}
	return ""
}

func paymentIntentCapture(intent *domain.PaymentIntent) *bool {
	if intent == nil || len(intent.Metadata) == 0 {
		return nil
	}
	var metadata struct {
		Capture *bool `json:"capture"`
	}
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return nil
	}
	return metadata.Capture
}

func paymentMetadataMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}
	if metadata == nil {
		return map[string]any{}, nil
	}
	return metadata, nil
}

func paymentMetadataSource(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var metadata struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.Source)
}

func paymentMetadataReturnURL(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var metadata struct {
		ReturnURL string `json:"return_url"`
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.ReturnURL)
}

func paymentIntentMatchesProduct(intent *domain.PaymentIntent, product *domain.PaymentProduct) bool {
	if intent == nil || product == nil || intent.ProductID == nil {
		return false
	}
	return *intent.ProductID == product.ID
}

func paymentIntentMatchesReturnURL(intent *domain.PaymentIntent, resolvedReturnURL string) bool {
	if intent == nil {
		return false
	}
	stored := paymentMetadataReturnURL(intent.Metadata)
	if stored == "" {
		return strings.TrimSpace(resolvedReturnURL) == ""
	}
	return stored == strings.TrimSpace(resolvedReturnURL)
}

func cloneInt16(value *int16) *int16 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

func ptrInt16(value int16) *int16 {
	out := value
	return &out
}

func sameInt16(a, b *int16) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func validateProduct(product *domain.PaymentProduct) error {
	if product == nil {
		return ErrInvalidInput
	}
	product.Code = strings.TrimSpace(product.Code)
	product.Title = strings.TrimSpace(product.Title)
	product.PaymentSubject = strings.TrimSpace(product.PaymentSubject)
	product.PaymentMode = strings.TrimSpace(product.PaymentMode)
	if !validProductCode(product.Code) {
		return ErrInvalidInput
	}
	if product.Title == "" || len(product.Title) > maxProductTitleLen {
		return ErrInvalidInput
	}
	if product.Amount <= 0 || product.Credits <= 0 || product.PriceVersion <= 0 {
		return ErrInvalidInput
	}
	if product.Currency == "" {
		product.Currency = domain.CurrencyRUB
	}
	if product.Currency != domain.CurrencyRUB {
		return ErrInvalidInput
	}
	if product.VATCode != nil && !validVATCode(*product.VATCode) {
		return ErrInvalidInput
	}
	if !validPaymentSubject(product.PaymentSubject) || !validPaymentMode(product.PaymentMode) {
		return ErrInvalidInput
	}
	return nil
}

func validProductCode(code string) bool {
	if len(code) < 3 || len(code) > maxProductCodeLen {
		return false
	}
	for i, r := range code {
		ok := r >= 'a' && r <= 'z' ||
			r >= '0' && r <= '9' ||
			r == '_' ||
			r == '-'
		if !ok {
			return false
		}
		if i == 0 && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func validVATCode(code int16) bool {
	return code >= 1 && code <= 6
}

func validPaymentSubject(value string) bool {
	if value == "" {
		return true
	}
	switch value {
	case "commodity",
		"excise",
		"job",
		"service",
		"gambling_bet",
		"gambling_prize",
		"lottery",
		"lottery_prize",
		"intellectual_activity",
		"payment",
		"agent_commission",
		"composite",
		"another":
		return true
	default:
		return false
	}
}

func validPaymentMode(value string) bool {
	if value == "" {
		return true
	}
	switch value {
	case "full_prepayment",
		"partial_prepayment",
		"advance",
		"full_payment",
		"partial_payment",
		"credit",
		"credit_payment":
		return true
	default:
		return false
	}
}

func defaultString(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func metricLabel(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "unknown"
}
