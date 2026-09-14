package api

import (
	"net/url"
	"strings"
	"vk-ai-aggregator/internal/platform/config"
)

// WebTestPaymentsEnabled deliberately excludes live and mock checkouts from
// the first web rollout. Existing VK payment configuration is unaffected.
func WebTestPaymentsEnabled(cfg *config.Config) bool {
	return cfg != nil && (cfg.Env == "development" || cfg.Env == "staging") &&
		strings.EqualFold(strings.TrimSpace(cfg.PaymentProvider), "yookassa") &&
		strings.HasPrefix(strings.TrimSpace(cfg.YooKassaSecretKey), "test_") &&
		strings.TrimSpace(cfg.YooKassaShopID) != "" && WebPaymentReturnURL(cfg) != ""
}

func WebPaymentReturnURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.WebOrigin), "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return ""
	}
	localHTTP := cfg.Env == "development" && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
	if u.Scheme != "https" && !localHTTP {
		return ""
	}
	u.Path = "/app/payment-return"
	return u.String()
}
