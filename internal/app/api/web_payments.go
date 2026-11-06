package api

import (
	"vk-ai-aggregator/internal/platform/config"
)

// WebPaymentsEnabled delegates to the same policy used by the renewal process.
func WebPaymentsEnabled(cfg *config.Config) bool {
	return cfg.WebPaymentsEnabled()
}

func WebPaymentReturnURL(cfg *config.Config) string {
	return cfg.WebPaymentReturnURL()
}
