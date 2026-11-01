package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/accounthistory"
)

func TestAccountHistoryPostgresRejectsInvalidBounds(t *testing.T) {
	r := postgres.NewAccountHistoryRepository(nil)
	for _, n := range []int{-1, 0, 52, 1000} {
		if _, err := r.ListAccountHistory(context.Background(), uuid.New(), n, nil); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("limit %d: %v", n, err)
		}
	}
	if _, err := r.ListAccountHistory(context.Background(), uuid.Nil, 10, nil); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("nil owner: %v", err)
	}
	for _, cursor := range []domain.AccountHistoryCursor{
		{},
		{CreatedAt: time.Now(), ID: uuid.New(), Source: -1},
		{CreatedAt: time.Now(), ID: uuid.New(), Source: 4},
	} {
		if _, err := r.ListAccountHistory(context.Background(), uuid.New(), 10, &cursor); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("invalid cursor: %v", err)
		}
	}
}

func TestAccountHistoryPostgresServiceRoundTrip(t *testing.T) {
	f := newHistoryFixture(t)
	ctx := context.Background()
	paid := f.payment(t, f.owner, "succeeded", "yookassa", `{"subscription":{"plan":{"code":"pro","name":"private-name"}},"private":"must-not-leak"}`)
	pending := f.payment(t, f.owner, "waiting_for_user", "mock", `{}`)
	f.ledger(t, f.owner, f.wallet, "topup", "committed", 100, "topup:yookassa:"+paid.String(), "private-reason", "", 1, nil)
	f.ledger(t, f.owner, f.wallet, "capture", "committed", -7, "", "private-job-reason", "", 1, nil)
	service := accounthistory.New(postgres.NewAccountHistoryRepository(f.p))
	var cursor string
	seen := map[string]accounthistory.Item{}
	for i := 0; i < 5; i++ {
		page, err := service.List(ctx, f.owner, 1, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if _, ok := seen[item.ID]; ok {
				t.Fatal("duplicate page row", item.ID)
			}
			seen[item.ID] = item
		}
		data, _ := json.Marshal(page)
		for _, forbidden := range []string{"private-name", "private-reason", "must-not-leak", "receipt_email", "idempotency_key", "provider_payment_id", "account_id", "owner_account_id"} {
			if strings.Contains(string(data), forbidden) {
				t.Fatalf("private field leaked: %s", forbidden)
			}
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
		if _, err := service.List(ctx, f.foreign, 1, cursor); !errors.Is(err, accounthistory.ErrInvalidQuery) {
			t.Fatalf("cross-owner cursor %v", err)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("grant duplicated or history missing: %+v", seen)
	}
	purchase := seen["payment:"+paid.String()]
	if purchase.StarsDelta != 200 || purchase.Money == nil || purchase.Money.AmountMinor != 12345 || purchase.PlanCode != "pro" {
		t.Fatalf("historical purchase changed: %+v", purchase)
	}
	if item := seen["payment:"+pending.String()]; item.StarsDelta != 0 || item.Status != "pending" {
		t.Fatalf("unpaid stars invented: %+v", item)
	}
}

type historyFixture struct {
	p                                     *pgxpool.Pool
	owner, foreign, wallet, foreignWallet uuid.UUID
	at                                    time.Time
}

func newHistoryFixture(t *testing.T) historyFixture {
	t.Helper()
	f := historyFixture{p: rewardPool(t), owner: uuid.New(), foreign: uuid.New(), wallet: uuid.New(), foreignWallet: uuid.New(), at: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}
	f.exec(t, `INSERT INTO accounts(id) VALUES($1),($2)`, f.owner, f.foreign)
	f.exec(t, `INSERT INTO credit_accounts(id,owner_account_id,currency) VALUES($1,$2,'credits'),($3,$4,'credits')`, f.wallet, f.owner, f.foreignWallet, f.foreign)
	return f
}

func (f historyFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.p.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (f historyFixture) payment(t *testing.T, owner uuid.UUID, status, provider, metadata string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	f.exec(t, `INSERT INTO payment_intents(id,account_id,status,amount,currency,credits,price_version,provider,provider_payment_id,idempotency_key,receipt_email,metadata,created_at) VALUES($1,$2,$3,12345,'rub',999,1,$4,$5,$5,'history@example.invalid',$6,$7)`, id, owner, status, provider, id.String(), metadata, f.at)
	return id
}

func (f historyFixture) ledger(t *testing.T, owner, wallet uuid.UUID, typ, status string, amount int64, key, reason, lot string, version int, job *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if key == "" {
		key = id.String()
	}
	f.exec(t, `INSERT INTO ledger_entries(id,account_id,owner_account_id,type,status,amount,idempotency_key,reason,lot_kind,credit_denomination_version,job_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, wallet, owner, typ, status, amount, key, reason, lot, version, job, f.at)
	return id
}

func (f historyFixture) list(t *testing.T, limit int, after *domain.AccountHistoryCursor) []domain.AccountHistoryRow {
	t.Helper()
	rows, err := postgres.NewAccountHistoryRepository(f.p).ListAccountHistory(context.Background(), f.owner, limit, after)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func historyRowsByID(rows []domain.AccountHistoryRow) map[uuid.UUID]domain.AccountHistoryRow {
	out := make(map[uuid.UUID]domain.AccountHistoryRow, len(rows))
	for _, row := range rows {
		out[row.ID] = row
	}
	return out
}

func TestAccountHistoryPostgresPurchasesAndOwnerJoins(t *testing.T) {
	f := newHistoryFixture(t)
	paid := f.payment(t, f.owner, "succeeded", "yookassa", `{"subscription":{"plan":{"code":"pro"}},"private":"must-not-project"}`)
	pending := f.payment(t, f.owner, "waiting_for_user", "mock", `{}`)
	foreign := f.payment(t, f.foreign, "succeeded", "yookassa", `{}`)
	unknown := f.payment(t, f.owner, "failed", "yookassa", `{"subscription":{"plan":{"code":"private-plan"}}}`)
	grant := f.ledger(t, f.owner, f.wallet, "topup", "committed", 73, "topup:yookassa:"+paid.String(), "private reason", "subscription", 1, nil)
	foreignKey := f.ledger(t, f.owner, f.wallet, "topup", "committed", 9, "topup:yookassa:"+foreign.String(), "", "", 2, nil)
	f.ledger(t, f.foreign, f.foreignWallet, "adjustment", "committed", 99, "", "", "", 2, nil)
	// Corrupted provenance must not import another wallet's movements/grants.
	f.ledger(t, f.owner, f.foreignWallet, "adjustment", "committed", 101, "", "", "", 2, nil)
	f.exec(t, `INSERT INTO web_referral_payment_evidence(intent_id,account_id,provider,provider_payment_id,amount,currency,mode,verified_at) VALUES($1,$2,'yookassa',$3,12345,'rub','live',$4)`, paid, f.owner, paid.String(), f.at)
	rows := historyRowsByID(f.list(t, 51, nil))
	if len(rows) != 4 {
		t.Fatalf("rows=%+v", rows)
	}
	if _, ok := rows[grant]; ok {
		t.Fatal("duplicate payment grant")
	}
	if _, ok := rows[foreignKey]; !ok {
		t.Fatal("foreign intent hid owner's grant")
	}
	got := rows[paid]
	if got.Kind != "subscription" || got.PlanCode != "pro" || got.StarsDelta != 73 || got.CreditDenominationVersion != 1 || got.AmountMinor == nil || *got.AmountMinor != 12345 || got.Currency != "rub" || got.TestMode == nil || *got.TestMode {
		t.Fatalf("purchase=%+v", got)
	}
	if got := rows[pending]; got.StarsDelta != 0 || got.CreditDenominationVersion != 2 || got.TestMode == nil || !*got.TestMode || got.Kind != "topup" {
		t.Fatalf("pending=%+v", got)
	}
	if got := rows[unknown]; got.PlanCode != "" || got.TestMode != nil {
		t.Fatalf("unknown=%+v", got)
	}
	// Mutable payment status must not change its cursor position/date.
	f.exec(t, `UPDATE payment_intents SET status='refunded',updated_at=now() WHERE id=$1`, paid)
	if got := historyRowsByID(f.list(t, 51, nil))[paid]; got.Status != "refunded" || !got.CreatedAt.Equal(f.at) {
		t.Fatalf("updated=%+v", got)
	}
	tx, err := f.p.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `SET TRANSACTION READ ONLY`); err != nil {
		t.Fatal(err)
	}
	if _, err := postgres.NewAccountHistoryRepository(tx).ListAccountHistory(context.Background(), f.owner, 51, nil); err != nil {
		t.Fatalf("history must work in a read-only transaction: %v", err)
	}
}

func TestAccountHistoryPostgresCommittedMovementsAndSafeLabels(t *testing.T) {
	f := newHistoryFixture(t)
	job, foreignJob, deletedJob, unknownJob := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, j := range []struct {
		id, owner           uuid.UUID
		operation, modality string
	}{{job, f.owner, "text_generate", "text"}, {foreignJob, f.foreign, "image_edit", "image"}, {deletedJob, f.owner, "audio_tts", "audio"}, {unknownJob, f.owner, "private-operation", "private-modality"}} {
		f.exec(t, `INSERT INTO jobs(id,account_id,operation_type,modality,idempotency_key,channel,recipient_ref,params) VALUES($1,$2,$3,$4,$5,'web',$6,'{"prompt":"private"}')`, j.id, j.owner, j.operation, j.modality, j.id.String(), j.owner.String())
	}
	capture := f.ledger(t, f.owner, f.wallet, "capture", "committed", -11, "", "", "", 1, &job)
	foreignLabel := f.ledger(t, f.owner, f.wallet, "capture", "committed", -3, "", "", "", 2, &foreignJob)
	deleted := f.ledger(t, f.owner, f.wallet, "refund", "committed", 2, "", "", "", 2, &deletedJob)
	unknown := f.ledger(t, f.owner, f.wallet, "capture", "committed", -2, "", "", "", 2, &unknownJob)
	f.exec(t, `DELETE FROM jobs WHERE id=$1`, deletedJob)
	cases := []struct {
		typ, reason, lot, key, kind string
		amount                      int64
	}{
		{"adjustment", "credit lot expiry", "", "", "expiry", -5},
		{"topup", "", "bonus", "", "bonus", 50},
		{"adjustment", "private reason", "", "payment_refund_debit:fixture", "refund_adjustment", -9},
		{"adjustment", "private reason", "", "payment_refund_compensate:fixture", "refund_compensation", 9},
		{"adjustment", "private reason", "", "paymentXrefundXdebit:fixture", "adjustment", -1},
		{"adjustment", "private reason", "", "", "adjustment", 7},
	}
	want := map[uuid.UUID]string{capture: "generation", foreignLabel: "generation", deleted: "generation_refund", unknown: "generation"}
	for _, c := range cases {
		want[f.ledger(t, f.owner, f.wallet, c.typ, "committed", c.amount, c.key, c.reason, c.lot, 2, nil)] = c.kind
	}
	for _, c := range []struct {
		typ, status string
		amount      int64
	}{{"reserve", "committed", -40}, {"release", "committed", 40}, {"capture", "pending", -10}, {"adjustment", "committed", 0}} {
		f.ledger(t, f.owner, f.wallet, c.typ, c.status, c.amount, "", "", "", 2, nil)
	}
	rows := historyRowsByID(f.list(t, 51, nil))
	if len(rows) != len(want) {
		t.Fatalf("count=%d want=%d rows=%+v", len(rows), len(want), rows)
	}
	for id, kind := range want {
		if rows[id].Kind != kind {
			t.Errorf("kind=%q want=%q", rows[id].Kind, kind)
		}
	}
	if got := rows[capture]; got.Modality != "text" || got.Operation != "text_generate" || got.StarsDelta != -11 || got.CreditDenominationVersion != 1 {
		t.Fatalf("capture=%+v", got)
	}
	for _, id := range []uuid.UUID{foreignLabel, deleted, unknown} {
		if got := rows[id]; got.Modality != "" || got.Operation != "" {
			t.Fatalf("unsafe label=%+v", got)
		}
	}
}

func TestAccountHistoryPostgresAuthenticatedPaymentModes(t *testing.T) {
	f := newHistoryFixture(t)
	want := map[uuid.UUID]*bool{}
	yes, no := true, false
	for _, c := range []struct {
		mode          string
		evidenceOwner uuid.UUID
		amount        int64
		expected      *bool
	}{
		{"test", f.owner, 12345, &yes},
		{"live", f.owner, 12345, &no},
		{"unknown", f.owner, 12345, nil},
		{"historical", f.owner, 12345, nil},
		{"live", f.foreign, 12345, nil},
		{"test", f.owner, 999, nil},
	} {
		id := f.payment(t, f.owner, "succeeded", "yookassa", `{"test":true}`)
		f.exec(t, `INSERT INTO web_referral_payment_evidence(intent_id,account_id,provider,provider_payment_id,amount,currency,mode,verified_at) VALUES($1,$2,'yookassa',$3,$4,'rub',$5,$6)`, id, c.evidenceOwner, id.String(), c.amount, c.mode, f.at)
		want[id] = c.expected
	}
	rows := f.list(t, 51, nil)
	if len(rows) != len(want) {
		t.Fatalf("mode rows=%d want=%d", len(rows), len(want))
	}
	for _, row := range rows {
		expected := want[row.ID]
		if (row.TestMode == nil) != (expected == nil) || (expected != nil && *row.TestMode != *expected) {
			t.Fatalf("mode=%v want=%v", row.TestMode, expected)
		}
	}
}

func TestAccountHistoryPostgresRefundsDeduplicateMoneyOnly(t *testing.T) {
	f := newHistoryFixture(t)
	intent := f.payment(t, f.owner, "partially_refunded", "yookassa", `{}`)
	foreign := f.payment(t, f.foreign, "refunded", "yookassa", `{}`)
	manual, foreignManual := uuid.New(), uuid.New()
	f.exec(t, `INSERT INTO payment_refunds(id,intent_id,provider_refund_id,amount,status,idempotency_key,reason,created_at) VALUES($1,$2,'shared-proof',300,'provider_pending',$3,'private',$4),($5,$6,'foreign-proof',800,'succeeded',$7,'private',$4)`, manual, intent, manual.String(), f.at, foreignManual, foreign, foreignManual.String())
	f.exec(t, `INSERT INTO web_referral_refund_evidence(provider,provider_refund_id,intent_id,amount,verified_at) VALUES('yookassa','shared-proof',$1,250,$3),('yookassa','external-only',$1,200,$3),('yookassa','foreign-proof',$2,800,$3)`, intent, foreign, f.at)
	correction := f.ledger(t, f.owner, f.wallet, "adjustment", "committed", -4, "payment_refund_debit:owned", "", "", 2, nil)
	rows := historyRowsByID(f.list(t, 51, nil))
	if len(rows) != 4 {
		t.Fatalf("rows=%+v", rows)
	}
	if got := rows[manual]; got.Source != domain.AccountHistoryRefund || got.Kind != "refund_money" || got.StarsDelta != 0 || got.AmountMinor == nil || *got.AmountMinor != 250 || got.Status != "succeeded" || !got.CreatedAt.Equal(f.at) {
		t.Fatalf("manual=%+v", got)
	}
	f.exec(t, `UPDATE payment_refunds SET status='failed' WHERE id=$1`, manual)
	if got := historyRowsByID(f.list(t, 51, nil))[manual]; got.Status != "succeeded" || got.AmountMinor == nil || *got.AmountMinor != 250 {
		t.Fatalf("authenticated proof lost to stale failure: %+v", got)
	}
	if rows[correction].Kind != "refund_adjustment" {
		t.Fatal("money refund hid star correction")
	}
	var externalID uuid.UUID
	if err := f.p.QueryRow(context.Background(), `SELECT md5('web-history-refund:yookassa:external-only')::uuid`).Scan(&externalID); err != nil {
		t.Fatal(err)
	}
	if got := rows[externalID]; got.Source != domain.AccountHistoryExternalRefund || got.AmountMinor == nil || *got.AmountMinor != 200 || got.Status != "succeeded" {
		t.Fatalf("external=%+v", got)
	}
}

func TestAccountHistoryPostgresKeysetTiesAndNewInsert(t *testing.T) {
	f := newHistoryFixture(t)
	for i := 0; i < 7; i++ {
		f.ledger(t, f.owner, f.wallet, "adjustment", "committed", int64(i+1), "", "", "", 2, nil)
	}
	intent := f.payment(t, f.owner, "created", "yookassa", `{}`)
	refund := uuid.New()
	f.exec(t, `INSERT INTO payment_refunds(id,intent_id,amount,status,idempotency_key,created_at) VALUES($1,$2,100,'created',$3,$4)`, refund, intent, refund.String(), f.at)
	f.exec(t, `INSERT INTO web_referral_refund_evidence(provider,provider_refund_id,intent_id,amount,verified_at) VALUES('yookassa','keyset-proof',$1,200,$2)`, intent, f.at)
	all := f.list(t, 51, nil)
	first := f.list(t, 3, nil)
	if len(first) != 3 || first[0].Source != domain.AccountHistoryExternalRefund || first[1].Source != domain.AccountHistoryRefund || first[2].Source != domain.AccountHistoryPayment {
		t.Fatalf("tie ordering=%+v", first)
	}
	for i := 0; i < 2; i++ {
		last := first[i]
		page := f.list(t, 1, &domain.AccountHistoryCursor{CreatedAt: last.CreatedAt, Source: last.Source, ID: last.ID})
		if len(page) != 1 || page[0].ID != first[i+1].ID || page[0].Source != first[i+1].Source {
			t.Fatalf("source boundary %d: %+v", i, page)
		}
	}
	inserted := f.ledger(t, f.owner, f.wallet, "adjustment", "committed", 8, "", "", "", 2, nil)
	f.exec(t, `UPDATE ledger_entries SET created_at=$2 WHERE id=$1`, inserted, f.at.Add(time.Hour))
	got := append([]domain.AccountHistoryRow{}, first...)
	for len(got) < len(all) {
		last := got[len(got)-1]
		page := f.list(t, 3, &domain.AccountHistoryCursor{CreatedAt: last.CreatedAt, Source: last.Source, ID: last.ID})
		if len(page) == 0 {
			t.Fatal("premature page end")
		}
		got = append(got, page...)
	}
	if len(got) != len(all) {
		t.Fatalf("count=%d want=%d", len(got), len(all))
	}
	for i := range all {
		if got[i].ID != all[i].ID || got[i].Source != all[i].Source {
			t.Fatalf("page %d differs", i)
		}
	}
	last := got[len(got)-1]
	if tail := f.list(t, 3, &domain.AccountHistoryCursor{CreatedAt: last.CreatedAt, Source: last.Source, ID: last.ID}); len(tail) != 0 {
		t.Fatalf("tail=%+v", tail)
	}
}

func TestAccountHistoryPostgresReferralClassification(t *testing.T) {
	p := rewardPool(t)
	f := newRewardFixture(t, p, domain.WebReferralPaymentLive)
	f.process(t, f.at.Add(24*time.Hour))
	f.refund(t)
	f.process(t, f.at.Add(49*time.Hour))
	rows, err := postgres.NewAccountHistoryRepository(p).ListAccountHistory(context.Background(), f.owner, 51, nil)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, row := range rows {
		kinds[row.Kind]++
	}
	if kinds["referral_bonus"] != 1 || kinds["referral_reversal"] != 1 || len(rows) != 2 {
		t.Fatalf("rows=%+v", rows)
	}
}
