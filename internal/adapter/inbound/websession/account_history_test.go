package websession

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/accounthistory"
)

type accountHistoryStub struct {
	owner  uuid.UUID
	limit  int
	cursor string
	calls  int
	err    error
}

func (s *accountHistoryStub) List(_ context.Context, owner uuid.UUID, limit int, cursor string) (accounthistory.Page, error) {
	s.owner = owner
	s.limit = limit
	s.cursor = cursor
	s.calls++
	return accounthistory.Page{Items: []accounthistory.Item{}}, s.err
}
func TestWebAccountHistoryRequiresCookieAndDerivesOwner(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	stub := &accountHistoryStub{}
	h.deps.AccountHistory = stub
	owner := uuid.New()
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest("GET", "/web/v1/billing/history", nil))
	if rec.Code != 401 || stub.calls != 0 {
		t.Fatalf("unauth read %d", rec.Code)
	}
	req := authenticatedConversationRequest(t, "GET", "/web/v1/billing/history?limit=15&cursor=position", sessions, owner)
	req.Header.Set("X-NeiroHub-Account-ID", uuid.NewString())
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 200 || stub.owner != owner || stub.limit != 15 || stub.cursor != "position" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("history read %d %s %+v", rec.Code, rec.Body, stub)
	}
	if rec.Body.String() != "{\"items\":[],\"next_cursor\":null}\n" {
		t.Fatalf("unsafe or inconsistent response: %s", rec.Body)
	}
}
func TestWebAccountHistoryRejectsQueryAndReportsSafeFailures(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	stub := &accountHistoryStub{}
	h.deps.AccountHistory = stub
	owner := uuid.New()
	for _, query := range []string{"?account_id=" + uuid.NewString(), "?limit=0", "?limit=-1", "?limit=51", "?limit=10000000", "?limit=x", "?limit=", "?limit=1&limit=2", "?cursor=a&cursor=b", "?cursor=" + strings.Repeat("x", 513)} {
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/billing/history"+query, sessions, owner))
		if rec.Code != 400 || stub.calls != 0 {
			t.Fatalf("bad query %s = %d", query, rec.Code)
		}
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{accounthistory.ErrInvalidQuery, 400}, {errors.New("secret internal storage reason"), 503}} {
		stub.err = tc.err
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, "GET", "/web/v1/billing/history", sessions, owner))
		if rec.Code != tc.status || strings.Contains(rec.Body.String(), "secret") || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("unsafe failure %d %s", rec.Code, rec.Body)
		}
	}
	h.deps.AccountHistory = nil
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, "GET", "/web/v1/billing/history", sessions, owner))
	if rec.Code != 503 {
		t.Fatal(rec.Code)
	}
}
