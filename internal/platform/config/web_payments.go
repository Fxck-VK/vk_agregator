package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// WebPaymentsEnabled is the shared server policy for API checkout and renewal
// wiring. A browser release-view preference never selects a merchant.
func (c *Config) WebPaymentsEnabled() bool {
	if c == nil || c.webLivePaymentFlagsError != "" || !validYooKassaReceiptMode(c.YooKassaReceiptMode) {
		return false
	}
	if c.WebLivePaymentsEnabled || c.WebLiveRecurringEnabled {
		return c.liveWebPaymentsReady()
	}
	return (c.Env == "development" || c.Env == "staging") &&
		strings.EqualFold(strings.TrimSpace(c.PaymentProvider), "yookassa") &&
		strings.HasPrefix(strings.TrimSpace(c.YooKassaSecretKey), "test_") &&
		strings.TrimSpace(c.YooKassaShopID) != "" && c.WebPaymentReturnURL() != ""
}

// WebRecurringPaymentsEnabled requires an independent live merchant opt-in.
// Test merchants retain their existing recurring test behavior.
func (c *Config) WebRecurringPaymentsEnabled() bool {
	return c != nil && c.WebPaymentsEnabled() && (c.Env != "production" || c.WebLiveRecurringEnabled)
}

func (c *Config) liveWebPaymentsReady() bool {
	if c == nil || c.Env != "production" || !c.WebLivePaymentsEnabled ||
		!validYooKassaReceiptMode(c.YooKassaReceiptMode) ||
		!strings.EqualFold(strings.TrimSpace(c.PaymentProvider), "yookassa") ||
		!strings.HasPrefix(strings.TrimSpace(c.YooKassaSecretKey), "live_") ||
		strings.TrimSpace(c.YooKassaShopID) == "" ||
		strings.TrimRight(strings.TrimSpace(c.YooKassaBaseURL), "/") != "https://api.yookassa.ru/v3" ||
		!validWebOrigin(c.WebOrigin) {
		return false
	}
	u, err := url.Parse(c.WebOrigin)
	if err != nil || strings.EqualFold(u.Hostname(), "localhost") {
		return false
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && ip.IsLoopback() {
		return false
	}
	return c.WebPaymentReturnURL() != ""
}

func validYooKassaReceiptMode(raw string) bool {
	mode := strings.TrimSpace(raw)
	return mode == "" || mode == "yookassa" || mode == "npd_manual"
}

func (c Config) validateLiveWebPayments() error {
	if c.webLivePaymentFlagsError != "" {
		return fmt.Errorf("config: %s must be canonical true or false", c.webLivePaymentFlagsError)
	}
	if !c.WebLivePaymentsEnabled && !c.WebLiveRecurringEnabled {
		return nil
	}
	if c.WebLiveRecurringEnabled && !c.WebLivePaymentsEnabled {
		return fmt.Errorf("config: WEB_LIVE_RECURRING_ENABLED requires WEB_LIVE_PAYMENTS_ENABLED")
	}
	if !c.liveWebPaymentsReady() {
		return fmt.Errorf("config: WEB_LIVE_PAYMENTS_ENABLED requires production, a configured live YooKassa merchant, the official API and a public HTTPS WEB_ORIGIN")
	}
	return nil
}

func liveWebPaymentEnvBool(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) == "true"
}

func liveWebPaymentEnvError() string {
	for _, name := range []string{"WEB_LIVE_PAYMENTS_ENABLED", "WEB_LIVE_RECURRING_ENABLED"} {
		value := strings.TrimSpace(os.Getenv(name))
		if value != "" && value != "true" && value != "false" {
			return name
		}
	}
	return ""
}

// WebPaymentReturnURL is server-owned and never taken from checkout input.
func (c *Config) WebPaymentReturnURL() string {
	if c == nil {
		return ""
	}
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(c.WebOrigin), "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return ""
	}
	localHTTP := c.Env == "development" && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
	if u.Scheme != "https" && !localHTTP {
		return ""
	}
	u.Path = "/app/payment-return"
	return u.String()
}
