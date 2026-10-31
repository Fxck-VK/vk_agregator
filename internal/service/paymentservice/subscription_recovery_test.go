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
	"vk-ai-aggregator/internal/service/economycatalog"
)

type recoveryProvider struct {
	domain.PaymentProvider
	verified domain.ProviderPayment
	reads    int
}

func (p *recoveryProvider) Code() domain.PaymentProviderCode { return domain.PaymentProviderYooKassa }
func (p *recoveryProvider) GetPayment(_ context.Context, id string) (domain.ProviderPayment, error) {
	p.reads++
	if id != p.verified.ProviderPaymentID {
		return domain.ProviderPayment{}, domain.ErrNotFound
	}
	return p.verified, nil
}

type recoveryFixture struct {
	processor *WebhookProcessor
	provider  *recoveryProvider
	payments  *memory.PaymentRepo
	credits   *memory.BillingRepo
	intent    *domain.PaymentIntent
	event     *domain.PaymentEvent
	owner     uuid.UUID
}

func newRecoveryFixture(t *testing.T) recoveryFixture {
	t.Helper()
	ctx := context.Background()
	payments, credits := memory.NewPaymentRepo(), memory.NewBillingRepo()
	owner, intentID := uuid.New(), uuid.New()
	lite, _ := economycatalog.FindPlan("lite")
	start, _ := economycatalog.FindPlan("start")
	metadata, err := json.Marshal(map[string]any{"source": "web", "subscription_submit_started_at": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
		"subscription": subscriptionPurchase{Plan: planSnapshot(lite), Next: planSnapshot(start), AutoRenew: true}})
	if err != nil {
		t.Fatal(err)
	}
	intent := &domain.PaymentIntent{ID: intentID, AccountID: owner, Status: domain.PaymentIntentCreated, Provider: domain.PaymentProviderYooKassa,
		Amount: 19900, Currency: domain.CurrencyRUB, Credits: 400, CreditDenominationVersion: 2, PriceVersion: economycatalog.Version, IdempotencyKey: "recovery:" + intentID.String(), Metadata: metadata}
	if err := payments.CreateIntent(ctx, intent); err != nil {
		t.Fatal(err)
	}
	if err := credits.SaveSubscription(ctx, &domain.AccountSubscription{AccountID: owner, PendingIntentID: intentID}); err != nil {
		t.Fatal(err)
	}
	provider := &recoveryProvider{PaymentProvider: paymentmock.New(), verified: domain.ProviderPayment{ProviderPaymentID: "accepted-no-local-binding", Status: domain.PaymentIntentSucceeded,
		Amount: 19900, Currency: domain.CurrencyRUB, Paid: true, Captured: true, VerifiedIntentID: intentID, VerifiedAccountID: owner, PaymentMethodSaved: true, PaymentMethodID: "verified-saved-method"}}
	tx := TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return fn(ctx, payments, credits)
	})
	processor := NewWebhookProcessor(payments, provider, billingservice.New(credits), tx)
	// Notification metadata deliberately disagrees. Only authenticated GetPayment
	// fields can establish a lost binding.
	event := &domain.PaymentEvent{Provider: domain.PaymentProviderYooKassa, EventType: "payment.succeeded", ProviderPaymentID: provider.verified.ProviderPaymentID,
		DedupKey: uuid.NewString(), Payload: json.RawMessage(`{"metadata":{"intent_id":"untrusted","account_id":"untrusted"}}`)}
	if _, err := payments.CreateEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	return recoveryFixture{processor, provider, payments, credits, intent, event, owner}
}

func TestSubscriptionRecoveryUsesVerifiedMetadataAndGrantsOnce(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := f.processor.ProcessEvent(ctx, f.event); err != nil {
			t.Fatal(err)
		}
	}
	duplicate := *f.event
	duplicate.ID = uuid.New()
	duplicate.DedupKey = uuid.NewString()
	if _, err := f.payments.CreateEvent(ctx, &duplicate); err != nil {
		t.Fatal(err)
	}
	if err := f.processor.ProcessEvent(ctx, &duplicate); err != nil {
		t.Fatal(err)
	}
	intent, err := f.payments.GetIntentByID(ctx, f.intent.ID)
	if err != nil || intent.ProviderPaymentID != f.provider.verified.ProviderPaymentID || intent.Status != domain.PaymentIntentSucceeded {
		t.Fatalf("binding/settlement failed: %+v %v", intent, err)
	}
	account, err := f.credits.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits)
	if err != nil || account.BalanceCached != 460 {
		t.Fatalf("expected one400-star grant plus60 bonus: %+v %v", account, err)
	}
	state, err := f.credits.GetSubscription(ctx, f.owner)
	if err != nil || state.ActiveIntentID != f.intent.ID || state.PendingIntentID != uuid.Nil || !state.TrialUsed {
		t.Fatalf("wrong subscription settlement: %+v %v", state, err)
	}
	lots, err := f.credits.CreditLotBalancesForOwner(ctx, f.owner, time.Now().UTC())
	count := 0
	for _, lot := range lots {
		if lot.Kind == domain.CreditLotSubscription {
			count++
			if lot.ExpiresAt == nil || !lot.ExpiresAt.Equal(state.PaidUntil) {
				t.Fatal("recovered grant expiry differs")
			}
		}
	}
	if err != nil || count != 1 {
		t.Fatalf("subscription lot count=%d %v", count, err)
	}
	if f.provider.reads < 1 {
		t.Fatal("recovery did not fetch provider state")
	}
}

func TestSubscriptionRecoveryRejectsUntrustedOrMismatchedBindings(t *testing.T) {
	changeMetadata := func(t *testing.T, f recoveryFixture, key string, value any) {
		t.Helper()
		var raw map[string]any
		if err := json.Unmarshal(f.intent.Metadata, &raw); err != nil {
			t.Fatal(err)
		}
		if value == nil {
			delete(raw, key)
		} else {
			raw[key] = value
		}
		data, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.payments.UpdateIntentMetadata(context.Background(), f.intent.ID, data); err != nil {
			t.Fatal(err)
		}
	}
	cases := map[string]func(*testing.T, recoveryFixture){
		"wrong_owner":  func(_ *testing.T, f recoveryFixture) { f.provider.verified.VerifiedAccountID = uuid.New() },
		"wrong_intent": func(_ *testing.T, f recoveryFixture) { f.provider.verified.VerifiedIntentID = uuid.New() },
		"missing_verified_metadata": func(_ *testing.T, f recoveryFixture) {
			f.provider.verified.VerifiedIntentID = uuid.Nil
			f.provider.verified.VerifiedAccountID = uuid.Nil
		},
		"wrong_amount":   func(_ *testing.T, f recoveryFixture) { f.provider.verified.Amount++ },
		"wrong_currency": func(_ *testing.T, f recoveryFixture) { f.provider.verified.Currency = domain.CurrencyCredits },
		"unpaid":         func(_ *testing.T, f recoveryFixture) { f.provider.verified.Paid = false },
		"uncaptured":     func(_ *testing.T, f recoveryFixture) { f.provider.verified.Captured = false },
		"missing_marker": func(t *testing.T, f recoveryFixture) { changeMetadata(t, f, "subscription_submit_started_at", nil) },
		"invalid_marker": func(t *testing.T, f recoveryFixture) {
			changeMetadata(t, f, "subscription_submit_started_at", "not-a-date")
		},
		"not_subscription": func(t *testing.T, f recoveryFixture) { changeMetadata(t, f, "subscription", nil) },
		"terminal_intent": func(t *testing.T, f recoveryFixture) {
			if err := f.payments.SetIntentProviderState(context.Background(), f.intent.ID, domain.PaymentIntentCanceled, "", ""); err != nil {
				t.Fatal(err)
			}
		},
		"other_provider_binding": func(t *testing.T, f recoveryFixture) {
			if err := f.payments.SetIntentProviderState(context.Background(), f.intent.ID, domain.PaymentIntentCreated, "another-provider-payment", ""); err != nil {
				t.Fatal(err)
			}
		},
		"not_pending": func(t *testing.T, f recoveryFixture) {
			state, _ := f.credits.GetSubscription(context.Background(), f.owner)
			state.PendingIntentID = uuid.New()
			if err := f.credits.SaveSubscription(context.Background(), state); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRecoveryFixture(t)
			change(t, f)
			if err := f.processor.ProcessEvent(context.Background(), f.event); err == nil {
				t.Fatal("unverified binding accepted")
			}
			intent, _ := f.payments.GetIntentByID(context.Background(), f.intent.ID)
			if intent.ProviderPaymentID == f.provider.verified.ProviderPaymentID {
				t.Fatal("mismatch bound provider id")
			}
			if _, err := f.credits.GetAccountByOwner(context.Background(), f.owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("rejected recovery granted balance: %v", err)
			}
			event, _ := f.payments.GetEventByID(context.Background(), f.event.ID)
			if event.ProcessedAt != nil {
				t.Fatal("rejected recovery acknowledged")
			}
		})
	}
}

func TestSubscriptionRecoveryHonorsCancellationBeforeLateVerifiedSuccess(t *testing.T) {
	f := newRecoveryFixture(t)
	ctx := context.Background()
	state, _ := f.credits.GetSubscription(ctx, f.owner)
	now := time.Now().UTC()
	state.CancelledAt = &now
	if err := f.credits.SaveSubscription(ctx, state); err != nil {
		t.Fatal(err)
	}
	if err := f.processor.ProcessEvent(ctx, f.event); err != nil {
		t.Fatal(err)
	}
	state, _ = f.credits.GetSubscription(ctx, f.owner)
	if state.PaidUntil.IsZero() || state.AutoRenew || state.PaymentMethodID != "" || state.CancelledAt == nil {
		t.Fatalf("late recovery lost paid period or cancellation: %+v", state)
	}
}

func TestSubscriptionRecoveryAcceptsVerifiedUnpaidTerminalEvents(t *testing.T) {
	for _, target := range []domain.PaymentIntentStatus{domain.PaymentIntentCanceled, domain.PaymentIntentFailed} {
		for _, renewal := range []bool{false, true} {
			name := string(target) + "/initial"
			if renewal {
				name = string(target) + "/renewal"
			}
			t.Run(name, func(t *testing.T) {
				f := newRecoveryFixture(t)
				ctx := context.Background()
				if renewal {
					start, _ := economycatalog.FindPlan("start")
					lite, _ := economycatalog.FindPlan("lite")
					previous := uuid.New()
					metadata, err := json.Marshal(map[string]any{"source": "web", subscriptionSubmitMarker: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
						"subscription": subscriptionPurchase{Plan: planSnapshot(start), Next: planSnapshot(start), AutoRenew: true, Renewal: true, PreviousIntentID: previous}})
					if err != nil {
						t.Fatal(err)
					}
					f.intent = &domain.PaymentIntent{ID: uuid.New(), AccountID: f.owner, Status: domain.PaymentIntentCreated, Provider: domain.PaymentProviderYooKassa,
						Amount: start.Amount, Currency: domain.CurrencyRUB, Credits: start.Stars, CreditDenominationVersion: 2, PriceVersion: economycatalog.Version, IdempotencyKey: uuid.NewString(), Metadata: metadata}
					if err := f.payments.CreateIntent(ctx, f.intent); err != nil {
						t.Fatal(err)
					}
					state, _ := f.credits.GetSubscription(ctx, f.owner)
					state.Plan, state.NextPlan = planSnapshot(lite), planSnapshot(start)
					state.TrialUsed, state.AutoRenew = true, true
					state.ActiveIntentID, state.PendingIntentID = previous, f.intent.ID
					state.PaymentMethodID = "saved-before-cancellation"
					state.PeriodStart, state.PaidUntil = time.Now().UTC().AddDate(0, 0, -8), time.Now().UTC().AddDate(0, 0, -1)
					if err := f.credits.SaveSubscription(ctx, state); err != nil {
						t.Fatal(err)
					}
					f.provider.verified.VerifiedIntentID, f.provider.verified.Amount = f.intent.ID, start.Amount
				}
				tx := TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
					return fn(ctx, f.payments, f.credits)
				})
				svc := New(f.payments, f.provider, Config{AccountReturnURL: "https://epifera.test/return", SubscriptionStore: f.credits, SubscriptionBilling: f.credits, SubscriptionTx: tx})
				cancelled, err := svc.CancelSubscription(ctx, f.owner)
				if err != nil {
					t.Fatal(err)
				}
				f.provider.verified.Status, f.provider.verified.Paid, f.provider.verified.Captured = target, false, false
				// The stale notification still claims success. Authenticated GET
				// establishes the terminal outcome and is the only recovery source.
				for i := 0; i < 2; i++ {
					if err := f.processor.ProcessEvent(ctx, f.event); err != nil {
						t.Fatal(err)
					}
				}
				intent, _ := f.payments.GetIntentByID(ctx, f.intent.ID)
				if intent.Status != target || intent.ProviderPaymentID != f.provider.verified.ProviderPaymentID {
					t.Fatalf("terminal binding not restored: %+v", intent)
				}
				state, _ := f.credits.GetSubscription(ctx, f.owner)
				if state.CancelledAt == nil || !state.CancelledAt.Equal(*cancelled.CancelledAt) || state.TrialUsed != renewal || state.AutoRenew || state.PaymentMethodID != "" || state.ActiveIntentID != cancelled.ActiveIntentID || !state.PaidUntil.Equal(cancelled.PaidUntil) {
					t.Fatalf("terminal recovery changed cancellation/trial/paid access: %+v", state)
				}
				if _, err := f.credits.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("terminal recovery granted credits: %v", err)
				}
				if renewal {
					_, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: f.owner, PlanCode: "lite", ReceiptEmail: "buyer@example.test", IdempotencyKey: uuid.NewString()})
					if !errors.Is(err, ErrTrialAlreadyUsed) {
						t.Fatalf("failed renewal reset used trial: %v", err)
					}
				}
				plan := "lite"
				if renewal {
					plan = "start"
				}
				fresh, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{AccountID: f.owner, PlanCode: plan, ReceiptEmail: "buyer@example.test", IdempotencyKey: uuid.NewString()})
				if err != nil || !fresh.Created || fresh.Intent.ID == f.intent.ID {
					t.Fatalf("terminal ambiguous payment blocked fresh checkout: %+v %v", fresh, err)
				}
				duplicate := *f.event
				duplicate.ID = uuid.New()
				duplicate.DedupKey = uuid.NewString()
				if _, err := f.payments.CreateEvent(ctx, &duplicate); err != nil {
					t.Fatal(err)
				}
				if err := f.processor.ProcessEvent(ctx, &duplicate); err != nil {
					t.Fatal(err)
				}
				state, _ = f.credits.GetSubscription(ctx, f.owner)
				if state.PendingIntentID != fresh.Intent.ID || state.TrialUsed != renewal || state.AutoRenew {
					t.Fatal("late duplicate changed the fresh checkout")
				}
				if _, err := f.credits.GetAccountByOwner(ctx, f.owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("late terminal duplicate granted credits: %v", err)
				}
			})
		}
	}
}

func TestSubscriptionRecoveryRejectsUnfinishedOrContradictoryTerminalResults(t *testing.T) {
	cases := []struct {
		name           string
		status         domain.PaymentIntentStatus
		paid, captured bool
	}{
		{"pending", domain.PaymentIntentProviderPending, false, false},
		{"waiting", domain.PaymentIntentWaitingForUser, false, false},
		{"cancelled_but_paid", domain.PaymentIntentCanceled, true, false},
		{"failed_but_captured", domain.PaymentIntentFailed, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecoveryFixture(t)
			f.provider.verified.Status, f.provider.verified.Paid, f.provider.verified.Captured = tc.status, tc.paid, tc.captured
			if err := f.processor.ProcessEvent(context.Background(), f.event); err == nil {
				t.Fatal("unsupported result recovered")
			}
			intent, _ := f.payments.GetIntentByID(context.Background(), f.intent.ID)
			if intent.ProviderPaymentID != "" || intent.Status != domain.PaymentIntentCreated {
				t.Fatal("unsupported result mutated binding")
			}
			if _, err := f.credits.GetAccountByOwner(context.Background(), f.owner, domain.CurrencyCredits); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("unsupported result granted credits: %v", err)
			}
		})
	}
}
