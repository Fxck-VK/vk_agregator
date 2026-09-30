package websession

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/service/accountauth"
)

type requestStartedKey struct{}

func observeWebRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.Header.Get("X-Request-ID"))
		if err != nil {
			id = uuid.New()
		}
		w.Header().Set("X-Request-ID", id.String())
		ctx := context.WithValue(r.Context(), requestStartedKey{}, time.Now())
		// Bound small metadata reads and refresh, not media streams or user writes.
		if r.URL.Path == "/web/v1/conversations" && r.Method == http.MethodGet || r.URL.Path == "/web/v1/me" || r.URL.Path == "/web/v1/balance" || r.URL.Path == "/web/v1/auth/refresh" {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func reportReadFailure(w http.ResponseWriter, r *http.Request, stage string, status int) {
	started, _ := r.Context().Value(requestStartedKey{}).(time.Time)
	slog.WarnContext(r.Context(), "web_request_failed", "request_id", w.Header().Get("X-Request-ID"), "stage", stage, "status", status, "duration_ms", time.Since(started).Milliseconds())
}

func sessionFailure(w http.ResponseWriter, r *http.Request, err error, stage string) {
	if errors.Is(err, accountauth.ErrInvalidSession) || errors.Is(err, accountauth.ErrSessionExpired) {
		reportReadFailure(w, r, stage, http.StatusUnauthorized)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	reportReadFailure(w, r, stage, http.StatusServiceUnavailable)
	writeError(w, http.StatusServiceUnavailable, "authentication unavailable")
}
