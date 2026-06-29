// Package billing exposes protected payment-intent endpoints. It is not a
// public billing surface: callers must authenticate with X-Admin-Token.
package billing

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/service/paymentservice"
)

const (
	defaultLimit = 20
	maxLimit     = 100

	defaultBillingAdminRateLimit = 120
)

// Config holds protected billing API settings.
type Config struct {
	Token                     string
	AllowLoadTestMockPayments bool
	// DefaultRole is the backend-enforced role for the legacy single-token
	// model. Empty/invalid values default to owner for backward compatibility.
	DefaultRole domain.OperatorRole
	// RateLimit bounds protected billing/admin endpoints per authenticated actor.
	RateLimit AdminRateLimitConfig
}

// AdminRateLimitConfig controls protected billing/admin endpoint request limits.
type AdminRateLimitConfig struct {
	Disabled bool
	Limit    int
	Window   time.Duration
	Limiter  ratelimit.KeyLimiter
}

// Deps are the services/repositories used by billing endpoints.
type Deps struct {
	Users      domain.UserRepository
	Billing    domain.BillingRepository
	Payment    *paymentservice.Service
	PaymentOps *paymentservice.WebhookProcessor
	Audits     domain.OperatorAuditRepository
}

// Handler serves /billing/* routes.
type Handler struct {
	cfg         Config
	deps        Deps
	rateMu      sync.Mutex
	rateBuckets map[string]billingAdminRateBucket
}

// NewHandler builds a protected billing handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	cfg.RateLimit = normalizeAdminRateLimit(cfg.RateLimit)
	return &Handler{cfg: cfg, deps: deps, rateBuckets: map[string]billingAdminRateBucket{}}
}

// Routes returns the protected billing routes.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /billing/payment-products", h.protected("payment_product_list", domain.OperatorPermissionPaymentsSafeRead, h.listProducts))
	mux.HandleFunc("POST /billing/payment-products", h.protected("payment_product_create", domain.OperatorPermissionRolePolicyManage, h.createProduct))
	mux.HandleFunc("GET /billing/payment-products/{id}", h.protected("payment_product_get", domain.OperatorPermissionPaymentsSafeRead, h.getProduct))
	mux.HandleFunc("PATCH /billing/payment-products/{id}", h.protected("payment_product_update", domain.OperatorPermissionRolePolicyManage, h.updateProduct))
	mux.HandleFunc("POST /billing/payment-products/{id}/disable", h.protected("payment_product_disable", domain.OperatorPermissionRolePolicyManage, h.disableProduct))
	mux.HandleFunc("POST /billing/payment-intents", h.protected("payment_intent_create", domain.OperatorPermissionRefundsManage, h.createIntent))
	mux.HandleFunc("GET /billing/payment-intents/{id}", h.protected("payment_intent_get", domain.OperatorPermissionPaymentsSafeRead, h.getIntent))
	mux.HandleFunc("POST /billing/payment-intents/{id}/sync", h.protected("payment_intent_sync", domain.OperatorPermissionRefundsManage, h.syncIntent))
	mux.HandleFunc("POST /billing/payment-intents/{id}/mock-status", h.protected("payment_intent_mock_status", domain.OperatorPermissionRefundsManage, h.setMockPaymentStatus))
	mux.HandleFunc("POST /billing/payment-intents/{id}/cancel", h.protected("payment_intent_cancel", domain.OperatorPermissionRefundsManage, h.cancelIntent))
	mux.HandleFunc("POST /billing/payment-intents/{id}/refund", h.protected("payment_intent_refund", domain.OperatorPermissionRefundsManage, h.refundIntent))
	mux.HandleFunc("GET /billing/operator/console", h.protected("payment_operator_console_get", domain.OperatorPermissionPaymentsSafeRead, h.operatorConsole))
	mux.HandleFunc("GET /billing/payment-intents/pending", h.protected("payment_intents_pending_list", domain.OperatorPermissionPaymentsSafeRead, h.listPendingIntents))
	mux.HandleFunc("GET /billing/payment-events/unprocessed", h.protected("payment_events_unprocessed_list", domain.OperatorPermissionPaymentsSafeRead, h.listUnprocessedEvents))
	mux.HandleFunc("GET /billing/payment-history", h.protected("payment_history_list", domain.OperatorPermissionPaymentsSafeRead, h.listHistory))
	return mux
}

func (h *Handler) protected(action string, permission domain.OperatorPermission, next http.HandlerFunc) http.HandlerFunc {
	return h.auth(h.operatorAction(action, h.rateLimit(h.requirePermission(permission, next))))
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminTokenEqual(r.Header.Get("X-Admin-Token"), h.cfg.Token) {
			metrics.AuthFailures.WithLabelValues("billing_admin", "invalid_admin_token").Inc()
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		identity := h.operatorIdentity()
		next(w, r.WithContext(context.WithValue(r.Context(), operatorIdentityContextKey{}, identity)))
	}
}

type operatorIdentityContextKey struct{}

type operatorIdentity struct {
	Role     domain.OperatorRole
	ActorRef string
}

func (h *Handler) operatorIdentity() operatorIdentity {
	role := h.cfg.DefaultRole
	if !role.Valid() {
		role = domain.OperatorRoleOwner
	}
	return operatorIdentity{
		Role:     role,
		ActorRef: safeStringRef("operator", h.cfg.Token),
	}
}

func operatorIdentityFromContext(ctx context.Context) operatorIdentity {
	identity, ok := ctx.Value(operatorIdentityContextKey{}).(operatorIdentity)
	if !ok || !identity.Role.Valid() {
		return operatorIdentity{Role: domain.OperatorRoleOwner, ActorRef: "operator_unknown"}
	}
	if strings.TrimSpace(identity.ActorRef) == "" {
		identity.ActorRef = "operator_unknown"
	}
	return identity
}

func (h *Handler) requirePermission(permission domain.OperatorPermission, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := operatorIdentityFromContext(r.Context())
		if !identity.Role.HasPermission(permission) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r)
	}
}

type billingAdminRateBucket struct {
	Count    int
	ResetAt  time.Time
	LastSeen time.Time
}

func normalizeAdminRateLimit(in AdminRateLimitConfig) AdminRateLimitConfig {
	if in.Limit <= 0 {
		in.Limit = defaultBillingAdminRateLimit
	}
	if in.Window <= 0 {
		in.Window = time.Minute
	}
	return in
}

func (h *Handler) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.cfg.RateLimit.Disabled {
			next(w, r)
			return
		}
		identity := operatorIdentityFromContext(r.Context())
		allowed, err := h.allowAdminRequest(r.Context(), "billing:"+identity.ActorRef, time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "rate limiter unavailable")
			return
		}
		if !allowed {
			writeError(w, http.StatusTooManyRequests, "rate limited")
			return
		}
		next(w, r)
	}
}

func (h *Handler) allowAdminRequest(ctx context.Context, key string, now time.Time) (bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "operator_unknown"
	}
	if h.cfg.RateLimit.Limiter != nil {
		return h.cfg.RateLimit.Limiter.Allow(ctx, key)
	}
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	bucket := h.rateBuckets[key]
	if bucket.ResetAt.IsZero() || !now.Before(bucket.ResetAt) {
		bucket = billingAdminRateBucket{ResetAt: now.Add(h.cfg.RateLimit.Window)}
	}
	bucket.Count++
	bucket.LastSeen = now
	h.rateBuckets[key] = bucket
	if len(h.rateBuckets) > 1024 {
		for candidate, candidateBucket := range h.rateBuckets {
			if now.After(candidateBucket.ResetAt) || now.Sub(candidateBucket.LastSeen) > h.cfg.RateLimit.Window*2 {
				delete(h.rateBuckets, candidate)
			}
		}
	}
	return bucket.Count <= h.cfg.RateLimit.Limit, nil
}

type actionStatusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *actionStatusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (h *Handler) operatorAction(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &actionStatusRecorder{ResponseWriter: w, code: http.StatusOK}
		next(rec, r)
		result := "success"
		if rec.code >= 400 {
			result = "error"
		}
		metrics.AdminActions.WithLabelValues(action, result).Inc()
		h.recordOperatorAudit(r, action, result)
	}
}

func (h *Handler) recordOperatorAudit(r *http.Request, action, result string) {
	if h.deps.Audits == nil {
		return
	}
	entryID := uuid.New()
	targetType := billingOperatorTargetType(action)
	identity := operatorIdentityFromContext(r.Context())
	entry := &domain.OperatorAuditEntry{
		ID:         entryID,
		ActorRef:   identity.ActorRef,
		Action:     sanitizeOperatorToken(action),
		TargetType: targetType,
		TargetRef:  safeStringRef("target", targetType+":"+r.URL.Path),
		Result:     billingOperatorAuditResult(result),
		RequestRef: billingOperatorRequestRef(r, entryID),
	}
	if entry.Action == "" {
		entry.Action = "unknown"
	}
	if entry.ActorRef == "" {
		entry.ActorRef = billingOperatorActorRef(h.cfg.Token)
	}
	if entry.TargetRef == "" {
		entry.TargetRef = safeUUIDRef("target", entryID)
	}
	_ = h.deps.Audits.Create(r.Context(), entry)
}

func billingOperatorActorRef(token string) string {
	if strings.TrimSpace(token) == "" {
		return "admin_dev"
	}
	return "admin_token"
}

func billingOperatorTargetType(action string) string {
	value := strings.ToLower(action)
	switch {
	case strings.Contains(value, "payment_product"):
		return "payment_products"
	case strings.Contains(value, "payment_intent"):
		return "payment_intents"
	case strings.Contains(value, "payment"):
		return "payments"
	default:
		return "billing"
	}
}

func billingOperatorAuditResult(result string) string {
	if result == "success" {
		return "success"
	}
	return "error"
}

func billingOperatorRequestRef(r *http.Request, fallback uuid.UUID) string {
	for _, header := range []string{"X-Request-ID", "X-Correlation-ID"} {
		if raw := strings.TrimSpace(r.Header.Get(header)); raw != "" {
			return safeStringRef("request", raw)
		}
	}
	return safeUUIDRef("request", fallback)
}

func adminTokenEqual(got, want string) bool {
	if want == "" || got == "" || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

type optionalInt16 struct {
	Set   bool
	Value *int16
}

func (o *optionalInt16) UnmarshalJSON(data []byte) error {
	o.Set = true
	raw := strings.TrimSpace(string(data))
	if raw == "null" {
		o.Value = nil
		return nil
	}
	var value int16
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

type createProductRequest struct {
	Code           string `json:"code"`
	Title          string `json:"title"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency,omitempty"`
	Credits        int64  `json:"credits"`
	VATCode        *int16 `json:"vat_code,omitempty"`
	PaymentSubject string `json:"payment_subject,omitempty"`
	PaymentMode    string `json:"payment_mode,omitempty"`
	IsActive       *bool  `json:"is_active,omitempty"`
}

type updateProductRequest struct {
	Title          *string       `json:"title,omitempty"`
	Amount         *int64        `json:"amount,omitempty"`
	Currency       *string       `json:"currency,omitempty"`
	Credits        *int64        `json:"credits,omitempty"`
	VATCode        optionalInt16 `json:"vat_code,omitempty"`
	PaymentSubject *string       `json:"payment_subject,omitempty"`
	PaymentMode    *string       `json:"payment_mode,omitempty"`
	IsActive       *bool         `json:"is_active,omitempty"`
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, offset := parsePagination(r)
	active := parseOptionalBoolQuery(r, "active")
	products, err := h.deps.Payment.ListProducts(r.Context(), active, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list payment products failed")
		return
	}
	hasMore := len(products) > limit
	if hasMore {
		products = products[:limit]
	}
	items := make([]PaymentProductDTO, 0, len(products))
	for _, product := range products {
		items = append(items, newProductDTO(product))
	}
	writeJSON(w, http.StatusOK, listResponse[PaymentProductDTO]{
		Items:      items,
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
	})
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	product, err := h.deps.Payment.GetProductAdmin(r.Context(), id)
	if err != nil {
		writeNotFoundOr500(w, err, "get payment product failed")
		return
	}
	writeJSON(w, http.StatusOK, newProductDTO(product))
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	var req createProductRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	product, err := h.deps.Payment.CreateProduct(r.Context(), paymentservice.CreateProductInput{
		Code:           req.Code,
		Title:          req.Title,
		Amount:         req.Amount,
		Currency:       domain.Currency(req.Currency),
		Credits:        req.Credits,
		VATCode:        req.VATCode,
		PaymentSubject: req.PaymentSubject,
		PaymentMode:    req.PaymentMode,
		IsActive:       active,
	})
	if err != nil {
		h.writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newProductDTO(product))
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req updateProductRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	var currency *domain.Currency
	if req.Currency != nil {
		value := domain.Currency(strings.TrimSpace(*req.Currency))
		currency = &value
	}
	product, err := h.deps.Payment.UpdateProduct(r.Context(), paymentservice.UpdateProductInput{
		ID:             id,
		Title:          req.Title,
		Amount:         req.Amount,
		Currency:       currency,
		Credits:        req.Credits,
		VATCodeSet:     req.VATCode.Set,
		VATCode:        req.VATCode.Value,
		PaymentSubject: req.PaymentSubject,
		PaymentMode:    req.PaymentMode,
		IsActive:       req.IsActive,
	})
	if err != nil {
		h.writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newProductDTO(product))
}

func (h *Handler) disableProduct(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	product, err := h.deps.Payment.DisableProduct(r.Context(), id)
	if err != nil {
		h.writeProductError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newProductDTO(product))
}

type createIntentRequest struct {
	ProductCode  string `json:"product_code"`
	ReceiptEmail string `json:"receipt_email,omitempty"`
	ReceiptPhone string `json:"receipt_phone,omitempty"`
	ReturnURL    string `json:"return_url,omitempty"`
	Capture      *bool  `json:"capture,omitempty"`
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
		Capture:        req.Capture,
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
	id, ok := h.paymentIntentIDFromPath(w, r)
	if !ok {
		return
	}
	if _, ok := parseOperatorPaymentActionRequest(w, r); !ok {
		return
	}
	intent, err := h.deps.PaymentOps.SyncIntent(r.Context(), id)
	if err != nil {
		h.writePaymentActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newIntentDTO(intent, true))
}

func (h *Handler) setMockPaymentStatus(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.AllowLoadTestMockPayments {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if h.deps.PaymentOps == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, ok := h.paymentIntentIDFromPath(w, r)
	if !ok {
		return
	}
	actionReq, ok := parseOperatorMockStatusRequest(w, r)
	if !ok {
		return
	}
	intent, err := h.deps.PaymentOps.ForceMockPaymentStatusForLoadTest(r.Context(), id, actionReq.Status)
	if err != nil {
		h.writePaymentActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newIntentDTO(intent, true))
}

func (h *Handler) cancelIntent(w http.ResponseWriter, r *http.Request) {
	if h.deps.PaymentOps == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, ok := h.paymentIntentIDFromPath(w, r)
	if !ok {
		return
	}
	if _, ok := parseOperatorPaymentActionRequest(w, r); !ok {
		return
	}
	intent, err := h.deps.PaymentOps.CancelIntent(r.Context(), id)
	if err != nil {
		h.writePaymentActionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newIntentDTO(intent, true))
}

func (h *Handler) refundIntent(w http.ResponseWriter, r *http.Request) {
	if h.deps.PaymentOps == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	id, ok := h.paymentIntentIDFromPath(w, r)
	if !ok {
		return
	}
	actionReq, ok := parseOperatorPaymentActionRequest(w, r)
	if !ok {
		return
	}
	result, err := h.deps.PaymentOps.RefundIntent(r.Context(), paymentservice.RefundIntentInput{
		IntentID:       id,
		IdempotencyKey: "billing_refund:" + id.String() + ":" + actionReq.IdempotencyKey,
		Reason:         actionReq.Reason,
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

func (h *Handler) paymentIntentIDFromPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := parseOperatorPaymentActionRef(h.cfg.Token, r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

type operatorPaymentActionRequest struct {
	IdempotencyKey string
	Reason         string
}

type operatorPaymentActionBody struct {
	Reason string `json:"reason,omitempty"`
}

type operatorMockStatusActionRequest struct {
	operatorPaymentActionRequest
	Status domain.PaymentIntentStatus
}

type operatorMockStatusActionBody struct {
	Reason string `json:"reason,omitempty"`
	Status string `json:"status"`
}

func parseOperatorPaymentActionRequest(w http.ResponseWriter, r *http.Request) (operatorPaymentActionRequest, bool) {
	clientKey := strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	if clientKey == "" {
		writeError(w, http.StatusBadRequest, "X-Idempotency-Key is required")
		return operatorPaymentActionRequest{}, false
	}
	var body operatorPaymentActionBody
	if r.Body == nil || r.ContentLength == 0 {
		writeError(w, http.StatusBadRequest, "reason is required")
		return operatorPaymentActionRequest{}, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return operatorPaymentActionRequest{}, false
	}
	reason := strings.TrimSpace(body.Reason)
	if !validOperatorReason(reason) {
		writeError(w, http.StatusBadRequest, "reason is required")
		return operatorPaymentActionRequest{}, false
	}
	return operatorPaymentActionRequest{IdempotencyKey: clientKey, Reason: reason}, true
}

func parseOperatorMockStatusRequest(w http.ResponseWriter, r *http.Request) (operatorMockStatusActionRequest, bool) {
	clientKey := strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	if clientKey == "" {
		writeError(w, http.StatusBadRequest, "X-Idempotency-Key is required")
		return operatorMockStatusActionRequest{}, false
	}
	var body operatorMockStatusActionBody
	if r.Body == nil || r.ContentLength == 0 {
		writeError(w, http.StatusBadRequest, "reason and status are required")
		return operatorMockStatusActionRequest{}, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return operatorMockStatusActionRequest{}, false
	}
	reason := strings.TrimSpace(body.Reason)
	if !validOperatorReason(reason) {
		writeError(w, http.StatusBadRequest, "reason is required")
		return operatorMockStatusActionRequest{}, false
	}
	status := domain.PaymentIntentStatus(strings.TrimSpace(body.Status))
	if !status.Valid() {
		writeError(w, http.StatusBadRequest, "invalid status")
		return operatorMockStatusActionRequest{}, false
	}
	return operatorMockStatusActionRequest{
		operatorPaymentActionRequest: operatorPaymentActionRequest{IdempotencyKey: clientKey, Reason: reason},
		Status:                       status,
	}, true
}

func validOperatorReason(reason string) bool {
	if len(reason) < 3 || len(reason) > 500 {
		return false
	}
	if strings.Contains(reason, "://") {
		return false
	}
	for _, r := range reason {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
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

func (h *Handler) listPendingIntents(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, offset := parsePagination(r)
	staleAfter := parseDurationQuery(r, "stale_after", 30*time.Second, 24*time.Hour)
	staleCutoff := time.Now().Add(-staleAfter)
	staleOnly := parseBoolQuery(r, "stale_only", false)
	filter := domain.PaymentIntentFilter{
		Statuses: []domain.PaymentIntentStatus{
			domain.PaymentIntentProviderPending,
			domain.PaymentIntentWaitingForUser,
		},
	}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" {
		filter.Provider = domain.PaymentProviderCode(provider)
	}
	if staleOnly {
		filter.UpdatedBefore = &staleCutoff
	}
	intents, err := h.deps.Payment.ListIntents(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list pending payment intents failed")
		return
	}
	hasMore := len(intents) > limit
	if hasMore {
		intents = intents[:limit]
	}
	items := make([]PaymentIntentDTO, 0, len(intents))
	for _, intent := range intents {
		dto := newIntentDTO(intent, true)
		dto.Stale = !intent.UpdatedAt.After(staleCutoff)
		if dto.Stale {
			dto.StaleSeconds = int64(time.Since(intent.UpdatedAt).Seconds())
		}
		items = append(items, dto)
	}
	writeJSON(w, http.StatusOK, pendingIntentsResponse{
		Items:             items,
		Pagination:        pagination{Limit: limit, Offset: offset, Count: len(items), HasMore: hasMore},
		StaleAfterSeconds: int64(staleAfter.Seconds()),
		StaleOnly:         staleOnly,
	})
}

func (h *Handler) listUnprocessedEvents(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, offset := parsePagination(r)
	unprocessed := false
	filter := domain.PaymentEventFilter{Processed: &unprocessed}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" {
		filter.Provider = domain.PaymentProviderCode(provider)
	}
	events, err := h.deps.Payment.ListEvents(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list payment events failed")
		return
	}
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	items := make([]PaymentEventDTO, 0, len(events))
	for _, event := range events {
		items = append(items, newEventDTO(event))
	}
	writeJSON(w, http.StatusOK, listResponse[PaymentEventDTO]{
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
	case errors.Is(err, paymentservice.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, paymentservice.ErrRefundCreditsSpent),
		errors.Is(err, paymentservice.ErrRefundNotAllowed),
		errors.Is(err, paymentservice.ErrWebhookMismatch),
		errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "payment action conflict")
	case errors.Is(err, paymentservice.ErrWebhookUnverified):
		writeError(w, http.StatusBadGateway, "payment provider verification failed")
	default:
		writeError(w, http.StatusInternalServerError, "payment action failed")
	}
}

func (h *Handler) writeProductError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, paymentservice.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid product")
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "product conflict")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, "product action failed")
	}
}

type PaymentProductDTO struct {
	ID             uuid.UUID `json:"id"`
	Code           string    `json:"code"`
	Title          string    `json:"title"`
	Amount         int64     `json:"amount"`
	Currency       string    `json:"currency"`
	Credits        int64     `json:"credits"`
	PriceVersion   int       `json:"price_version"`
	VATCode        *int16    `json:"vat_code,omitempty"`
	PaymentSubject string    `json:"payment_subject"`
	PaymentMode    string    `json:"payment_mode"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	Stale             bool      `json:"stale,omitempty"`
	StaleSeconds      int64     `json:"stale_seconds,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PaymentEventDTO struct {
	ID                uuid.UUID  `json:"id"`
	Provider          string     `json:"provider"`
	EventType         string     `json:"event_type"`
	ProviderPaymentID string     `json:"provider_payment_id,omitempty"`
	ProviderRefundID  string     `json:"provider_refund_id,omitempty"`
	DedupKey          string     `json:"dedup_key,omitempty"`
	ProcessedAt       *time.Time `json:"processed_at,omitempty"`
	ReceivedAt        time.Time  `json:"received_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
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

type pendingIntentsResponse struct {
	Items             []PaymentIntentDTO `json:"items"`
	Pagination        pagination         `json:"pagination"`
	StaleAfterSeconds int64              `json:"stale_after_seconds"`
	StaleOnly         bool               `json:"stale_only"`
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

func newProductDTO(product *domain.PaymentProduct) PaymentProductDTO {
	if product == nil {
		return PaymentProductDTO{}
	}
	return PaymentProductDTO{
		ID:             product.ID,
		Code:           product.Code,
		Title:          product.Title,
		Amount:         product.Amount,
		Currency:       string(product.Currency),
		Credits:        product.Credits,
		PriceVersion:   product.PriceVersion,
		VATCode:        product.VATCode,
		PaymentSubject: product.PaymentSubject,
		PaymentMode:    product.PaymentMode,
		IsActive:       product.IsActive,
		CreatedAt:      product.CreatedAt,
		UpdatedAt:      product.UpdatedAt,
	}
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

func newEventDTO(event *domain.PaymentEvent) PaymentEventDTO {
	if event == nil {
		return PaymentEventDTO{}
	}
	return PaymentEventDTO{
		ID:                event.ID,
		Provider:          string(event.Provider),
		EventType:         event.EventType,
		ProviderPaymentID: event.ProviderPaymentID,
		ProviderRefundID:  event.ProviderRefundID,
		DedupKey:          event.DedupKey,
		ProcessedAt:       event.ProcessedAt,
		ReceivedAt:        event.ReceivedAt,
		UpdatedAt:         event.UpdatedAt,
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

func parseBoolQuery(r *http.Request, key string, fallback bool) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseOptionalBoolQuery(r *http.Request, key string) *bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &value
}

func parseDurationQuery(r *http.Request, key string, fallback, max time.Duration) time.Duration {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		if seconds, parseErr := strconv.Atoi(raw); parseErr == nil && seconds > 0 {
			duration = time.Duration(seconds) * time.Second
		} else {
			return fallback
		}
	}
	if duration <= 0 {
		return fallback
	}
	if max > 0 && duration > max {
		return max
	}
	return duration
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
