package mock_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/domain"
)

func TestCreatePaymentIsIdempotent(t *testing.T) {
	provider := mock.New()
	ctx := context.Background()
	intentID := uuid.New()
	input := domain.CreatePaymentInput{
		IntentID:       intentID,
		UserID:         uuid.New(),
		Amount:         9900,
		Currency:       domain.CurrencyRUB,
		Credits:        100,
		IdempotencyKey: "pay:" + intentID.String(),
	}

	first, err := provider.CreatePayment(ctx, input)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	second, err := provider.CreatePayment(ctx, input)
	if err != nil {
		t.Fatalf("create duplicate payment: %v", err)
	}
	if second.ProviderPaymentID != first.ProviderPaymentID {
		t.Fatalf("duplicate payment id = %q, want %q", second.ProviderPaymentID, first.ProviderPaymentID)
	}
	if first.Status != domain.PaymentIntentWaitingForUser {
		t.Fatalf("status = %q, want waiting_for_user", first.Status)
	}
}

func TestGetCancelAndRefundPayment(t *testing.T) {
	provider := mock.New()
	ctx := context.Background()
	intentID := uuid.New()
	created, err := provider.CreatePayment(ctx, domain.CreatePaymentInput{
		IntentID:       intentID,
		UserID:         uuid.New(),
		Amount:         10000,
		Currency:       domain.CurrencyRUB,
		Credits:        100,
		IdempotencyKey: "pay:" + intentID.String(),
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if err := provider.SetPaymentStatus(created.ProviderPaymentID, domain.PaymentIntentSucceeded); err != nil {
		t.Fatalf("set payment status: %v", err)
	}
	payment, err := provider.GetPayment(ctx, created.ProviderPaymentID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if !payment.Paid || !payment.Captured || !payment.Refundable {
		t.Fatalf("unexpected paid flags: paid=%v captured=%v refundable=%v", payment.Paid, payment.Captured, payment.Refundable)
	}

	refundID := uuid.New()
	refund, err := provider.CreateRefund(ctx, domain.CreateRefundInput{
		RefundID:          refundID,
		IntentID:          intentID,
		ProviderPaymentID: created.ProviderPaymentID,
		Amount:            10000,
		Currency:          domain.CurrencyRUB,
		IdempotencyKey:    "payrefund:" + refundID.String(),
	})
	if err != nil {
		t.Fatalf("create refund: %v", err)
	}
	if refund.Status != domain.PaymentRefundSucceeded {
		t.Fatalf("refund status = %q, want succeeded", refund.Status)
	}
	refundAgain, err := provider.CreateRefund(ctx, domain.CreateRefundInput{
		RefundID:          refundID,
		IntentID:          intentID,
		ProviderPaymentID: created.ProviderPaymentID,
		Amount:            10000,
		Currency:          domain.CurrencyRUB,
		IdempotencyKey:    "payrefund:" + refundID.String(),
	})
	if err != nil {
		t.Fatalf("duplicate refund: %v", err)
	}
	if refundAgain.ProviderRefundID != refund.ProviderRefundID {
		t.Fatalf("duplicate refund id = %q, want %q", refundAgain.ProviderRefundID, refund.ProviderRefundID)
	}
}

func TestCancelPayment(t *testing.T) {
	provider := mock.New()
	ctx := context.Background()
	intentID := uuid.New()
	created, err := provider.CreatePayment(ctx, domain.CreatePaymentInput{
		IntentID:       intentID,
		UserID:         uuid.New(),
		Amount:         10000,
		Currency:       domain.CurrencyRUB,
		Credits:        100,
		IdempotencyKey: "pay:" + intentID.String(),
	})
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if err := provider.CancelPayment(ctx, created.ProviderPaymentID); err != nil {
		t.Fatalf("cancel payment: %v", err)
	}
	payment, err := provider.GetPayment(ctx, created.ProviderPaymentID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if payment.Status != domain.PaymentIntentCanceled {
		t.Fatalf("payment status = %q, want canceled", payment.Status)
	}
}

func TestParseWebhookDedupKeyUsesRefundIDWhenPresent(t *testing.T) {
	provider := mock.New()
	event, err := provider.ParseWebhook(
		context.Background(),
		[]byte(`{"event_type":"refund.succeeded","provider_payment_id":"pay-1","provider_refund_id":"refund-1"}`),
		http.Header{},
	)
	if err != nil {
		t.Fatalf("parse webhook: %v", err)
	}
	if event.Provider != domain.PaymentProviderMock {
		t.Fatalf("provider = %q, want mock", event.Provider)
	}
	if event.DedupKey != "webhook:mock:refund.succeeded:refund-1" {
		t.Fatalf("dedup key = %q", event.DedupKey)
	}
	if event.ProviderPaymentID != "pay-1" || event.ProviderRefundID != "refund-1" {
		t.Fatalf("unexpected ids: payment=%q refund=%q", event.ProviderPaymentID, event.ProviderRefundID)
	}
}
