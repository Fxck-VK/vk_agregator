package websession

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

type deletionStub struct {
	owner, id uuid.UUID
	calls     int
	err       error
}

func (s *deletionStub) Delete(_ context.Context, owner, id uuid.UUID) error {
	s.owner, s.id = owner, id
	s.calls++
	return s.err
}
func TestArtifactDeletionRequiresOwnerSessionAndCSRF(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	stub := &deletionStub{}
	h.deps.ArtifactDeletion = stub
	owner, id := uuid.New(), uuid.New()
	path := "/web/v1/artifacts/" + id.String()
	for _, test := range []struct {
		name   string
		csrf   bool
		err    error
		status int
	}{{"missing csrf", false, nil, 403}, {"accepted", true, nil, 202}, {"repeat", true, nil, 202}, {"foreign", true, domain.ErrNotFound, 404}, {"busy", true, domain.ErrConflict, 409}} {
		t.Run(test.name, func(t *testing.T) {
			stub.err = test.err
			before := stub.calls
			req := authenticatedConversationRequest(t, http.MethodDelete, path, sessions, owner)
			if test.csrf {
				req.Header.Set("Origin", "https://app.example.test")
				req.Header.Set("X-CSRF-Token", "csrf")
				req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
			}
			rec := httptest.NewRecorder()
			h.Routes().ServeHTTP(rec, req)
			if rec.Code != test.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !test.csrf && stub.calls != before {
				t.Fatal("unsafe request reached deletion")
			}
			if test.csrf && (stub.owner != owner || stub.id != id) {
				t.Fatal("owner scope lost")
			}
		})
	}
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Origin", "https://app.example.test")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatal("anonymous delete allowed", rec.Code)
	}
}
