package payment_test

import (
	"strings"
	"testing"

	paymentadapter "vk-ai-aggregator/internal/adapter/payment"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
)

func TestNewProviderDefaultsToMock(t *testing.T) {
	provider, err := paymentadapter.NewProvider(config.Config{})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider.Code() != domain.PaymentProviderMock {
		t.Fatalf("provider code = %q, want mock", provider.Code())
	}
}

func TestNewProviderYooKassa(t *testing.T) {
	provider, err := paymentadapter.NewProvider(config.Config{
		PaymentProvider:   "yookassa",
		YooKassaShopID:    "shop-1",
		YooKassaSecretKey: "secret",
		YooKassaBaseURL:   "https://example.com/v3",
		YooKassaReturnURL: "https://neiirohub.ru/payments/return",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	if provider.Code() != domain.PaymentProviderYooKassa {
		t.Fatalf("provider code = %q, want yookassa", provider.Code())
	}
}

func TestNewProviderYooKassaRequiresConfig(t *testing.T) {
	_, err := paymentadapter.NewProvider(config.Config{PaymentProvider: "yookassa"})
	if err == nil || !strings.Contains(err.Error(), "shop id") {
		t.Fatalf("expected yookassa config error, got %v", err)
	}
}

func TestNewProviderRejectsUnknownProvider(t *testing.T) {
	_, err := paymentadapter.NewProvider(config.Config{PaymentProvider: "stripe"})
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
