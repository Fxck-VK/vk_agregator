// Package paymentservice owns payment-intent lifecycle rules shared by VK Bot,
// VK Mini App and protected operator endpoints.
package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
)

var (
	ErrInvalidInput           = errors.New("paymentservice: invalid input")
	ErrReceiptContactRequired = errors.New("paymentservice: receipt email or phone is required")
	ErrForbidden              = errors.New("paymentservice: forbidden")
)

// Config controls payment lifecycle behavior.
type Config struct {
	ReturnURL string
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
	return s.repo.ListActiveProducts(ctx)
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
}

// CreateIntentResult reports the intent and whether this call inserted the
// local row. Provider creation may still be retried for an existing local row
// that lacks provider fields.
type CreateIntentResult struct {
	Intent       *domain.PaymentIntent
	Created      bool
	ReusedActive bool
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
	in.Source = strings.TrimSpace(in.Source)
	if in.UserID == uuid.Nil || in.ProductCode == "" || in.IdempotencyKey == "" {
		return CreateIntentResult{}, ErrInvalidInput
	}

	if existing, err := s.repo.GetIntentByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		if existing.UserID != in.UserID {
			return CreateIntentResult{}, ErrForbidden
		}
		intent, err := s.ensureProviderPayment(ctx, existing, nil, in.ReturnURL)
		if err != nil {
			return CreateIntentResult{}, err
		}
		return CreateIntentResult{Intent: intent, Created: false}, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return CreateIntentResult{}, err
	}

	if !in.ForceNew {
		active, err := s.ActiveWaitingIntent(ctx, in.UserID)
		if err == nil {
			return CreateIntentResult{Intent: active, Created: false, ReusedActive: true}, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return CreateIntentResult{}, err
		}
	}

	if in.ReceiptEmail == "" && in.ReceiptPhone == "" {
		return CreateIntentResult{}, ErrReceiptContactRequired
	}

	product, err := s.repo.GetActiveProductByCode(ctx, in.ProductCode)
	if err != nil {
		return CreateIntentResult{}, err
	}
	metadata, err := json.Marshal(map[string]any{
		"product_code":  product.Code,
		"price_version": product.PriceVersion,
		"source":        in.Source,
	})
	if err != nil {
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
			return CreateIntentResult{}, err
		}
		existing, getErr := s.repo.GetIntentByIdempotencyKey(ctx, in.IdempotencyKey)
		if getErr != nil {
			return CreateIntentResult{}, getErr
		}
		if existing.UserID != in.UserID {
			return CreateIntentResult{}, ErrForbidden
		}
		intent, err := s.ensureProviderPayment(ctx, existing, nil, in.ReturnURL)
		if err != nil {
			return CreateIntentResult{}, err
		}
		return CreateIntentResult{Intent: intent, Created: false}, nil
	}

	intent, err = s.ensureProviderPayment(ctx, intent, product, in.ReturnURL)
	if err != nil {
		return CreateIntentResult{}, err
	}
	metrics.PaymentsCreated.WithLabelValues(string(intent.Provider), metricLabel(in.Source)).Inc()
	return CreateIntentResult{Intent: intent, Created: true}, nil
}

// ActiveWaitingIntent returns the newest user-owned payment that still needs
// user confirmation. It does not grant credits and does not query the provider.
func (s *Service) ActiveWaitingIntent(ctx context.Context, userID uuid.UUID) (*domain.PaymentIntent, error) {
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
	}
	result, err := s.provider.CreatePayment(ctx, createInput)
	if err != nil {
		recordPaymentProviderError(s.provider.Code(), "create_payment", err)
		return nil, fmt.Errorf("paymentservice: create provider payment: %w", err)
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

func cloneInt16(value *int16) *int16 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
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
