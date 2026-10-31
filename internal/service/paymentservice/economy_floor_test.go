package paymentservice

import (
	"testing"
	"vk-ai-aggregator/internal/domain"
)

func TestActiveTopUpsCannotUndercutEconomyIncomeFloor(t *testing.T) {
	for _, tc := range []struct {
		amount        int64
		active, valid bool
	}{{43920, true, true}, {43919, true, false}, {50000, true, true}, {1, false, true}} {
		p := &domain.PaymentProduct{Code: "economic_pack", Title: "Stars", Amount: tc.amount, Currency: domain.CurrencyRUB, Credits: 1000, CreditDenominationVersion: domain.CurrentCreditDenominationVersion, PriceVersion: 1, PaymentSubject: "service", PaymentMode: "full_payment", IsActive: tc.active}
		err := validateProduct(p)
		if (err == nil) != tc.valid {
			t.Fatalf("amount%d active%v valid%v err%v", tc.amount, tc.active, tc.valid, err)
		}
	}
}
