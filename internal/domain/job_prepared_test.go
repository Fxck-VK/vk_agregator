package domain

import "testing"

func TestPreparedJobStatusIsValidNonActiveAndNonTerminal(t *testing.T) {
	if !JobStatusPrepared.Valid() {
		t.Fatal("prepared status must be valid")
	}
	if JobStatusPrepared.IsActiveWork() {
		t.Fatal("prepared status must not count as active work")
	}
	if JobStatusPrepared.IsTerminal() {
		t.Fatal("prepared status must remain activatable")
	}
	if !JobStatusPrepared.CanTransitionTo(JobStatusValidated) {
		t.Fatal("prepared status must have an explicit future activation transition")
	}
}

func TestPreparedConfirmationExpiryUsesStableSafeErrorDetails(t *testing.T) {
	if PreparedConfirmationExpiredCode != "prepared_confirmation_expired" {
		t.Fatalf("confirmation expiry code = %q", PreparedConfirmationExpiredCode)
	}
	if PreparedConfirmationExpiredMessage != "image generation confirmation expired" {
		t.Fatalf("confirmation expiry message = %q", PreparedConfirmationExpiredMessage)
	}
}
