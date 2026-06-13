package billing

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

const defaultOperatorStaleAfter = 5 * time.Minute

// OperatorConsoleDTO is a read-only, display-safe payment and ledger snapshot
// for the admin UI. It intentionally omits raw UUIDs, YooKassa payloads,
// confirmation URLs, provider-native errors and idempotency keys.
type OperatorConsoleDTO struct {
	GeneratedAt    time.Time                        `json:"generated_at"`
	Intents        []OperatorPaymentIntentDTO       `json:"intents"`
	Events         []OperatorPaymentEventDTO        `json:"events"`
	Refunds        []OperatorPaymentRefundDTO       `json:"refunds"`
	Reconciliation OperatorPaymentReconciliationDTO `json:"reconciliation"`
	Billing        *OperatorBillingDTO              `json:"billing,omitempty"`
	Pagination     pagination                       `json:"pagination"`
}

type OperatorPaymentIntentDTO struct {
	DisplayID          string    `json:"display_id"`
	ActionRef          string    `json:"action_ref,omitempty"`
	UserRef            string    `json:"user_ref"`
	ProductRef         string    `json:"product_ref,omitempty"`
	Status             string    `json:"status"`
	Amount             int64     `json:"amount"`
	Currency           string    `json:"currency"`
	Credits            int64     `json:"credits"`
	Provider           string    `json:"provider"`
	ProviderPaymentRef string    `json:"provider_payment_ref,omitempty"`
	ConfirmationState  string    `json:"confirmation_state"`
	CaptureState       string    `json:"capture_state"`
	CancelState        string    `json:"cancel_state"`
	RefundState        string    `json:"refund_state"`
	Stale              bool      `json:"stale"`
	StaleSeconds       int64     `json:"stale_seconds,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type OperatorPaymentEventDTO struct {
	DisplayID          string     `json:"display_id"`
	Provider           string     `json:"provider"`
	EventType          string     `json:"event_type"`
	ProviderPaymentRef string     `json:"provider_payment_ref,omitempty"`
	ProviderRefundRef  string     `json:"provider_refund_ref,omitempty"`
	Processed          bool       `json:"processed"`
	ProcessedAt        *time.Time `json:"processed_at,omitempty"`
	ReceivedAt         time.Time  `json:"received_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type OperatorPaymentRefundDTO struct {
	DisplayID         string    `json:"display_id"`
	IntentRef         string    `json:"intent_ref"`
	ProviderRefundRef string    `json:"provider_refund_ref,omitempty"`
	Amount            int64     `json:"amount"`
	Status            string    `json:"status"`
	ReasonPresent     bool      `json:"reason_present"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type OperatorPaymentReconciliationDTO struct {
	Status                string `json:"status"`
	PendingCount          int    `json:"pending_count"`
	StaleCount            int    `json:"stale_count"`
	UnprocessedEventCount int    `json:"unprocessed_event_count"`
	RefundCount           int    `json:"refund_count"`
	StaleAfterSeconds     int64  `json:"stale_after_seconds"`
}

type OperatorBillingDTO struct {
	UserRef        string                          `json:"user_ref"`
	BalanceCredits int64                           `json:"balance_credits"`
	Ledger         []OperatorLedgerEntryDTO        `json:"ledger"`
	Reservations   []OperatorBillingReservationDTO `json:"reservations"`
}

type OperatorLedgerEntryDTO struct {
	DisplayID      string    `json:"display_id"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Amount         int64     `json:"amount"`
	JobRef         string    `json:"job_ref,omitempty"`
	ReservationRef string    `json:"reservation_ref,omitempty"`
	ReasonClass    string    `json:"reason_class,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type OperatorBillingReservationDTO struct {
	DisplayID string    `json:"display_id"`
	JobRef    string    `json:"job_ref"`
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	ExpiresAt time.Time `json:"expires_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *Handler) operatorConsole(w http.ResponseWriter, r *http.Request) {
	if h.deps.Payment == nil {
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return
	}
	limit, offset := parsePagination(r)
	now := time.Now()
	staleAfter := parseDurationQuery(r, "stale_after", defaultOperatorStaleAfter, 24*time.Hour)
	staleCutoff := now.Add(-staleAfter)
	filter, userID, ok := parseOperatorPaymentFilter(w, r)
	if !ok {
		return
	}

	intents, err := h.deps.Payment.ListIntents(r.Context(), filter, limit+1, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list payment console intents failed")
		return
	}
	hasMore := len(intents) > limit
	if hasMore {
		intents = intents[:limit]
	}
	intentItems := make([]OperatorPaymentIntentDTO, 0, len(intents))
	for _, intent := range intents {
		intentItems = append(intentItems, h.newOperatorPaymentIntentDTO(intent, now, staleCutoff))
	}

	events, err := h.deps.Payment.ListEvents(r.Context(), operatorPaymentEventFilter(filter), limit+1, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list payment console events failed")
		return
	}
	if len(events) > limit {
		events = events[:limit]
	}
	eventItems := make([]OperatorPaymentEventDTO, 0, len(events))
	for _, event := range events {
		eventItems = append(eventItems, newOperatorPaymentEventDTO(event))
	}

	refunds, err := h.deps.Payment.ListRefunds(r.Context(), domain.PaymentRefundFilter{}, limit+1, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list payment console refunds failed")
		return
	}
	if len(refunds) > limit {
		refunds = refunds[:limit]
	}
	refundItems := make([]OperatorPaymentRefundDTO, 0, len(refunds))
	for _, refund := range refunds {
		refundItems = append(refundItems, newOperatorPaymentRefundDTO(refund))
	}

	dto := OperatorConsoleDTO{
		GeneratedAt: now,
		Intents:     intentItems,
		Events:      eventItems,
		Refunds:     refundItems,
		Reconciliation: OperatorPaymentReconciliationDTO{
			Status:                paymentReconciliationStatus(intentItems, eventItems),
			PendingCount:          countOperatorPendingIntents(intentItems),
			StaleCount:            countOperatorStaleIntents(intentItems),
			UnprocessedEventCount: countOperatorUnprocessedEvents(eventItems),
			RefundCount:           len(refundItems),
			StaleAfterSeconds:     int64(staleAfter.Seconds()),
		},
		Pagination: pagination{Limit: limit, Offset: offset, Count: len(intentItems), HasMore: hasMore},
	}
	if userID != nil && h.deps.Billing != nil {
		billingDTO, err := h.operatorBillingSnapshot(r.Context(), *userID, limit)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "load billing snapshot failed")
			return
		}
		dto.Billing = billingDTO
	}
	writeJSON(w, http.StatusOK, dto)
}

func parseOperatorPaymentFilter(w http.ResponseWriter, r *http.Request) (domain.PaymentIntentFilter, *uuid.UUID, bool) {
	var filter domain.PaymentIntentFilter
	var userID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("user_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return filter, nil, false
		}
		filter.UserID = &id
		userID = &id
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		parsed := domain.PaymentIntentStatus(status)
		if !parsed.Valid() {
			writeError(w, http.StatusBadRequest, "invalid status")
			return filter, nil, false
		}
		filter.Status = parsed
	}
	if provider := strings.TrimSpace(r.URL.Query().Get("provider")); provider != "" {
		parsed := domain.PaymentProviderCode(provider)
		if !parsed.Valid() {
			writeError(w, http.StatusBadRequest, "invalid provider")
			return filter, nil, false
		}
		filter.Provider = parsed
	}
	return filter, userID, true
}

func operatorPaymentEventFilter(intentFilter domain.PaymentIntentFilter) domain.PaymentEventFilter {
	filter := domain.PaymentEventFilter{}
	if intentFilter.Provider != "" {
		filter.Provider = intentFilter.Provider
	}
	return filter
}

func (h *Handler) operatorBillingSnapshot(ctx context.Context, userID uuid.UUID, limit int) (*OperatorBillingDTO, error) {
	account, err := h.deps.Billing.GetAccountByUser(ctx, userID, domain.CurrencyCredits)
	if err != nil {
		return nil, err
	}
	entries, err := h.deps.Billing.ListEntries(ctx, account.ID, limit+1, 0)
	if err != nil {
		return nil, err
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	ledger := make([]OperatorLedgerEntryDTO, 0, len(entries))
	reservations := make([]OperatorBillingReservationDTO, 0, len(entries))
	seenReservations := map[uuid.UUID]bool{}
	for _, entry := range entries {
		ledger = append(ledger, newOperatorLedgerEntryDTO(entry))
		if entry.ReservationID == nil || seenReservations[*entry.ReservationID] {
			continue
		}
		seenReservations[*entry.ReservationID] = true
		reservation, err := h.deps.Billing.GetReservation(ctx, *entry.ReservationID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return nil, err
		}
		reservations = append(reservations, newOperatorBillingReservationDTO(reservation))
	}
	return &OperatorBillingDTO{
		UserRef:        safeUUIDRef("user", userID),
		BalanceCredits: account.BalanceCached,
		Ledger:         ledger,
		Reservations:   reservations,
	}, nil
}

func (h *Handler) newOperatorPaymentIntentDTO(intent *domain.PaymentIntent, now, staleCutoff time.Time) OperatorPaymentIntentDTO {
	if intent == nil {
		return OperatorPaymentIntentDTO{}
	}
	stale := (intent.Status == domain.PaymentIntentProviderPending || intent.Status == domain.PaymentIntentWaitingForUser) &&
		!intent.UpdatedAt.After(staleCutoff)
	dto := OperatorPaymentIntentDTO{
		DisplayID:          safeUUIDRef("pay", intent.ID),
		ActionRef:          newOperatorPaymentActionRef(h.cfg.Token, intent.ID),
		UserRef:            safeUUIDRef("user", intent.UserID),
		Status:             string(intent.Status),
		Amount:             intent.Amount,
		Currency:           string(intent.Currency),
		Credits:            intent.Credits,
		Provider:           string(intent.Provider),
		ProviderPaymentRef: safeStringRef("provider_payment", intent.ProviderPaymentID),
		ConfirmationState:  confirmationState(intent.ConfirmationURL),
		CaptureState:       captureState(intent.Status),
		CancelState:        cancelState(intent.Status),
		RefundState:        refundState(intent.Status),
		Stale:              stale,
		CreatedAt:          intent.CreatedAt,
		UpdatedAt:          intent.UpdatedAt,
	}
	if intent.ProductID != nil {
		dto.ProductRef = safeUUIDRef("product", *intent.ProductID)
	}
	if stale {
		dto.StaleSeconds = int64(now.Sub(intent.UpdatedAt).Seconds())
	}
	return dto
}

func newOperatorPaymentEventDTO(event *domain.PaymentEvent) OperatorPaymentEventDTO {
	if event == nil {
		return OperatorPaymentEventDTO{}
	}
	return OperatorPaymentEventDTO{
		DisplayID:          safeUUIDRef("event", event.ID),
		Provider:           string(event.Provider),
		EventType:          sanitizeOperatorToken(event.EventType),
		ProviderPaymentRef: safeStringRef("provider_payment", event.ProviderPaymentID),
		ProviderRefundRef:  safeStringRef("provider_refund", event.ProviderRefundID),
		Processed:          event.ProcessedAt != nil,
		ProcessedAt:        event.ProcessedAt,
		ReceivedAt:         event.ReceivedAt,
		UpdatedAt:          event.UpdatedAt,
	}
}

func newOperatorPaymentRefundDTO(refund *domain.PaymentRefund) OperatorPaymentRefundDTO {
	if refund == nil {
		return OperatorPaymentRefundDTO{}
	}
	return OperatorPaymentRefundDTO{
		DisplayID:         safeUUIDRef("refund", refund.ID),
		IntentRef:         safeUUIDRef("pay", refund.IntentID),
		ProviderRefundRef: safeStringRef("provider_refund", refund.ProviderRefundID),
		Amount:            refund.Amount,
		Status:            string(refund.Status),
		ReasonPresent:     strings.TrimSpace(refund.Reason) != "",
		CreatedAt:         refund.CreatedAt,
		UpdatedAt:         refund.UpdatedAt,
	}
}

func newOperatorLedgerEntryDTO(entry *domain.LedgerEntry) OperatorLedgerEntryDTO {
	if entry == nil {
		return OperatorLedgerEntryDTO{}
	}
	dto := OperatorLedgerEntryDTO{
		DisplayID:   safeUUIDRef("ledger", entry.ID),
		Type:        string(entry.Type),
		Status:      string(entry.Status),
		Amount:      entry.Amount,
		ReasonClass: ledgerReasonClass(entry.Reason),
		CreatedAt:   entry.CreatedAt,
	}
	if entry.JobID != nil {
		dto.JobRef = safeUUIDRef("job", *entry.JobID)
	}
	if entry.ReservationID != nil {
		dto.ReservationRef = safeUUIDRef("reservation", *entry.ReservationID)
	}
	return dto
}

func newOperatorBillingReservationDTO(reservation *domain.CreditReservation) OperatorBillingReservationDTO {
	if reservation == nil {
		return OperatorBillingReservationDTO{}
	}
	return OperatorBillingReservationDTO{
		DisplayID: safeUUIDRef("reservation", reservation.ID),
		JobRef:    safeUUIDRef("job", reservation.JobID),
		Status:    string(reservation.Status),
		Amount:    reservation.Amount,
		ExpiresAt: reservation.ExpiresAt,
		UpdatedAt: reservation.UpdatedAt,
	}
}

func confirmationState(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "none"
	}
	return "available"
}

func captureState(status domain.PaymentIntentStatus) string {
	switch status {
	case domain.PaymentIntentSucceeded, domain.PaymentIntentPartiallyRefunded, domain.PaymentIntentRefunded:
		return "captured"
	case domain.PaymentIntentCanceled, domain.PaymentIntentExpired, domain.PaymentIntentFailed:
		return "closed_without_capture"
	case domain.PaymentIntentCreated, domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser:
		return "open"
	default:
		return "unknown"
	}
}

func cancelState(status domain.PaymentIntentStatus) string {
	switch status {
	case domain.PaymentIntentCreated, domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser:
		return "cancelable_by_operator_endpoint"
	case domain.PaymentIntentCanceled:
		return "canceled"
	case domain.PaymentIntentSucceeded, domain.PaymentIntentExpired, domain.PaymentIntentFailed, domain.PaymentIntentRefunded, domain.PaymentIntentPartiallyRefunded:
		return "terminal"
	default:
		return "unknown"
	}
}

func refundState(status domain.PaymentIntentStatus) string {
	switch status {
	case domain.PaymentIntentSucceeded, domain.PaymentIntentPartiallyRefunded:
		return "eligible_policy_check_required"
	case domain.PaymentIntentRefunded:
		return "refunded"
	case domain.PaymentIntentCreated, domain.PaymentIntentProviderPending, domain.PaymentIntentWaitingForUser, domain.PaymentIntentCanceled, domain.PaymentIntentExpired, domain.PaymentIntentFailed:
		return "unavailable"
	default:
		return "unknown"
	}
}

func paymentReconciliationStatus(intents []OperatorPaymentIntentDTO, events []OperatorPaymentEventDTO) string {
	if countOperatorStaleIntents(intents) > 0 || countOperatorUnprocessedEvents(events) > 0 {
		return "needs_attention"
	}
	return "ok"
}

func countOperatorPendingIntents(items []OperatorPaymentIntentDTO) int {
	count := 0
	for _, item := range items {
		if item.Status == string(domain.PaymentIntentProviderPending) || item.Status == string(domain.PaymentIntentWaitingForUser) {
			count++
		}
	}
	return count
}

func countOperatorStaleIntents(items []OperatorPaymentIntentDTO) int {
	count := 0
	for _, item := range items {
		if item.Stale {
			count++
		}
	}
	return count
}

func countOperatorUnprocessedEvents(items []OperatorPaymentEventDTO) int {
	count := 0
	for _, item := range items {
		if !item.Processed {
			count++
		}
	}
	return count
}

func ledgerReasonClass(reason string) string {
	value := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case value == "":
		return ""
	case strings.Contains(value, "refund"):
		return "refund"
	case strings.Contains(value, "topup") || strings.Contains(value, "top-up") || strings.Contains(value, "payment"):
		return "topup"
	case strings.Contains(value, "reserve"):
		return "reservation"
	case strings.Contains(value, "capture"):
		return "capture"
	case strings.Contains(value, "release"):
		return "release"
	default:
		return "other"
	}
}

func sanitizeOperatorToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
			if b.Len() >= 64 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return "other"
	}
	return b.String()
}

const operatorPaymentActionRefPrefix = "opact_v1_"

func newOperatorPaymentActionRef(adminToken string, id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	block, err := aes.NewCipher(operatorPaymentActionRefKey(adminToken))
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	sealed := gcm.Seal(nonce, nonce, []byte(id.String()), nil)
	return operatorPaymentActionRefPrefix + base64.RawURLEncoding.EncodeToString(sealed)
}

func parseOperatorPaymentActionRef(adminToken, value string) (uuid.UUID, bool) {
	value = strings.TrimSpace(value)
	if id, err := uuid.Parse(value); err == nil && id != uuid.Nil {
		return id, true
	}
	if !strings.HasPrefix(value, operatorPaymentActionRefPrefix) {
		return uuid.Nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, operatorPaymentActionRefPrefix))
	if err != nil {
		return uuid.Nil, false
	}
	block, err := aes.NewCipher(operatorPaymentActionRefKey(adminToken))
	if err != nil {
		return uuid.Nil, false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return uuid.Nil, false
	}
	nonceSize := gcm.NonceSize()
	if len(raw) <= nonceSize {
		return uuid.Nil, false
	}
	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(string(plaintext))
	return id, err == nil && id != uuid.Nil
}

func operatorPaymentActionRefKey(adminToken string) []byte {
	seed := strings.TrimSpace(adminToken)
	if seed == "" {
		seed = "dev-admin-operator-action-ref"
	}
	sum := sha256.Sum256([]byte("vk-ai-aggregator:operator-payment-action-ref:v1:" + seed))
	return sum[:]
}

func safeUUIDRef(prefix string, id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return safeStringRef(prefix, id.String())
}

func safeStringRef(prefix, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return prefix + "_" + hex.EncodeToString(sum[:])[:12]
}
