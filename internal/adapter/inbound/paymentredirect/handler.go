// Package paymentredirect serves short public payment continuation links.
package paymentredirect

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/service/paymentservice"
)

type PaymentService interface {
	GetIntentAdmin(ctx context.Context, intentID uuid.UUID) (*domain.PaymentIntent, error)
}

type RateLimiter interface {
	Allow(key string) bool
}

type Deps struct {
	Payment     PaymentService
	RateLimiter RateLimiter
	Logger      *slog.Logger
}

type Handler struct {
	payment     PaymentService
	rateLimiter RateLimiter
	logger      *slog.Logger
	now         func() time.Time
}

func NewHandler(deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		payment:     deps.Payment,
		rateLimiter: deps.RateLimiter,
		logger:      logger,
		now:         time.Now,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /payments/vk/{id}", h.redirectVKPayment)
	return mux
}

func (h *Handler) redirectVKPayment(w http.ResponseWriter, r *http.Request) {
	setNoStoreHeaders(w)
	if h.rateLimiter != nil && !h.rateLimiter.Allow(clientIP(r)) {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	if h.payment == nil {
		http.NotFound(w, r)
		return
	}
	intentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	intent, err := h.payment.GetIntentAdmin(r.Context(), intentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("payment redirect lookup failed", logging.ErrorAttr(err))
		http.Error(w, "payment unavailable", http.StatusInternalServerError)
		return
	}
	if paymentMetadataSource(intent.Metadata) != "vk_bot" {
		http.NotFound(w, r)
		return
	}
	if intent.Status != domain.PaymentIntentWaitingForUser {
		http.Error(w, "payment is not waiting for confirmation", http.StatusGone)
		return
	}
	if intent.ExpiresAt != nil && !intent.ExpiresAt.After(h.now()) {
		http.Error(w, "payment is not waiting for confirmation", http.StatusGone)
		return
	}
	target := strings.TrimSpace(intent.ConfirmationURL)
	if !safeHTTPSURL(target) {
		h.logger.Warn("payment redirect target is unavailable")
		http.Error(w, "payment unavailable", http.StatusGone)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func setNoStoreHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}

func paymentMetadataSource(raw json.RawMessage) string {
	var metadata struct {
		Source string `json:"source"`
	}
	if len(raw) == 0 {
		return ""
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.Source)
}

func safeHTTPSURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

var _ PaymentService = (*paymentservice.Service)(nil)
