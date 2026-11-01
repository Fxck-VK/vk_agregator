package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/adapter/inbound/admin"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestOperatorAccountPublicIDLookupProtectedAndPIIFree(t *testing.T) {
	repo := memory.NewAccountIdentityRepo()
	identity, err := repo.CreateAccountWithIdentity(context.Background(), domain.IdentityProviderEmail, "sensitive@example.test", "sensitive@example.test")
	if err != nil {
		t.Fatal(err)
	}
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{Accounts: repo}).Routes()
	request := func(query string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/admin/users/operator"+query, nil)
		if authenticated {
			req.Header.Set("X-Admin-Token", testAdminToken)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if got := request("?public_id=EP-000001", false); got.Code != http.StatusUnauthorized {
		t.Fatal("lookup not protected", got.Code)
	}
	got := request("?public_id=ep-000001", true)
	if got.Code != 200 {
		t.Fatalf("web-only account lookup: %d %s", got.Code, got.Body.String())
	}
	var payload struct {
		Account struct {
			PublicID   string `json:"public_id"`
			AccountRef string `json:"account_ref"`
		}
		User any
	}
	if err = json.Unmarshal(got.Body.Bytes(), &payload); err != nil || payload.Account.PublicID != "EP-000001" || payload.Account.AccountRef == "" || payload.User != nil {
		t.Fatal("bad account summary", err)
	}
	for _, secret := range []string{identity.AccountID.String(), identity.ID.String(), "sensitive@example.test"} {
		if strings.Contains(got.Body.String(), secret) {
			t.Fatal("lookup leaks account internals")
		}
	}
	for _, tc := range []struct {
		query  string
		status int
	}{{"?public_id=EP-1", 400}, {"?public_id=EP-000002", 404}, {"?public_id=EP-000001&user_id=" + uuid.NewString(), 400}, {"?public_id=", 400}, {"?public_id=EP-000001&public_id=EP-000002", 400}} {
		if got := request(tc.query, true); got.Code != tc.status {
			t.Fatalf("%s: %d want %d", tc.query, got.Code, tc.status)
		}
	}
}

type brokenPublicIDRepo struct{}

func (brokenPublicIDRepo) GetAccountPublicNumber(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("synthetic private database detail")
}
func (brokenPublicIDRepo) GetAccountByPublicNumber(context.Context, int64) (*domain.Account, error) {
	return nil, errors.New("synthetic private database detail")
}

func TestOperatorAccountPublicIDStorageFailureIsSanitized(t *testing.T) {
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{Accounts: brokenPublicIDRepo{}}).Routes()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/operator?public_id=EP-000001", nil)
	req.Header.Set("X-Admin-Token", testAdminToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "private database") {
		t.Fatal("storage failure leaked or returned success", rec.Code)
	}
}

func TestOperatorAccountPublicIDMissingRepositoryFailsClosed(t *testing.T) {
	h := admin.NewHandler(admin.Config{Token: testAdminToken}, admin.Deps{}).Routes()
	req := httptest.NewRequest(http.MethodGet, "/admin/users/operator?public_id=EP-000001", nil)
	req.Header.Set("X-Admin-Token", testAdminToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 503 {
		t.Fatal("unconfigured lookup must fail closed", rec.Code)
	}
}
