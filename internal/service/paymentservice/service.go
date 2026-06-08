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

// CreateIntentInput describes a user-owned top-up intent creation request.
type CreateIntentInput struct {
	UserID         uuid.UUID
	ProductCode    string
	ReceiptEmail   string
	ReceiptPhone   string
	IdempotencyKey string
	ReturnURL      string
	Source         string
}

// CreateIntentResult reports the intent and whether this call inserted the
// local row. Provider creation may still be retried for an existing local row
// that lacks provider fields.
type CreateIntentResult struct {
	Intent  *domain.PaymentIntent
	Created bool
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
	if in.ReceiptEmail == "" && in.ReceiptPhone == "" {
		return CreateIntentResult{}, ErrReceiptContactRequired
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
		UserID:         in.UserID,
		ProductID:      &product.ID,
		Status:         domain.PaymentIntentCreated,
		Amount:         product.Amount,
		Currency:       product.Currency,
		Credits:        product.Credits,
		PriceVersion:   product.PriceVersion,
		Provider:       s.provider.Code(),
		IdempotencyKey: in.IdempotencyKey,
		ReceiptEmail:   in.ReceiptEmail,
		ReceiptPhone:   in.ReceiptPhone,
		Metadata:       metadata,
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
		Description:    paymentDescription(product),
		ReturnURL:      defaultString(returnURL, s.cfg.ReturnURL),
		ReceiptEmail:   intent.ReceiptEmail,
		ReceiptPhone:   intent.ReceiptPhone,
		IdempotencyKey: "pay:" + intent.ID.String(),
	}
	if product != nil {
		createInput.VATCode = product.VATCode
		createInput.PaymentSubject = product.PaymentSubject
		createInput.PaymentMode = product.PaymentMode
		createInput.Metadata = intent.Metadata
	}
	result, err := s.provider.CreatePayment(ctx, createInput)
	if err != nil {
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
