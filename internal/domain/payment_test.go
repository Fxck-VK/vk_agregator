package domain

import "testing"

func TestPaymentIntentStatusTransitions(t *testing.T) {
	if !PaymentIntentCreated.CanTransitionTo(PaymentIntentProviderPending) {
		t.Fatal("created should transition to provider_pending")
	}
	if !PaymentIntentWaitingForUser.CanTransitionTo(PaymentIntentSucceeded) {
		t.Fatal("waiting_for_user should transition to succeeded")
	}
	if !PaymentIntentWaitingForUser.CanTransitionTo(PaymentIntentProviderPending) {
		t.Fatal("waiting_for_user should transition to provider_pending")
	}
	if !PaymentIntentSucceeded.CanTransitionTo(PaymentIntentPartiallyRefunded) {
		t.Fatal("succeeded should transition to partially_refunded")
	}
	if !PaymentIntentPartiallyRefunded.CanTransitionTo(PaymentIntentRefunded) {
		t.Fatal("partially_refunded should transition to refunded")
	}
	if PaymentIntentSucceeded.CanTransitionTo(PaymentIntentCanceled) {
		t.Fatal("succeeded must not roll back to canceled")
	}
	if !PaymentIntentCanceled.IsTerminal() {
		t.Fatal("canceled should be terminal")
	}
	if !PaymentIntentRefunded.IsTerminal() {
		t.Fatal("refunded should be terminal")
	}
	if !PaymentIntentStatus("created").Valid() {
		t.Fatal("created should be valid")
	}
	if PaymentIntentStatus("unknown").Valid() {
		t.Fatal("unknown should be invalid")
	}
}

func TestPaymentProviderCodeValid(t *testing.T) {
	if !PaymentProviderMock.Valid() || !PaymentProviderYooKassa.Valid() {
		t.Fatal("known payment providers should be valid")
	}
	if PaymentProviderCode("openai").Valid() {
		t.Fatal("AI provider code must not be a valid payment provider")
	}
}
