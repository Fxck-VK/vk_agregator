package account

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
)

func TestBrowserPasswordReplacementRequiresCurrentPassword(t *testing.T) {
	for _, proof := range []string{"", "incorrect-password", "original-password"} {
		t.Run(proof, func(t *testing.T) {
			h, services := newTestHandler(t)
			ctx := context.Background()
			account, err := services.auth.ResolveVerifiedEmailPassword(ctx, "member@example.test")
			if err != nil {
				t.Fatal(err)
			}
			if err := services.auth.SetPasswordForVerifiedEmail(ctx, account.AccountID, account.AccountID, "member@example.test", "original-password"); err != nil {
				t.Fatal(err)
			}
			body := map[string]string{"email": "member@example.test", "password": "replacement-password"}
			if proof != "" {
				body["current_password"] = proof
			}
			payload, _ := json.Marshal(body)
			rec := httptest.NewRecorder()
			h.ServeBrowserAccountAction(rec, httptest.NewRequest("POST", "/web/v1/account/password/set", strings.NewReader(string(payload))), domain.RequestPrincipal{AccountID: account.AccountID, SessionID: uuid.New(), Method: domain.AuthenticationMethodAccountSession})
			if proof == "original-password" {
				if rec.Code != 204 {
					t.Fatalf("confirmed change status %d, want 204", rec.Code)
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "replacement-password"); err != nil {
					t.Fatal("new password not accepted")
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "original-password"); err == nil {
					t.Fatal("old password still accepted")
				}
			} else {
				if rec.Code < 400 {
					t.Fatal("unconfirmed password replacement succeeded")
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "original-password"); err != nil {
					t.Fatal("rejected change replaced original password")
				}
				if _, err := services.auth.AuthenticateEmailPassword(ctx, "member@example.test", "replacement-password"); err == nil {
					t.Fatal("rejected password accepted")
				}
			}
			if strings.Contains(rec.Body.String(), "original-password") || strings.Contains(rec.Body.String(), "replacement-password") {
				t.Fatal("response contains credentials")
			}
		})
	}
}
