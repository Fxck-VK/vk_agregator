// Package accounthistory exposes safe account-owned financial history reads.
package accounthistory

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

var ErrInvalidQuery = errors.New("invalid account history query")
var ErrUnavailable = errors.New("account history unavailable")

const maxSafeInteger int64 = 9007199254740991

type Money struct {
	AmountMinor int64           `json:"amount_minor"`
	Currency    domain.Currency `json:"currency"`
}
type Item struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	CreatedAt  time.Time `json:"created_at"`
	StarsDelta int64     `json:"stars_delta"`
	Status     string    `json:"status"`
	Money      *Money    `json:"money,omitempty"`
	PlanCode   string    `json:"plan_code,omitempty"`
	Modality   string    `json:"modality,omitempty"`
	Operation  string    `json:"operation,omitempty"`
	TestMode   *bool     `json:"test_mode,omitempty"`
}
type Page struct {
	Items      []Item  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}
type Service struct {
	repo domain.AccountHistoryRepository
}

func New(repo domain.AccountHistoryRepository) *Service { return &Service{repo: repo} }

type pageCursor struct {
	Version   int                         `json:"v"`
	Owner     uuid.UUID                   `json:"owner"`
	CreatedAt time.Time                   `json:"at"`
	Source    domain.AccountHistorySource `json:"source"`
	ID        uuid.UUID                   `json:"id"`
}

func decodeCursor(raw string, owner uuid.UUID) (*domain.AccountHistoryCursor, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 512 {
		return nil, ErrInvalidQuery
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidQuery
	}
	var c pageCursor
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&c); err != nil {
		return nil, ErrInvalidQuery
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || c.Version != 1 || c.Owner != owner || c.ID == uuid.Nil || c.CreatedAt.IsZero() || c.Source < domain.AccountHistoryLedger || c.Source > domain.AccountHistoryExternalRefund {
		return nil, ErrInvalidQuery
	}
	return &domain.AccountHistoryCursor{CreatedAt: c.CreatedAt, Source: c.Source, ID: c.ID}, nil
}

// List derives no identity from cursor contents. Owner always comes from the
// authenticated principal; the cursor is only an owner-bound read position.
func (s *Service) List(ctx context.Context, owner uuid.UUID, limit int, cursor string) (Page, error) {
	if owner == uuid.Nil {
		return Page{}, domain.ErrForbidden
	}
	if s == nil || s.repo == nil {
		return Page{}, ErrUnavailable
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return Page{}, ErrInvalidQuery
	}
	after, err := decodeCursor(cursor, owner)
	if err != nil {
		return Page{}, err
	}
	rows, err := s.repo.ListAccountHistory(ctx, owner, limit+1, after)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: make([]Item, 0, min(len(rows), limit))}
	for i, row := range rows {
		if i == limit {
			break
		}
		item, err := safeItem(row)
		if err != nil {
			return Page{}, err
		}
		page.Items = append(page.Items, item)
	}
	if len(rows) > limit {
		last := rows[limit-1]
		data, err := json.Marshal(pageCursor{Version: 1, Owner: owner, CreatedAt: last.CreatedAt, Source: last.Source, ID: last.ID})
		if err != nil {
			return Page{}, err
		}
		next := base64.RawURLEncoding.EncodeToString(data)
		page.NextCursor = &next
	}
	return page, nil
}

func safeItem(row domain.AccountHistoryRow) (Item, error) {
	prefixes := []string{"ledger:", "payment:", "refund:", "external_refund:"}
	if row.Source < 0 || int(row.Source) >= len(prefixes) || row.ID == uuid.Nil || row.CreatedAt.IsZero() {
		return Item{}, ErrUnavailable
	}
	kind := row.Kind
	switch row.Source {
	case domain.AccountHistoryPayment:
		if kind != "subscription" && kind != "topup" {
			return Item{}, ErrUnavailable
		}
	case domain.AccountHistoryRefund, domain.AccountHistoryExternalRefund:
		if kind != "refund_money" {
			return Item{}, ErrUnavailable
		}
	case domain.AccountHistoryLedger:
		switch kind {
		case "generation", "generation_refund", "bonus", "expiry", "refund_adjustment", "refund_compensation", "referral_bonus", "referral_reversal", "adjustment":
		default:
			return Item{}, ErrUnavailable
		}
	}
	stars, err := domain.CurrentCreditAmount(row.StarsDelta, row.CreditDenominationVersion)
	if err != nil || stars > maxSafeInteger || stars < -maxSafeInteger {
		return Item{}, ErrUnavailable
	}
	status, ok := publicStatus(row.Status, row.Source)
	if !ok {
		return Item{}, ErrUnavailable
	}
	item := Item{ID: prefixes[row.Source] + row.ID.String(), Kind: kind, CreatedAt: row.CreatedAt, StarsDelta: stars, Status: status, TestMode: row.TestMode}
	if row.AmountMinor != nil {
		if *row.AmountMinor <= 0 || *row.AmountMinor > maxSafeInteger || row.Currency != domain.CurrencyRUB || row.Source == domain.AccountHistoryLedger {
			return Item{}, ErrUnavailable
		}
		item.Money = &Money{AmountMinor: *row.AmountMinor, Currency: domain.CurrencyRUB}
	}
	if row.Source != domain.AccountHistoryLedger && item.Money == nil {
		return Item{}, ErrUnavailable
	}
	if row.Kind == "subscription" {
		switch row.PlanCode {
		case "lite", "start", "pro", "ultima", "elite":
			item.PlanCode = row.PlanCode
		}
	}
	if domain.Modality(row.Modality).Valid() {
		item.Modality = row.Modality
	}
	if domain.OperationType(row.Operation).Valid() {
		item.Operation = row.Operation
	}
	return item, nil
}
func publicStatus(status string, source domain.AccountHistorySource) (string, bool) {
	if source == domain.AccountHistoryLedger {
		return "succeeded", status == "committed" || status == "succeeded"
	}
	switch status {
	case "created", "provider_pending", "waiting_for_user":
		return "pending", true
	case "succeeded":
		return "succeeded", true
	case "canceled", "expired":
		return "canceled", true
	case "failed":
		return "failed", true
	case "refunded":
		return "refunded", source == domain.AccountHistoryPayment
	case "partially_refunded":
		return "partially_refunded", source == domain.AccountHistoryPayment
	default:
		return "", false
	}
}
