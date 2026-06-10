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
	tests := []struct {
		name   string
		setup  func(*http.Request)
		secure bool
	}{
		{
			name: "direct tls",
			setup: func(r *http.Request) {
				r.TLS = &tls.ConnectionState{}
			},
			secure: true,
		},
		{
			name: "x forwarded proto",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "https,http")
			},
			secure: true,
		},
		{
			name: "forwarded proto",
			setup: func(r *http.Request) {
				r.Header.Set("Forwarded", `for=192.0.2.1;proto=https;host=payments.example`)
			},
			secure: true,
		},
		{
			name: "cloudflare visitor",
			setup: func(r *http.Request) {
				r.Header.Set("CF-Visitor", `{"scheme":"https"}`)
			},
			secure: true,
		},
		{
			name: "plain http",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-Proto", "http")
			},
			secure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", nil)
			tt.setup(req)
			if got := isSecureWebhookRequest(req); got != tt.secure {
				t.Fatalf("isSecureWebhookRequest = %v, want %v", got, tt.secure)
			}
		})
	}
}

func TestWebhookHandlerRejectsInsecureWhenHTTPSRequired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := webhookHandler(nil, logger, domain.PaymentProviderYooKassa, true)
	req := httptest.NewRequest(http.MethodPost, "/billing/webhooks/yookassa", strings.NewReader(`{"event":"payment.succeeded"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUpgradeRequired)
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
