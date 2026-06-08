// Package billing exposes protected payment-intent endpoints. It is not a
// public billing surface: callers must authenticate with X-Admin-Token.
package billing

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Config holds protected billing API settings.
type Config struct {
	Token string
}

// Deps are the services/repositories used by billing endpoints.
type Deps struct {
	Users      domain.UserRepository
	Payment    *paymentservice.Service
	PaymentOps *paymentservice.WebhookProcessor
}

// Handler serves /billing/* routes.
type Handler struct {
	cfg  Config
	deps Deps
}

// NewHandler builds a protected billing handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	return &Handler{cfg: cfg, deps: deps}
}

// Routes returns the protected billing routes.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /billing/payment-intents", h.auth(h.createIntent))
	mux.HandleFunc("GET /billing/payment-intents/{id}", h.auth(h.getIntent))
	mux.HandleFunc("POST /billing/payment-intents/{id}/sync", h.auth(h.syncIntent))
	mux.HandleFunc("POST /billing/payment-intents/{id}/refund", h.auth(h.refundIntent))
	mux.HandleFunc("GET /billing/payment-history", h.auth(h.listHistory))
	return mux
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.Token == "" || r.Header.Get("X-Admin-Token") != h.cfg.Token {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

type createIntentRequest struct {
	ProductCode  string `json:"product_code"`
	ReceiptEmail string `json:"receipt_email,omitempty"`
	ReceiptPhone string `json:"receipt_phone,omitempty"`
	ReturnURL    string `json:"return_url,omitempty"`
}

func (h *Handler) createIntent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil || h.deps.Users == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	userID, ok := h.userIDFromTrustedRequest(w, r)
	if !ok {
		return
	}
	if _, err := h.deps.Users.GetByID(r.Context(), userID); err != nil {
		writeNotFoundOr500(w, err, "get user failed")
		return
	}
	clientKey := strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	if clientKey == "" {
		writeError(w, http.StatusBadRequest, "X-Idempotency-Key is required")
		return
	}
	var req createIntentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	result, err := h.deps.Payment.CreateIntent(r.Context(), paymentservice.CreateIntentInput{
		UserID:         userID,
		ProductCode:    req.ProductCode,
		ReceiptEmail:   req.ReceiptEmail,
		ReceiptPhone:   req.ReceiptPhone,
		IdempotencyKey: "billing_payment:" + userID.String() + ":" + clientKey,
		ReturnURL:      req.ReturnURL,
		Source:         "billing_admin",
	})
	if err != nil {
		h.writePaymentError(w, err)
		return
	}
	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, newIntentDTO(result.Intent, true))
}

func (h *Handler) getIntent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	intent, err := h.deps.Payment.GetIntentAdmin(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get payment intent failed")
		return
	}
	writeJSON(w, http.StatusOK, newIntentDTO(intent, true))
}

func (h *Handler) syncIntent(w http.ResponseWriter, r *http.Request) {
	if h.deps.PaymentOps == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	intent, err := h.deps.PaymentOps.SyncIntent(r.Context(), id)
	if err != nil {
		h.writePaymentActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newIntentDTO(intent, true))
}

type refundIntentRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (h *Handler) refundIntent(w http.ResponseWriter, r *http.Request) {
	if h.deps.PaymentOps == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	clientKey := strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	if clientKey == "" {
		writeError(w, http.StatusBadRequest, "X-Idempotency-Key is required")
		return
	}
	var req refundIntentRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}
	result, err := h.deps.PaymentOps.RefundIntent(r.Context(), paymentservice.RefundIntentInput{
		IntentID:       id,
		IdempotencyKey: "billing_refund:" + id.String() + ":" + clientKey,
		Reason:         req.Reason,
	})
	if err != nil {
		h.writePaymentActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, refundResponse{
		Intent: newIntentDTO(result.Intent, true),
		Refund: newRefundDTO(result.Refund),
	})
}

func (h *Handler) listHistory(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, offset := parsePagination(r)
	var filter domain.PaymentIntentFilter
	if raw := strings.TrimSpace(r.URL.Query().Get("user_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		filter.UserID = &id
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		filter.Status = domain.PaymentIntentStatus(status)
	}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" {
		filter.Provider = domain.PaymentProviderCode(provider)
	}
	intents, err := h.deps.Payment.ListIntents(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list payment history failed")
		return
	}
	hasMore := len(intents) > limit
	if hasMore {
		intents = intents[:limit]
	}
	items := make([]PaymentIntentDTO, 0, len(intents))
	for _, intent := range intents {
		items = append(items, newIntentDTO(intent, true))
	}
	writeJSON(w, http.StatusOK, listResponse[PaymentIntentDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) userIDFromTrustedRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("user_id"))
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		writeError(w, http.StatusBadRequest, "trusted user id is required")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) writePaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paymentservice.ErrInvalidInput),
		errors.Is(err, paymentservice.ErrReceiptContactRequired):
		writeError(w, http.StatusBadRequest, "invalid payment request")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, paymentservice.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	default:
		writeError(w, http.StatusBadGateway, "payment provider error")
	}
}

func (h *Handler) writePaymentActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paymentservice.ErrInvalidInput),
		errors.Is(err, paymentservice.ErrWebhookUnsupported):
		writeError(w, http.StatusBadRequest, "invalid payment action")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, paymentservice.ErrRefundCreditsSpent),
		errors.Is(err, paymentservice.ErrRefundNotAllowed),
		errors.Is(err, paymentservice.ErrWebhookMismatch):
		writeError(w, http.StatusConflict, "payment action conflict")
	case errors.Is(err, paymentservice.ErrWebhookUnverified):
		writeError(w, http.StatusBadGateway, "payment provider verification failed")
	default:
		writeError(w, http.StatusInternalServerError, "payment action failed")
	}
}

type PaymentIntentDTO struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id,omitempty"`
	ProductID         uuid.UUID `json:"product_id,omitempty"`
	Status            string    `json:"status"`
	Amount            int64     `json:"amount"`
	Currency          string    `json:"currency"`
	Credits           int64     `json:"credits"`
	PriceVersion      int       `json:"price_version"`
	Provider          string    `json:"provider,omitempty"`
	ProviderPaymentID string    `json:"provider_payment_id,omitempty"`
	ConfirmationURL   string    `json:"confirmation_url,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PaymentRefundDTO struct {
	ID               uuid.UUID `json:"id"`
	IntentID         uuid.UUID `json:"intent_id"`
	ProviderRefundID string    `json:"provider_refund_id,omitempty"`
	Amount           int64     `json:"amount"`
	Status           string    `json:"status"`
	Reason           string    `json:"reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type refundResponse struct {
	Intent PaymentIntentDTO `json:"intent"`
	Refund PaymentRefundDTO `json:"refund"`
}

type pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Count   int  `json:"count"`
	HasMore bool `json:"has_more"`
}

type listResponse[T any] struct {
	Items      []T        `json:"items"`
	Pagination pagination `json:"pagination"`
}

func newIntentDTO(intent *domain.PaymentIntent, includeOperatorFields bool) PaymentIntentDTO {
	dto := PaymentIntentDTO{
		ID:              intent.ID,
		Status:          string(intent.Status),
		Amount:          intent.Amount,
		Currency:        string(intent.Currency),
		Credits:         intent.Credits,
		PriceVersion:    intent.PriceVersion,
		ConfirmationURL: intent.ConfirmationURL,
		CreatedAt:       intent.CreatedAt,
		UpdatedAt:       intent.UpdatedAt,
	}
	if intent.ProductID != nil {
		dto.ProductID = *intent.ProductID
	}
	if includeOperatorFields {
		dto.UserID = intent.UserID
		dto.Provider = string(intent.Provider)
		dto.ProviderPaymentID = intent.ProviderPaymentID
	}
	return dto
}

func newRefundDTO(refund *domain.PaymentRefund) PaymentRefundDTO {
	if refund == nil {
		return PaymentRefundDTO{}
	}
	return PaymentRefundDTO{
		ID:               refund.ID,
		IntentID:         refund.IntentID,
		ProviderRefundID: refund.ProviderRefundID,
		Amount:           refund.Amount,
		Status:           string(refund.Status),
		Reason:           refund.Reason,
		CreatedAt:        refund.CreatedAt,
		UpdatedAt:        refund.UpdatedAt,
	}
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	return limit, offset
}

func writeNotFoundOr500(w http.ResponseWriter, err error, msg string) {
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, msg)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
