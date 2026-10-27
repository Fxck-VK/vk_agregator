package account

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDisabledPasswordOnlyLoginCannotBypassBrowserProof(t *testing.T) {
	h := NewHandler(Deps{DisablePasswordOnlyLogin: true}).Routes()
	r := httptest.NewRequest(http.MethodPost, "/account/password/login", strings.NewReader(`{"email":"synthetic@example.test","password":"synthetic-password"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("retired password login status = %d, want 404", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/account/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("other account routes were removed: /account/me status = %d", w.Code)
	}
}
