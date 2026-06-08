// Package yookassa implements the YooKassa payment provider adapter.
package yookassa

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"vk-ai-aggregator/internal/domain"
)

const (
	defaultBaseURL         = "https://api.yookassa.ru/v3"
	defaultHTTPTimeout     = 15 * time.Second
	defaultPaymentSubject  = "service"
	defaultPaymentMode     = "full_payment"
	defaultVATCode         = int16(1)
	maxHTTPIdempotencySize = 64
	maxResponseBytes       = 1 << 20
)

// Config contains only YooKassa transport credentials and endpoints. Secrets
// must be supplied from runtime config and must never be logged.
type Config struct {
	ShopID    string
	SecretKey string
	BaseURL   string
	ReturnURL string

	HTTPClient *http.Client
}

// Provider is a YooKassa implementation of domain.PaymentProvider.
type Provider struct {
	shopID    string
	secretKey string
	baseURL   string
	returnURL string
	client    *http.Client
}

var _ domain.PaymentProvider = (*Provider)(nil)

// New builds a YooKassa payment provider adapter.
func New(cfg Config) (*Provider, error) {
	shopID := strings.TrimSpace(cfg.ShopID)
	secretKey := strings.TrimSpace(cfg.SecretKey)
	returnURL := strings.TrimSpace(cfg.ReturnURL)
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if shopID == "" {
		return nil, errors.New("yookassa payment: shop id is required")
	}
	if secretKey == "" {
		return nil, errors.New("yookassa payment: secret key is required")
	}
	if returnURL == "" {
		return nil, errors.New("yookassa payment: return url is required")
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Provider{
		shopID:    shopID,
		secretKey: secretKey,
		baseURL:   baseURL,
		returnURL: returnURL,
		client:    client,
	}, nil
}

// Code returns the payment provider code.
func (p *Provider) Code() domain.PaymentProviderCode {
	return domain.PaymentProviderYooKassa
}

// CreatePayment creates a captured redirect payment in YooKassa.
func (p *Provider) CreatePayment(ctx context.Context, in domain.CreatePaymentInput) (domain.CreatePaymentResult, error) {
	if in.Amount <= 0 {
		return domain.CreatePaymentResult{}, errors.New("yookassa payment: amount must be positive")
	}
	if err := validateHTTPIdempotencyKey(in.IdempotencyKey); err != nil {
		return domain.CreatePaymentResult{}, err
	}
	receipt, err := newReceipt(in)
	if err != nil {
		return domain.CreatePaymentResult{}, err
	}
	value, err := formatKopecks(in.Amount)
	if err != nil {
		return domain.CreatePaymentResult{}, err
	}
	currency := yooCurrency(in.Currency)
	returnURL := strings.TrimSpace(in.ReturnURL)
	if returnURL == "" {
		returnURL = p.returnURL
	}

	request := createPaymentRequest{
		Amount: amountValue{
			Value:    value,
			Currency: currency,
		},
		Capture: true,
		Confirmation: confirmationRequest{
			Type:      "redirect",
			ReturnURL: returnURL,
		},
		Description: descriptionOrDefault(in.Description),
		Metadata:    metadataForPayment(in),
		Receipt:     receipt,
	}

	var response paymentResponse
	raw, err := p.doJSON(ctx, http.MethodPost, "/payments", in.IdempotencyKey, request, &response)
	if err != nil {
		return domain.CreatePaymentResult{}, err
	}
	return domain.CreatePaymentResult{
		ProviderPaymentID: response.ID,
		ConfirmationURL:   response.Confirmation.ConfirmationURL,
		Status:            mapPaymentStatus(response),
		Raw:               raw,
	}, nil
}

// GetPayment fetches and normalizes the current YooKassa payment state.
func (p *Provider) GetPayment(ctx context.Context, providerPaymentID string) (domain.ProviderPayment, error) {
	providerPaymentID = strings.TrimSpace(providerPaymentID)
	if providerPaymentID == "" {
		return domain.ProviderPayment{}, errors.New("yookassa payment: provider payment id is required")
	}

	var response paymentResponse
	raw, err := p.doJSON(ctx, http.MethodGet, "/payments/"+providerPaymentID, "", nil, &response)
	if err != nil {
		return domain.ProviderPayment{}, err
	}
	amount, err := parseKopecks(response.Amount.Value)
	if err != nil {
		return domain.ProviderPayment{}, fmt.Errorf("yookassa payment: parse amount: %w", err)
	}
	return domain.ProviderPayment{
		ProviderPaymentID: response.ID,
		Status:            mapPaymentStatus(response),
		Amount:            amount,
		Currency:          domainCurrency(response.Amount.Currency),
		Paid:              response.Paid,
		Captured:          response.Paid || response.Status == "succeeded",
		Refundable:        response.Refundable,
		Raw:               raw,
	}, nil
}

// CancelPayment requests cancellation for a provider payment.
func (p *Provider) CancelPayment(ctx context.Context, providerPaymentID string) error {
	providerPaymentID = strings.TrimSpace(providerPaymentID)
	if providerPaymentID == "" {
		return errors.New("yookassa payment: provider payment id is required")
	}

	var response paymentResponse
	_, err := p.doJSON(
		ctx,
		http.MethodPost,
		"/payments/"+providerPaymentID+"/cancel",
		cancelIdempotencyKey(providerPaymentID),
		map[string]any{},
		&response,
	)
	return err
}

// CreateRefund creates a YooKassa refund for an already successful payment.
func (p *Provider) CreateRefund(ctx context.Context, in domain.CreateRefundInput) (domain.RefundResult, error) {
	if strings.TrimSpace(in.ProviderPaymentID) == "" {
		return domain.RefundResult{}, errors.New("yookassa payment: provider payment id is required")
	}
	if in.Amount <= 0 {
		return domain.RefundResult{}, errors.New("yookassa payment: refund amount must be positive")
	}
	if err := validateHTTPIdempotencyKey(in.IdempotencyKey); err != nil {
		return domain.RefundResult{}, err
	}
	receipt, err := newRefundReceipt(in)
	if err != nil {
		return domain.RefundResult{}, err
	}
	value, err := formatKopecks(in.Amount)
	if err != nil {
		return domain.RefundResult{}, err
	}

	request := createRefundRequest{
		PaymentID: strings.TrimSpace(in.ProviderPaymentID),
		Amount: amountValue{
			Value:    value,
			Currency: yooCurrency(in.Currency),
		},
		Description: strings.TrimSpace(in.Reason),
		Metadata:    metadataForRefund(in),
		Receipt:     receipt,
	}

	var response refundResponse
	raw, err := p.doJSON(ctx, http.MethodPost, "/refunds", in.IdempotencyKey, request, &response)
	if err != nil {
		return domain.RefundResult{}, err
	}
	amount, err := parseKopecks(response.Amount.Value)
	if err != nil {
		return domain.RefundResult{}, fmt.Errorf("yookassa payment: parse refund amount: %w", err)
	}
	return domain.RefundResult{
		ProviderRefundID: response.ID,
		Status:           mapRefundStatus(response.Status),
		Amount:           amount,
		Currency:         domainCurrency(response.Amount.Currency),
		Raw:              raw,
	}, nil
}

// ParseWebhook parses YooKassa notification JSON into the internal webhook
// inbox shape. Authenticity checks belong to the HTTP layer before this method
// stores or processes the event.
func (p *Provider) ParseWebhook(ctx context.Context, raw []byte, _ http.Header) (domain.WebhookEvent, error) {
	select {
	case <-ctx.Done():
		return domain.WebhookEvent{}, ctx.Err()
	default:
	}

	var notification webhookNotification
	if err := json.Unmarshal(raw, &notification); err != nil {
		return domain.WebhookEvent{}, fmt.Errorf("yookassa payment: parse webhook: %w", err)
	}
	eventType := strings.TrimSpace(notification.Event)
	if eventType == "" {
		return domain.WebhookEvent{}, errors.New("yookassa payment: webhook event is required")
	}
	if len(notification.Object) == 0 {
		return domain.WebhookEvent{}, errors.New("yookassa payment: webhook object is required")
	}

	var object struct {
		ID        string `json:"id"`
		PaymentID string `json:"payment_id"`
	}
	if err := json.Unmarshal(notification.Object, &object); err != nil {
		return domain.WebhookEvent{}, fmt.Errorf("yookassa payment: parse webhook object: %w", err)
	}

	paymentID := strings.TrimSpace(object.ID)
	refundID := ""
	if strings.HasPrefix(eventType, "refund.") {
		refundID = paymentID
		paymentID = strings.TrimSpace(object.PaymentID)
	}
	if paymentID == "" && refundID == "" {
		return domain.WebhookEvent{}, errors.New("yookassa payment: webhook payment or refund id is required")
	}
	naturalID := paymentID
	if refundID != "" {
		naturalID = refundID
	}
	return domain.WebhookEvent{
		Provider:          domain.PaymentProviderYooKassa,
		EventType:         eventType,
		ProviderPaymentID: paymentID,
		ProviderRefundID:  refundID,
		DedupKey:          "webhook:yookassa:" + eventType + ":" + naturalID,
		Payload:           append(json.RawMessage(nil), raw...),
	}, nil
}

func (p *Provider) doJSON(ctx context.Context, method, path, idempotencyKey string, body any, out any) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("yookassa payment: encode request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("yookassa payment: build request: %w", err)
	}
	req.SetBasicAuth(p.shopID, p.secretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		if err := validateHTTPIdempotencyKey(idempotencyKey); err != nil {
			return nil, err
		}
		req.Header.Set("Idempotence-Key", idempotencyKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yookassa payment: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("yookassa payment: read response: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return nil, errors.New("yookassa payment: response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiErrorFromResponse(resp.StatusCode, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return nil, fmt.Errorf("yookassa payment: decode response: %w", err)
		}
	}
	return append(json.RawMessage(nil), raw...), nil
}

type amountValue struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type confirmationRequest struct {
	Type      string `json:"type"`
	ReturnURL string `json:"return_url"`
}

type createPaymentRequest struct {
	Amount       amountValue         `json:"amount"`
	Capture      bool                `json:"capture"`
	Confirmation confirmationRequest `json:"confirmation"`
	Description  string              `json:"description,omitempty"`
	Metadata     map[string]any      `json:"metadata,omitempty"`
	Receipt      receipt             `json:"receipt"`
}

type receipt struct {
	Customer receiptCustomer `json:"customer"`
	Items    []receiptItem   `json:"items"`
}

type receiptCustomer struct {
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

type receiptItem struct {
	Description    string      `json:"description"`
	Quantity       float64     `json:"quantity"`
	Amount         amountValue `json:"amount"`
	VATCode        int16       `json:"vat_code"`
	PaymentSubject string      `json:"payment_subject,omitempty"`
	PaymentMode    string      `json:"payment_mode,omitempty"`
}

type createRefundRequest struct {
	PaymentID   string         `json:"payment_id"`
	Amount      amountValue    `json:"amount"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Receipt     receipt        `json:"receipt"`
}

type confirmationResponse struct {
	Type            string `json:"type"`
	ConfirmationURL string `json:"confirmation_url"`
}

type cancellationDetails struct {
	Reason string `json:"reason"`
}

type paymentResponse struct {
	ID                  string               `json:"id"`
	Status              string               `json:"status"`
	Paid                bool                 `json:"paid"`
	Amount              amountValue          `json:"amount"`
	Confirmation        confirmationResponse `json:"confirmation"`
	Refundable          bool                 `json:"refundable"`
	CancellationDetails cancellationDetails  `json:"cancellation_details"`
}

type refundResponse struct {
	ID        string      `json:"id"`
	Status    string      `json:"status"`
	Amount    amountValue `json:"amount"`
	PaymentID string      `json:"payment_id"`
}

type webhookNotification struct {
	Type   string          `json:"type"`
	Event  string          `json:"event"`
	Object json.RawMessage `json:"object"`
}

type providerErrorResponse struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Parameter   string `json:"parameter"`
}

type APIError struct {
	StatusCode  int
	Code        string
	Description string
}

func (e APIError) Error() string {
	if e.Code != "" && e.Description != "" {
		return fmt.Sprintf("yookassa payment: status %d: %s: %s", e.StatusCode, e.Code, e.Description)
	}
	if e.Code != "" {
		return fmt.Sprintf("yookassa payment: status %d: %s", e.StatusCode, e.Code)
	}
	if e.Description != "" {
		return fmt.Sprintf("yookassa payment: status %d: %s", e.StatusCode, e.Description)
	}
	return fmt.Sprintf("yookassa payment: status %d", e.StatusCode)
}

func apiErrorFromResponse(statusCode int, raw []byte) error {
	var response providerErrorResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return APIError{StatusCode: statusCode}
	}
	return APIError{
		StatusCode:  statusCode,
		Code:        strings.TrimSpace(response.Code),
		Description: strings.TrimSpace(response.Description),
	}
}

func newReceipt(in domain.CreatePaymentInput) (receipt, error) {
	email := strings.TrimSpace(in.ReceiptEmail)
	phone := strings.TrimSpace(in.ReceiptPhone)
	if email == "" && phone == "" {
		return receipt{}, errors.New("yookassa payment: receipt email or phone is required")
	}
	vatCode := defaultVATCode
	if in.VATCode != nil {
		vatCode = *in.VATCode
	}
	value, err := formatKopecks(in.Amount)
	if err != nil {
		return receipt{}, err
	}
	return receipt{
		Customer: receiptCustomer{
			Email: email,
			Phone: phone,
		},
		Items: []receiptItem{{
			Description:    descriptionOrDefault(in.Description),
			Quantity:       1,
			Amount:         amountValue{Value: value, Currency: yooCurrency(in.Currency)},
			VATCode:        vatCode,
			PaymentSubject: defaultString(in.PaymentSubject, defaultPaymentSubject),
			PaymentMode:    defaultString(in.PaymentMode, defaultPaymentMode),
		}},
	}, nil
}

func newRefundReceipt(in domain.CreateRefundInput) (receipt, error) {
	email := strings.TrimSpace(in.ReceiptEmail)
	phone := strings.TrimSpace(in.ReceiptPhone)
	if email == "" && phone == "" {
		return receipt{}, errors.New("yookassa payment: refund receipt email or phone is required")
	}
	vatCode := defaultVATCode
	if in.VATCode != nil {
		vatCode = *in.VATCode
	}
	value, err := formatKopecks(in.Amount)
	if err != nil {
		return receipt{}, err
	}
	return receipt{
		Customer: receiptCustomer{
			Email: email,
			Phone: phone,
		},
		Items: []receiptItem{{
			Description:    defaultString(in.Reason, "NeiroHub balance refund"),
			Quantity:       1,
			Amount:         amountValue{Value: value, Currency: yooCurrency(in.Currency)},
			VATCode:        vatCode,
			PaymentSubject: defaultString(in.PaymentSubject, defaultPaymentSubject),
			PaymentMode:    defaultString(in.PaymentMode, defaultPaymentMode),
		}},
	}, nil
}

func metadataForPayment(in domain.CreatePaymentInput) map[string]any {
	metadata := metadataFromRaw(in.Metadata)
	if in.IntentID.String() != "" {
		metadata["intent_id"] = in.IntentID.String()
	}
	if in.UserID.String() != "" {
		metadata["user_id"] = in.UserID.String()
	}
	if in.Credits > 0 {
		metadata["credits"] = in.Credits
	}
	return metadata
}

func metadataForRefund(in domain.CreateRefundInput) map[string]any {
	metadata := metadataFromRaw(in.Metadata)
	if in.RefundID.String() != "" {
		metadata["refund_id"] = in.RefundID.String()
	}
	if in.IntentID.String() != "" {
		metadata["intent_id"] = in.IntentID.String()
	}
	return metadata
}

func metadataFromRaw(raw json.RawMessage) map[string]any {
	metadata := map[string]any{}
	if len(raw) == 0 {
		return metadata
	}
	var extra map[string]any
	if err := json.Unmarshal(raw, &extra); err != nil {
		return metadata
	}
	for key, value := range extra {
		if strings.TrimSpace(key) != "" {
			metadata[key] = value
		}
	}
	return metadata
}

func mapPaymentStatus(payment paymentResponse) domain.PaymentIntentStatus {
	switch payment.Status {
	case "succeeded":
		return domain.PaymentIntentSucceeded
	case "canceled":
		if payment.CancellationDetails.Reason == "expired_on_confirmation" {
			return domain.PaymentIntentExpired
		}
		return domain.PaymentIntentCanceled
	case "waiting_for_capture":
		return domain.PaymentIntentProviderPending
	case "pending":
		if payment.Confirmation.ConfirmationURL != "" {
			return domain.PaymentIntentWaitingForUser
		}
		return domain.PaymentIntentProviderPending
	default:
		return domain.PaymentIntentProviderPending
	}
}

func mapRefundStatus(status string) domain.PaymentRefundStatus {
	switch status {
	case "succeeded":
		return domain.PaymentRefundSucceeded
	case "canceled":
		return domain.PaymentRefundCanceled
	case "pending":
		return domain.PaymentRefundProviderPending
	default:
		return domain.PaymentRefundProviderPending
	}
}

func formatKopecks(amount int64) (string, error) {
	if amount < 0 {
		return "", errors.New("amount must not be negative")
	}
	return fmt.Sprintf("%d.%02d", amount/100, amount%100), nil
}

func parseKopecks(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("empty amount")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid amount %q", value)
	}
	majorPart := parts[0]
	if majorPart == "" {
		majorPart = "0"
	}
	var major int64
	for _, ch := range majorPart {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid amount %q", value)
		}
		major = major*10 + int64(ch-'0')
	}
	minorPart := ""
	if len(parts) == 2 {
		minorPart = parts[1]
	}
	if len(minorPart) > 2 {
		return 0, fmt.Errorf("invalid amount %q", value)
	}
	for len(minorPart) < 2 {
		minorPart += "0"
	}
	var minor int64
	for _, ch := range minorPart {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid amount %q", value)
		}
		minor = minor*10 + int64(ch-'0')
	}
	return major*100 + minor, nil
}

func yooCurrency(currency domain.Currency) string {
	if currency == "" {
		currency = domain.CurrencyRUB
	}
	return strings.ToUpper(string(currency))
}

func domainCurrency(currency string) domain.Currency {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "", "RUB":
		return domain.CurrencyRUB
	default:
		return domain.Currency(strings.ToLower(currency))
	}
}

func descriptionOrDefault(description string) string {
	return defaultString(description, "NeiroHub balance top-up")
}

func defaultString(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func validateHTTPIdempotencyKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("yookassa payment: idempotence key is required")
	}
	if len(key) > maxHTTPIdempotencySize {
		return fmt.Errorf("yookassa payment: idempotence key exceeds %d characters", maxHTTPIdempotencySize)
	}
	return nil
}

func cancelIdempotencyKey(providerPaymentID string) string {
	key := "cancel:" + providerPaymentID
	if len(key) <= maxHTTPIdempotencySize {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	return "cancel:" + hex.EncodeToString(sum[:])[:32]
}
