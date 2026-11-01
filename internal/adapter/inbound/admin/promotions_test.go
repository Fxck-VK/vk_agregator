package admin_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vk-ai-aggregator/internal/adapter/inbound/admin"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/promotionservice"
)

func TestAdminPromotionPermissionsIdempotencyAndSafeValidation(t *testing.T) {
	repo := memory.NewPaymentRepo()
	credits := memory.NewBillingRepo()
	tx := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return fn(ctx, repo, credits)
	})
	svc := promotionservice.New(tx, repo)
	request := func(role domain.OperatorRole, token, key, body string) *httptest.ResponseRecorder {
		h := admin.NewHandler(admin.Config{Token: testAdminToken, DefaultRole: role}, admin.Deps{Promotions: svc}).Routes()
		req := httptest.NewRequest(http.MethodPost, "/admin/promotions", strings.NewReader(body))
		req.Header.Set("X-Admin-Token", token)
		req.Header.Set("X-Idempotency-Key", key)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	body := `{"code":"FIRST15","percent":15,"enabled":true,"starts_at":"2026-10-01T00:00:00Z","max_uses":10,"plan_codes":["start"]}`
	key := uuid.NewString()
	if r := request(domain.OperatorRoleOwner, "", key, body); r.Code != 401 {
		t.Fatal(r.Code)
	}
	if r := request(domain.OperatorRoleSupport, testAdminToken, key, body); r.Code != 403 {
		t.Fatal(r.Code)
	}
	if r := request(domain.OperatorRoleOwner, testAdminToken, "", body); r.Code != 400 {
		t.Fatal(r.Code)
	}
	r := request(domain.OperatorRoleOwner, testAdminToken, key, body)
	if r.Code != 200 && r.Code != 201 {
		t.Fatalf("create %d %s", r.Code, r.Body.String())
	}
	var p domain.Promotion
	if e := json.Unmarshal(r.Body.Bytes(), &p); e != nil || p.ID == uuid.Nil || p.Revision != 1 {
		t.Fatal(e, p)
	}
	r2 := request(domain.OperatorRoleOwner, testAdminToken, key, body)
	if r2.Body.String() != r.Body.String() {
		t.Fatal("replay differs")
	}
	if r := request(domain.OperatorRoleOwner, testAdminToken, key, strings.Replace(body, `"percent":15`, `"percent":30`, 1)); r.Code != 409 {
		t.Fatal("payload replay", r.Code)
	}
	if r := request(domain.OperatorRoleOwner, testAdminToken, uuid.NewString(), strings.Replace(body, `"percent":15`, `"percent":100`, 1)); r.Code != 400 {
		t.Fatal("invalid accepted", r.Code)
	}
}
