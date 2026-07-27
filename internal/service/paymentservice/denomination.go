package paymentservice

import (
	"vk-ai-aggregator/internal/domain"
)

func currentIntentCredits(intent *domain.PaymentIntent) (int64, error) {
	if intent == nil {
		return 0, ErrInvalidInput
	}
	return intent.CurrentCredits()
}
