package domain

import (
	"math"
	"testing"
)

func TestConvertCreditAmountLegacyCreditsToCurrentStars(t *testing.T) {
	t.Parallel()

	got, err := ConvertCreditAmount(99, LegacyCreditDenominationVersion, CurrentCreditDenominationVersion)
	if err != nil {
		t.Fatalf("convert legacy credits: %v", err)
	}
	if got != 198 {
		t.Fatalf("converted amount = %d, want 198", got)
	}
}

func TestConvertCreditAmountDefaultsMissingVersionToLegacy(t *testing.T) {
	t.Parallel()

	got, err := ConvertCreditAmount(10, 0, CurrentCreditDenominationVersion)
	if err != nil {
		t.Fatalf("convert missing version: %v", err)
	}
	if got != 20 {
		t.Fatalf("converted amount = %d, want 20", got)
	}
}

func TestConvertCreditAmountPreservesSignedLedgerAmounts(t *testing.T) {
	t.Parallel()

	got, err := ConvertCreditAmount(-99, LegacyCreditDenominationVersion, CurrentCreditDenominationVersion)
	if err != nil {
		t.Fatalf("convert debit: %v", err)
	}
	if got != -198 {
		t.Fatalf("converted debit = %d, want -198", got)
	}
}

func TestConvertCreditAmountRejectsLossyAndOverflowingConversions(t *testing.T) {
	t.Parallel()

	if _, err := ConvertCreditAmount(3, CurrentCreditDenominationVersion, LegacyCreditDenominationVersion); err == nil {
		t.Fatal("expected lossy conversion to fail")
	}
	if _, err := ConvertCreditAmount(math.MaxInt64, LegacyCreditDenominationVersion, CurrentCreditDenominationVersion); err == nil {
		t.Fatal("expected overflowing conversion to fail")
	}
}

func TestStarDenominationUsesIntegerKopecks(t *testing.T) {
	t.Parallel()

	if StarKopecks != 50 {
		t.Fatalf("StarKopecks = %d, want 50", StarKopecks)
	}
	if StarsPerLegacyCredit != 2 {
		t.Fatalf("StarsPerLegacyCredit = %d, want 2", StarsPerLegacyCredit)
	}
}
