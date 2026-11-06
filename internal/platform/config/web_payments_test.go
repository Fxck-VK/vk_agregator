package config

import (
	"strings"
	"testing"
)

func TestWebPaymentsAreEnvironmentAndMerchantBound(t *testing.T) {
	live := Config{Env: "production", PaymentProvider: "yookassa", YooKassaShopID: "synthetic-shop", YooKassaSecretKey: "live_synthetic", YooKassaBaseURL: "https://api.yookassa.ru/v3", WebOrigin: "https://app.example.test", WebLivePaymentsEnabled: true}
	tests := []struct {
		name                string
		change              func(*Config)
		checkout, recurring bool
	}{
		{"live checkout explicit opt in", nil, true, false},
		{"live recurring separate opt in", func(c *Config) { c.WebLiveRecurringEnabled = true }, true, true},
		{"live disabled by default", func(c *Config) { c.WebLivePaymentsEnabled = false }, false, false},
		{"recurring requires checkout", func(c *Config) { c.WebLivePaymentsEnabled = false; c.WebLiveRecurringEnabled = true }, false, false},
		{"production rejects test merchant", func(c *Config) { c.YooKassaSecretKey = "test_synthetic" }, false, false},
		{"production rejects missing shop", func(c *Config) { c.YooKassaShopID = "" }, false, false},
		{"production rejects provider override", func(c *Config) { c.YooKassaBaseURL = "https://example.test/v3" }, false, false},
		{"production rejects unknown receipt mode", func(c *Config) { c.YooKassaReceiptMode = "unknown" }, false, false},
		{"production rejects insecure origin", func(c *Config) { c.WebOrigin = "http://app.example.test" }, false, false},
		{"production rejects local origin", func(c *Config) { c.WebOrigin = "https://localhost" }, false, false},
		{"production rejects credential origin", func(c *Config) { c.WebOrigin = "https://name:password@app.example.test" }, false, false},
		{"production rejects mock", func(c *Config) { c.PaymentProvider = "mock" }, false, false},
		{"DEV cannot opt into live", func(c *Config) { c.Env = "development"; c.WebLiveRecurringEnabled = true }, false, false},
		{"staging cannot opt into live", func(c *Config) { c.Env = "staging" }, false, false},
		{"unknown environment cannot opt in", func(c *Config) { c.Env = "preview" }, false, false},
		{"test merchant preserved", func(c *Config) {
			c.Env = "staging"
			c.WebLivePaymentsEnabled = false
			c.YooKassaSecretKey = "test_synthetic"
		}, true, true},
		{"local test merchant preserved", func(c *Config) {
			c.Env = "development"
			c.WebLivePaymentsEnabled = false
			c.YooKassaSecretKey = "test_synthetic"
			c.WebOrigin = "http://localhost:7159"
		}, true, true},
		{"test context rejects accidental live flag", func(c *Config) { c.Env = "development"; c.YooKassaSecretKey = "test_synthetic" }, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := live
			if tt.change != nil {
				tt.change(&c)
			}
			if got := c.WebPaymentsEnabled(); got != tt.checkout {
				t.Fatalf("checkout = %v, want %v", got, tt.checkout)
			}
			if got := c.WebRecurringPaymentsEnabled(); got != tt.recurring {
				t.Fatalf("recurring = %v, want %v", got, tt.recurring)
			}
		})
	}
}

func TestLiveWebPaymentFlagsValidateFailClosed(t *testing.T) {
	c := Config{Env: "production", PaymentProvider: "yookassa", YooKassaShopID: "synthetic-shop", YooKassaSecretKey: "live_synthetic", YooKassaBaseURL: "https://api.yookassa.ru/v3", WebOrigin: "https://app.example.test", WebLivePaymentsEnabled: true}
	if err := c.validateLiveWebPayments(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.Env = "development" },
		func(c *Config) { c.YooKassaShopID = "" },
		func(c *Config) { c.YooKassaSecretKey = "test_synthetic" },
		func(c *Config) { c.YooKassaBaseURL = "http://127.0.0.1:9999" },
		func(c *Config) { c.WebLivePaymentsEnabled = false; c.WebLiveRecurringEnabled = true },
	} {
		copy := c
		mutate(&copy)
		if copy.validateLiveWebPayments() == nil {
			t.Fatal("invalid live configuration accepted")
		}
	}
	if err := (Config{}).validateLiveWebPayments(); err != nil {
		t.Fatal("default-off policy rejected", err)
	}
}

func TestLoadLiveWebPaymentFlags(t *testing.T) {
	t.Setenv("WEB_LIVE_PAYMENTS_ENABLED", "true")
	t.Setenv("WEB_LIVE_RECURRING_ENABLED", "false")
	c := Load()
	if !c.WebLivePaymentsEnabled || c.WebLiveRecurringEnabled {
		t.Fatal("live flags not loaded independently")
	}
}

func TestLoadRejectsNonCanonicalLiveWebFlags(t *testing.T) {
	for _, name := range []string{"WEB_LIVE_PAYMENTS_ENABLED", "WEB_LIVE_RECURRING_ENABLED"} {
		for _, value := range []string{"on", "yes", "T", "TRUE", "1", "invalid"} {
			t.Run(name+"/"+value, func(t *testing.T) {
				t.Setenv("WEB_LIVE_PAYMENTS_ENABLED", "false")
				t.Setenv("WEB_LIVE_RECURRING_ENABLED", "false")
				t.Setenv(name, value)
				c := Load()
				err := c.validateLiveWebPayments()
				if err == nil || !strings.Contains(err.Error(), "canonical true or false") {
					t.Fatal("noncanonical flag not rejected consistently")
				}
				if c.WebPaymentsEnabled() || c.WebRecurringPaymentsEnabled() {
					t.Fatal("invalid flag enabled checkout")
				}
			})
		}
	}
}

func TestLiveWebFlagWhitespaceUsesSameParsingAsValidation(t *testing.T) {
	for _, name := range []string{"WEB_LIVE_PAYMENTS_ENABLED", "WEB_LIVE_RECURRING_ENABLED"} {
		for _, input := range []struct {
			value string
			want  bool
		}{{" true ", true}, {" false ", false}, {" \t ", false}} {
			t.Run(name+"/"+input.value, func(t *testing.T) {
				t.Setenv("WEB_LIVE_PAYMENTS_ENABLED", "false")
				t.Setenv("WEB_LIVE_RECURRING_ENABLED", "false")
				t.Setenv(name, input.value)
				c := Load()
				got := c.WebLivePaymentsEnabled
				if name == "WEB_LIVE_RECURRING_ENABLED" {
					got = c.WebLiveRecurringEnabled
				}
				if got != input.want || c.webLivePaymentFlagsError != "" {
					t.Fatal("flag parsing and validation disagree")
				}
			})
		}
	}
}
