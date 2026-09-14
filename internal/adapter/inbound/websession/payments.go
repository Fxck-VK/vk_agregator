package websession

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accountservice"
	"vk-ai-aggregator/internal/service/paymentservice"
)

// WebPaymentService reuses account-native intents. This adapter never grants credits.
type WebPaymentService interface {
	ListActiveProducts(context.Context) ([]*domain.PaymentProduct, error)
	CreateAccountIntent(context.Context, paymentservice.CreateAccountIntentInput) (paymentservice.CreateIntentResult, error)
	GetAccountIntent(context.Context, uuid.UUID, uuid.UUID) (*domain.PaymentIntent, error)
}

// WebReceiptContacts supplies a server-owned contact, never a browser DTO.
type WebReceiptContacts interface {
	VerifiedReceiptEmail(context.Context, uuid.UUID) (string, error)
}

type safePaymentProduct struct {
	Code     string          `json:"code"`
	Amount   int64           `json:"amount"`
	Currency domain.Currency `json:"currency"`
	Credits  int64           `json:"credits"`
}
type safeWebPayment struct {
	ID              uuid.UUID                  `json:"id"`
	Status          domain.PaymentIntentStatus `json:"status"`
	Amount          int64                      `json:"amount"`
	Currency        domain.Currency            `json:"currency"`
	Credits         int64                      `json:"credits"`
	ConfirmationURL string                     `json:"confirmation_url,omitempty"`
}

func (h *Handler) listWebPaymentProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Payments == nil {
		writeError(w, 503, "payments unavailable")
		return
	}
	products, err := h.deps.Payments.ListActiveProducts(r.Context())
	if err != nil {
		writeError(w, 503, "payments unavailable")
		return
	}
	items := make([]safePaymentProduct, 0, len(products))
	for _, p := range products {
		if p == nil || !p.IsActive || p.Currency != domain.CurrencyRUB || p.Amount <= 0 {
			continue
		}
		credits, err := p.CurrentCredits()
		if err != nil || credits <= 0 {
			continue
		}
		items = append(items, safePaymentProduct{Code: p.Code, Amount: p.Amount, Currency: p.Currency, Credits: credits})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Credits > items[j].Credits })
	writeJSON(w, 200, struct {
		Items             []safePaymentProduct `json:"items"`
		CheckoutAvailable bool                 `json:"checkout_available"`
	}{items, h.cfg.TestPaymentsEnabled})
}

func (h *Handler) createWebPayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Payments == nil || !h.cfg.TestPaymentsEnabled {
		writeError(w, 503, "test checkout unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	key, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Idempotency-Key")))
	if err != nil || key == uuid.Nil {
		writeError(w, 400, "invalid idempotency key")
		return
	}
	var req struct {
		ProductCode string `json:"product_code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ProductCode = strings.TrimSpace(req.ProductCode)
	if len(req.ProductCode) == 0 || len(req.ProductCode) > 64 {
		writeError(w, 400, "invalid payment details")
		return
	}
	if h.deps.PaymentCreateLimiter != nil {
		allowed, err := h.deps.PaymentCreateLimiter.Allow(r.Context(), principal.AccountID.String())
		if err != nil {
			writeError(w, 503, "payments unavailable")
			return
		}
		if !allowed {
			writeError(w, 429, "too many payment requests")
			return
		}
	}
	if h.deps.ReceiptContacts == nil {
		writeError(w, 503, "payments unavailable")
		return
	}
	email, err := h.deps.ReceiptContacts.VerifiedReceiptEmail(r.Context(), principal.AccountID)
	if err != nil {
		if errors.Is(err, accountservice.ErrReceiptEmailUnavailable) {
			writeError(w, 422, "verified account email required")
		} else {
			writeError(w, 503, "payments unavailable")
		}
		return
	}
	result, err := h.deps.Payments.CreateAccountIntent(r.Context(), paymentservice.CreateAccountIntentInput{
		AccountID: principal.AccountID, ProductCode: req.ProductCode, ReceiptEmail: email,
		IdempotencyKey: "web-payment:" + principal.AccountID.String() + ":" + key.String(),
	})
	if err != nil {
		writeWebPaymentError(w, err)
		return
	}
	if result.Intent == nil || result.Intent.AccountID != principal.AccountID {
		writeError(w, 503, "payments unavailable")
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeSafeWebPayment(w, status, result.Intent)
}

func (h *Handler) getWebPayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.deps.Payments == nil {
		writeError(w, 503, "payments unavailable")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("paymentID"))
	if err != nil || id == uuid.Nil {
		writeError(w, 404, "payment not found")
		return
	}
	intent, err := h.deps.Payments.GetAccountIntent(r.Context(), principal.AccountID, id)
	if err != nil {
		writeWebPaymentError(w, err)
		return
	}
	if intent == nil || intent.AccountID != principal.AccountID || intent.ID != id {
		writeError(w, 404, "payment not found")
		return
	}
	writeSafeWebPayment(w, 200, intent)
}

func writeSafeWebPayment(w http.ResponseWriter, status int, intent *domain.PaymentIntent) {
	credits, err := intent.CurrentCredits()
	if err != nil {
		writeError(w, 503, "payments unavailable")
		return
	}
	dto := safeWebPayment{ID: intent.ID, Status: intent.Status, Amount: intent.Amount, Currency: intent.Currency, Credits: credits}
	if intent.Status == domain.PaymentIntentWaitingForUser && intent.Provider == domain.PaymentProviderYooKassa && safeYooKassaCheckoutURL(intent.ConfirmationURL) {
		dto.ConfirmationURL = intent.ConfirmationURL
	}
	writeJSON(w, status, dto)
}

func safeYooKassaCheckoutURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	switch u.Host {
	case "yoomoney.ru", "yookassa.ru", "checkout.yookassa.ru":
		return true
	}
	return false
}

func writeWebPaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, paymentservice.ErrForbidden):
		writeError(w, 404, "payment not found")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, 409, "payment request conflict")
	case errors.Is(err, paymentservice.ErrInvalidInput), errors.Is(err, paymentservice.ErrReceiptContactRequired):
		writeError(w, 400, "invalid payment details")
	default:
		writeError(w, 503, "payments unavailable")
	}
}
