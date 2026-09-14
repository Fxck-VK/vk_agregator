package api

import (
	"testing"
	"vk-ai-aggregator/internal/platform/config"
)

func TestWebTestPaymentsRequireTestShopAndSafeReturnOrigin(t *testing.T) {
	cfg := &config.Config{Env: "staging", PaymentProvider: "yookassa", YooKassaShopID: "synthetic-shop", YooKassaSecretKey: "test_synthetic", WebOrigin: "https://dev-web.example.test"}
	if !WebTestPaymentsEnabled(cfg) || WebPaymentReturnURL(cfg) != "https://dev-web.example.test/app/payment-return" {
		t.Fatal("valid test config disabled")
	}
	for _, change := range []func(*config.Config){
		func(c *config.Config) { c.Env = "production" },
		func(c *config.Config) { c.YooKassaSecretKey = "live_synthetic" },
		func(c *config.Config) { c.YooKassaShopID = "" },
		func(c *config.Config) { c.PaymentProvider = "mock" },
		func(c *config.Config) { c.WebOrigin = "https://user:pass@example.test" },
		func(c *config.Config) { c.WebOrigin = "http://example.test" },
	} {
		copy := *cfg
		change(&copy)
		if WebTestPaymentsEnabled(&copy) {
			t.Fatal("unsafe/non-test config enabled")
		}
	}
}
