package paymentservice

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
)

// Stage evidence in the same transaction callback as the principal changes;
// failed callbacks discard it, just as the PostgreSQL implementation must.
type referralEvidenceRecorder struct {
	domain.PaymentRepository
	payments []domain.WebReferralPaymentEvidence
	refunds  []domain.WebReferralRefundEvidence
	fail     error
}

func (r *referralEvidenceRecorder) RecordWebReferralPayment(_ context.Context, proof domain.WebReferralPaymentEvidence) error {
	if r.fail != nil {
		return r.fail
	}
	r.payments = append(r.payments, proof)
	return nil
}
func (r *referralEvidenceRecorder) RecordWebReferralRefund(_ context.Context, proof domain.WebReferralRefundEvidence) error {
	if r.fail != nil {
		return r.fail
	}
	r.refunds = append(r.refunds, proof)
	return nil
}

func referralEvidenceTx(repo *memory.PaymentRepo, credits *memory.BillingRepo, recorded *referralEvidenceRecorder) PaymentTxRunner {
	return TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		staged := &referralEvidenceRecorder{PaymentRepository: repo, fail: recorded.fail}
		if err := fn(ctx, staged, credits); err != nil {
			return err
		}
		recorded.payments = append(recorded.payments, staged.payments...)
		recorded.refunds = append(recorded.refunds, staged.refunds...)
		return nil
	})
}

type referralEvidenceProvider struct {
	domain.PaymentProvider
	code        domain.PaymentProviderCode
	payment     domain.ProviderPayment
	refund      domain.VerifiedPaymentRefund
	refundCalls int
}

func (p *referralEvidenceProvider) Code() domain.PaymentProviderCode { return p.code }
func (p *referralEvidenceProvider) GetPayment(context.Context, string) (domain.ProviderPayment, error) {
	return p.payment, nil
}
func (p *referralEvidenceProvider) GetRefund(context.Context, string) (domain.VerifiedPaymentRefund, error) {
	return p.refund, nil
}
func (p *referralEvidenceProvider) CreateRefund(_ context.Context, in domain.CreateRefundInput) (domain.RefundResult, error) {
	p.refundCalls++
	return domain.RefundResult{ProviderRefundID: "manual-refund", Amount: in.Amount, Currency: in.Currency, Status: domain.PaymentRefundSucceeded}, nil
}

func referralEvidenceFixture(t *testing.T) (*WebhookProcessor, *memory.PaymentRepo, *memory.BillingRepo, *referralEvidenceRecorder, *referralEvidenceProvider, *domain.PaymentIntent) {
	t.Helper()
	repo, credits := memory.NewPaymentRepo(), memory.NewBillingRepo()
	intent := &domain.PaymentIntent{ID: uuid.New(), AccountID: uuid.New(), Provider: domain.PaymentProviderYooKassa, ProviderPaymentID: "verified-payment", Status: domain.PaymentIntentWaitingForUser, Amount: 10000, Credits: 100, CreditDenominationVersion: domain.CurrentCreditDenominationVersion, Currency: domain.CurrencyRUB, IdempotencyKey: uuid.NewString(), Metadata: json.RawMessage(`{"source":"web","test":false}`)}
	if err := repo.CreateIntent(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	provider := &referralEvidenceProvider{PaymentProvider: paymentmock.New(), code: domain.PaymentProviderYooKassa, payment: domain.ProviderPayment{ProviderPaymentID: intent.ProviderPaymentID, Status: domain.PaymentIntentSucceeded, Paid: true, Captured: true, Amount: intent.Amount, Currency: intent.Currency, Raw: json.RawMessage(`{"test":false}`)}}
	recorded := &referralEvidenceRecorder{}
	processor := NewWebhookProcessor(repo, provider, billingservice.New(credits, billingservice.WithStartingBalance(0)), referralEvidenceTx(repo, credits, recorded))
	processor.now = func() time.Time { return time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC) }
	return processor, repo, credits, recorded, provider, intent
}

func TestWebReferralPaymentEvidenceUsesAuthenticatedMode(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      domain.WebReferralPaymentMode
	}{
		{"live", `{"test":false}`, domain.WebReferralPaymentLive},
		{"test", `{"test":true}`, domain.WebReferralPaymentTest},
		{"missing", `{}`, domain.WebReferralPaymentUnknown},
		{"null", `{"test":null}`, domain.WebReferralPaymentUnknown},
		{"wrong-type", `{"test":"false"}`, domain.WebReferralPaymentUnknown},
		{"malformed", `{`, domain.WebReferralPaymentUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			processor, _, credits, recorded, provider, intent := referralEvidenceFixture(t)
			provider.payment.Raw = json.RawMessage(tc.raw)
			if _, err := processor.SyncIntent(context.Background(), intent.ID); err != nil {
				t.Fatal(err)
			}
			if len(recorded.payments) != 1 {
				t.Fatalf("settlement evidence count = %d, want 1", len(recorded.payments))
			}
			proof := recorded.payments[0]
			if proof.Mode != tc.want || proof.IntentID != intent.ID || proof.AccountID != intent.AccountID || proof.Amount != intent.Amount || proof.Currency != string(intent.Currency) || proof.ProviderPaymentID != intent.ProviderPaymentID || proof.Provider != string(intent.Provider) || !proof.VerifiedAt.Equal(processor.now()) {
				t.Fatalf("incorrect verified proof: %+v", proof)
			}
			account, err := credits.GetAccountByOwner(context.Background(), intent.AccountID, domain.CurrencyCredits)
			if err != nil || account.BalanceCached != intent.Credits {
				t.Fatalf("evidence without settlement: %+v %v", account, err)
			}
		})
	}
}

func TestWebReferralPaymentEvidenceExcludesUnverifiedAndMock(t *testing.T) {
	for _, tc := range []string{"unpaid", "not-captured", "amount-mismatch", "mock", "legacy-owner"} {
		t.Run(tc, func(t *testing.T) {
			processor, repo, credits, recorded, provider, intent := referralEvidenceFixture(t)
			switch tc {
			case "unpaid":
				provider.payment.Paid = false
			case "not-captured":
				provider.payment.Captured = false
			case "amount-mismatch":
				provider.payment.Amount++
			case "mock":
				intent.Provider = domain.PaymentProviderMock
				provider.code = domain.PaymentProviderMock
			case "legacy-owner":
				intent.UserID, intent.AccountID = uuid.New(), uuid.Nil
			}
			wrapped := &referralEvidenceRecorder{PaymentRepository: repo}
			_, _ = processor.applyProviderPayment(context.Background(), wrapped, credits, intent, provider.payment)
			recorded.payments = append(recorded.payments, wrapped.payments...)
			if len(recorded.payments) != 0 {
				t.Fatal("ineligible payment produced proof")
			}
		})
	}
}

func TestWebReferralRefundEvidenceSurvivesUnsupportedPrincipalRefund(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		t.Run(map[bool]string{false: "topup", true: "partial-subscription"}[subscription], func(t *testing.T) {
			processor, repo, _, recorded, provider, intent := referralEvidenceFixture(t)
			if subscription {
				// Existing subscription principal processing refuses partial refunds.
				plan := domain.SubscriptionPlanSnapshot{Code: "start", Amount: intent.Amount, Stars: intent.Credits, PeriodDays: 30, PolicyVersion: 1}
				intent.Metadata, _ = json.Marshal(map[string]any{"subscription": subscriptionPurchase{Plan: plan, Next: plan}})
				// Store an immutable subscription intent rather than alter existing data.
				intent.ID, intent.IdempotencyKey, intent.ProviderPaymentID = uuid.New(), uuid.NewString(), "partial-sub-payment"
				if err := repo.CreateIntent(context.Background(), intent); err != nil {
					t.Fatal(err)
				}
			}
			provider.refund = domain.VerifiedPaymentRefund{ID: "external-refund", PaymentID: intent.ProviderPaymentID, Status: domain.PaymentRefundSucceeded, Amount: 100, Currency: intent.Currency}
			event := &domain.PaymentEvent{Provider: intent.Provider, ProviderPaymentID: intent.ProviderPaymentID, ProviderRefundID: provider.refund.ID, EventType: "refund.succeeded", DedupKey: uuid.NewString()}
			if _, err := repo.CreateEvent(context.Background(), event); err != nil {
				t.Fatal(err)
			}
			err := processor.ProcessEvent(context.Background(), event)
			if subscription && !errors.Is(err, ErrWebhookUnsupported) {
				t.Fatalf("partial principal refund = %v", err)
			}
			if !subscription && err != nil {
				t.Fatal(err)
			}
			if len(recorded.refunds) != 1 {
				t.Fatalf("durable refund proof count = %d, want 1", len(recorded.refunds))
			}
			proof := recorded.refunds[0]
			if proof.IntentID != intent.ID || proof.ProviderRefundID != provider.refund.ID || proof.ProviderPaymentID != intent.ProviderPaymentID || proof.Amount != 100 || proof.Currency != string(intent.Currency) || !proof.VerifiedAt.Equal(processor.now()) {
				t.Fatalf("incorrect refund proof: %+v", proof)
			}
		})
	}
}

func TestWebReferralManualRefundEvidenceAndReplay(t *testing.T) {
	processor, _, _, recorded, provider, intent := referralEvidenceFixture(t)
	ctx := context.Background()
	if _, err := processor.SyncIntent(ctx, intent.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := processor.RefundIntent(ctx, RefundIntentInput{IntentID: intent.ID, IdempotencyKey: "manual-referral-refund"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(recorded.refunds) != 1 || provider.refundCalls != 1 {
		t.Fatalf("refund proof/calls = %d/%d, want 1/1", len(recorded.refunds), provider.refundCalls)
	}
}

func TestWebReferralPaymentEvidenceSubscriptionAndCanonicalDualOwner(t *testing.T) {
	for _, subscription := range []bool{false, true} {
		t.Run(map[bool]string{false: "dual-owner-topup", true: "subscription"}[subscription], func(t *testing.T) {
			processor, repo, credits, recorded, provider, original := referralEvidenceFixture(t)
			intent := *original
			intent.ID, intent.IdempotencyKey, intent.ProviderPaymentID = uuid.New(), uuid.NewString(), "second-payment"
			intent.UserID = uuid.New()
			if subscription {
				plan := domain.SubscriptionPlanSnapshot{Code: "start", Amount: intent.Amount, Stars: intent.Credits, PeriodDays: 30, PolicyVersion: 1}
				intent.Metadata, _ = json.Marshal(map[string]any{"subscription": subscriptionPurchase{Plan: plan, Next: plan}})
			}
			if err := repo.CreateIntent(context.Background(), &intent); err != nil {
				t.Fatal(err)
			}
			if subscription {
				state, err := credits.LockSubscription(context.Background(), intent.AccountID)
				if err != nil {
					t.Fatal(err)
				}
				state.PendingIntentID = intent.ID
				if err := credits.SaveSubscription(context.Background(), state); err != nil {
					t.Fatal(err)
				}
			}
			provider.payment.ProviderPaymentID = intent.ProviderPaymentID
			if _, err := processor.SyncIntent(context.Background(), intent.ID); err != nil {
				t.Fatal(err)
			}
			if len(recorded.payments) != 1 || recorded.payments[0].AccountID != intent.AccountID {
				t.Fatal("canonical account settlement proof missing")
			}
		})
	}
}

func TestWebReferralPaymentEvidenceWebhookReplay(t *testing.T) {
	processor, repo, _, recorded, _, intent := referralEvidenceFixture(t)
	event := &domain.PaymentEvent{Provider: intent.Provider, ProviderPaymentID: intent.ProviderPaymentID, EventType: "payment.succeeded", DedupKey: uuid.NewString()}
	ctx := context.Background()
	if _, err := repo.CreateEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := processor.ProcessEvent(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if len(recorded.payments) != 1 {
		t.Fatalf("replayed settlement produced %d proofs", len(recorded.payments))
	}
}

func TestWebReferralPaymentEvidenceDoesNotRetroactivelyQualifyOldWriterReplay(t *testing.T) {
	processor, repo, _, recorded, _, intent := referralEvidenceFixture(t)
	if err := repo.UpdateIntentStatus(context.Background(), intent.ID, intent.Status, domain.PaymentIntentSucceeded); err != nil {
		t.Fatal(err)
	}
	if _, err := processor.SyncIntent(context.Background(), intent.ID); err != nil {
		t.Fatal(err)
	}
	if len(recorded.payments) != 0 {
		t.Fatal("old settlement replay generated new eligible proof")
	}
}

func TestWebReferralRefundEvidenceRejectsUnverifiedBindingAndMoney(t *testing.T) {
	for _, tc := range []string{"refund-id", "payment-id", "pending", "currency", "zero", "negative", "excessive"} {
		t.Run(tc, func(t *testing.T) {
			processor, repo, _, recorded, provider, intent := referralEvidenceFixture(t)
			provider.refund = domain.VerifiedPaymentRefund{ID: "external-refund", PaymentID: intent.ProviderPaymentID, Status: domain.PaymentRefundSucceeded, Amount: 100, Currency: intent.Currency}
			switch tc {
			case "refund-id":
				provider.refund.ID = "other-refund"
			case "payment-id":
				provider.refund.PaymentID = "other-payment"
			case "pending":
				provider.refund.Status = domain.PaymentRefundProviderPending
			case "currency":
				provider.refund.Currency = domain.CurrencyCredits
			case "zero":
				provider.refund.Amount = 0
			case "negative":
				provider.refund.Amount = -1
			case "excessive":
				provider.refund.Amount = intent.Amount + 1
			}
			event := &domain.PaymentEvent{Provider: intent.Provider, ProviderPaymentID: intent.ProviderPaymentID, ProviderRefundID: "external-refund", EventType: "refund.succeeded", DedupKey: uuid.NewString()}
			ctx := context.Background()
			if _, err := repo.CreateEvent(ctx, event); err != nil {
				t.Fatal(err)
			}
			if err := processor.ProcessEvent(ctx, event); !errors.Is(err, ErrWebhookUnverified) {
				t.Fatalf("unverified refund error = %v", err)
			}
			stored, _ := repo.GetEventByID(ctx, event.ID)
			if len(recorded.refunds) != 0 || stored.ProcessedAt != nil {
				t.Fatal("invalid refund persisted proof or acknowledged event")
			}
		})
	}
}

type referralFailingBilling struct{ domain.BillingRepository }

func (r referralFailingBilling) AppendEntry(context.Context, *domain.LedgerEntry) error {
	return errors.New("ledger unavailable")
}

func TestWebReferralPaymentEvidenceWaitsForLedgerSettlement(t *testing.T) {
	processor, repo, credits, _, provider, intent := referralEvidenceFixture(t)
	recorded := &referralEvidenceRecorder{PaymentRepository: repo}
	_, err := processor.applyProviderPayment(context.Background(), recorded, referralFailingBilling{credits}, intent, provider.payment)
	if err == nil || len(recorded.payments) != 0 {
		t.Fatalf("failed settlement proof count = %d, error = %v", len(recorded.payments), err)
	}
}

func TestWebReferralEvidenceFailurePreventsWebhookAcknowledgement(t *testing.T) {
	for _, refund := range []bool{false, true} {
		t.Run(map[bool]string{false: "payment", true: "refund"}[refund], func(t *testing.T) {
			processor, repo, _, recorded, provider, intent := referralEvidenceFixture(t)
			recorded.fail = errors.New("evidence unavailable")
			event := &domain.PaymentEvent{Provider: intent.Provider, ProviderPaymentID: intent.ProviderPaymentID, EventType: "payment.succeeded", DedupKey: uuid.NewString()}
			if refund {
				event.EventType, event.ProviderRefundID = "refund.succeeded", "external-refund"
				provider.refund = domain.VerifiedPaymentRefund{ID: event.ProviderRefundID, PaymentID: intent.ProviderPaymentID, Status: domain.PaymentRefundSucceeded, Amount: 100, Currency: intent.Currency}
			}
			ctx := context.Background()
			if _, err := repo.CreateEvent(ctx, event); err != nil {
				t.Fatal(err)
			}
			if err := processor.ProcessEvent(ctx, event); !errors.Is(err, recorded.fail) {
				t.Fatalf("evidence error lost: %v", err)
			}
			stored, _ := repo.GetEventByID(ctx, event.ID)
			if len(recorded.payments) != 0 || len(recorded.refunds) != 0 || stored.ProcessedAt != nil {
				t.Fatal("failed evidence transaction committed or acknowledged event")
			}
		})
	}
}
