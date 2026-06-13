package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	paymentmock "vk-ai-aggregator/internal/adapter/payment/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

func TestIsSecureWebhookRequest(t *testing.T) {
	trustedSecurity := mustWebhookSecurity(t, []string{"10.0.0.0/8"}, nil, true, false)
	tests := []struct {
		name   string
		sec    webhookSecurityConfig
		setup  func(*http.Request)
		secure bool
	}{
		{
			name: "direct tls",
			sec:  webhookSecurityConfig{requireHTTPS: true},
			setup: func(r *http.Request) {
				r.TLS = &tls.ConnectionState{}
			},
			secure: true,
		},
		{
			name: "x forwarded proto from trusted proxy",
			sec:  trustedSecurity,
			setup: func(r *http.Request) {
				r.RemoteAddr = "10.0.0.10:12345"
				r.Header.Set("X-Forwarded-Proto", "https,http")
			},
			secure: true,
		},
		{
			name: "x forwarded proto from untrusted remote",
			sec:  trustedSecurity,
			setup: func(r *http.Request) {
				r.RemoteAddr = "198.51.100.10:12345"
				r.Header.Set("X-Forwarded-Proto", "https")
			},
			secure: false,
		},
		{
			name: "forwarded proto from trusted proxy",
			sec:  trustedSecurity,
			setup: func(r *http.Request) {
				r.RemoteAddr = "10.0.0.10:12345"
				r.Header.Set("Forwarded", `for=192.0.2.1;proto=https;host=payments.example`)
			},
			secure: true,
		},
		{
			name: "cloudflare visitor from trusted proxy",
			sec:  trustedSecurity,
			setup: func(r *http.Request) {
				r.RemoteAddr = "10.0.0.10:12345"
				r.Header.Set("CF-Visitor", `{"scheme":"https"}`)
			},
			secure: true,
		},
		{
			name: "plain http",
			sec:  trustedSecurity,
			setup: func(r *http.Request) {
				r.RemoteAddr = "10.0.0.10:12345"
				r.Header.Set("X-Forwarded-Proto", "http")
			},
			secure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", nil)
			tt.setup(req)
			if got := tt.sec.isSecureRequest(req); got != tt.secure {
				t.Fatalf("isSecureWebhookRequest = %v, want %v", got, tt.secure)
			}
		})
	}
}

func TestWebhookHandlerRejectsInsecureWhenHTTPSRequired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := webhookHandler(nil, logger, domain.PaymentProviderYooKassa, webhookSecurityConfig{requireHTTPS: true})
	req := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", strings.NewReader(`{"event":"payment.succeeded"}`))
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUpgradeRequired)
	}
}

func TestWebhookSecurityAllowlistUsesTrustedForwardedForOnly(t *testing.T) {
	security := mustWebhookSecurity(t, []string{"10.0.0.0/8"}, []string{"203.0.113.0/24"}, false, true)

	trustedProxyReq := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", nil)
	trustedProxyReq.RemoteAddr = "10.0.0.10:12345"
	trustedProxyReq.Header.Set("X-Forwarded-For", "203.0.113.20, 10.0.0.10")
	if !security.sourceAllowed(trustedProxyReq) {
		t.Fatal("trusted proxy forwarded allowlisted client was rejected")
	}

	spoofedDirectReq := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", nil)
	spoofedDirectReq.RemoteAddr = "198.51.100.10:12345"
	spoofedDirectReq.Header.Set("X-Forwarded-For", "203.0.113.20")
	if security.sourceAllowed(spoofedDirectReq) {
		t.Fatal("direct request spoofing X-Forwarded-For bypassed allowlist")
	}
}

func TestWebhookHandlerRejectsNotAllowlistedSource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := webhookHandler(nil, logger, domain.PaymentProviderYooKassa, mustWebhookSecurity(t, nil, []string{"203.0.113.0/24"}, false, true))
	req := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", strings.NewReader(`{"event":"payment.succeeded"}`))
	req.RemoteAddr = "198.51.100.10:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func mustWebhookSecurity(t *testing.T, trustedProxies, allowlist []string, requireHTTPS, allowlistEnabled bool) webhookSecurityConfig {
	t.Helper()
	proxies, err := parseIPNets(trustedProxies)
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}
	nets, err := parseIPNets(allowlist)
	if err != nil {
		t.Fatalf("parse allowlist: %v", err)
	}
	return webhookSecurityConfig{
		requireHTTPS:       requireHTTPS,
		trustedProxies:     proxies,
		ipAllowlistEnabled: allowlistEnabled,
		ipAllowlist:        nets,
	}
}

func TestReadinessHandlerReportsWebhookInboxStats(t *testing.T) {
	repo := memory.NewPaymentRepo()
	provider := paymentmock.New()
	processor := paymentservice.NewWebhookProcessor(repo, provider, nil, nil)
	if _, _, err := processor.IngestWebhook(context.Background(), []byte(`{"event_type":"payment.succeeded","provider_payment_id":"mock-pay-1"}`), nil); err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := readinessHandler(testPinger{}, processor, domain.PaymentProviderMock, logger)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body readinessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}
	if body.Status != "ok" ||
		body.Checks["postgres"] != "ok" ||
		body.Checks["webhook_inbox"] != "ok" ||
		body.PaymentWebhook.Provider != string(domain.PaymentProviderMock) ||
		body.PaymentWebhook.UnprocessedEvents != 1 {
		t.Fatalf("unexpected readiness body: %+v", body)
	}
}

func TestReadinessHandlerFailsClosedWhenPostgresUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := readinessHandler(testPinger{err: errors.New("down")}, nil, domain.PaymentProviderMock, logger)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body readinessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readiness body: %v", err)
	}
	if body.Status != "degraded" || body.Checks["postgres"] != "unavailable" {
		t.Fatalf("unexpected readiness body: %+v", body)
	}
}

type testPinger struct {
	err error
}

func (p testPinger) Ping(context.Context) error {
	return p.err
}
