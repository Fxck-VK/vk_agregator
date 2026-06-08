// Package payment constructs payment provider adapters from runtime config.
package payment

import (
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/payment/yookassa"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
)

// NewProvider builds the configured payment provider adapter.
func NewProvider(cfg config.Config) (domain.PaymentProvider, error) {
	switch domain.PaymentProviderCode(strings.ToLower(strings.TrimSpace(cfg.PaymentProvider))) {
	case "", domain.PaymentProviderMock:
		return mock.New(), nil
	case domain.PaymentProviderYooKassa:
		return yookassa.New(yookassa.Config{
			ShopID:    cfg.YooKassaShopID,
			SecretKey: cfg.YooKassaSecretKey,
			BaseURL:   cfg.YooKassaBaseURL,
			ReturnURL: cfg.YooKassaReturnURL,
		})
	default:
		return nil, fmt.Errorf("payment: unsupported provider %q", cfg.PaymentProvider)
	}
}
