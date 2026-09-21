package websession

import (
	"github.com/google/uuid"
	"net/http/httptest"
	"testing"
)

func TestSpeechAuthCSRFAndStrictRequests(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	for _, path := range []string{"/web/v1/speech-jobs/" + uuid.NewString(), "/web/v1/speech-artifacts/" + uuid.NewString()} {
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("missing auth %d", w.Code)
		}
	}
	body := `{"model_id":"gpt_4o_mini_tts","speech":{"text":"synthetic","voice":"alloy","format":"wav","speed":1}}`
	for _, path := range []string{"/web/v1/speech-jobs/prepare", "/web/v1/speech-jobs/" + uuid.NewString() + "/activate"} {
		r := safeConversationManagementRequest(t, "POST", path, sessions, uuid.New(), body)
		r.Header.Del("X-CSRF-Token")
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("missing CSRF %d", w.Code)
		}
	}
	for _, tc := range []struct {
		body string
		code int
	}{
		{body, 503},
		{`{"model_id":"whisper_1","speech":{"format":"json","file_bytes":"cHJpdmF0ZQ=="}}`, 400},
		{`{"model_id":"gpt_4o_mini_tts","speech":{"text":"synthetic","voice":"alloy","format":"wav","speed":1},"price":1}`, 400},
	} {
		r := safeConversationManagementRequest(t, "POST", "/web/v1/speech-jobs/prepare", sessions, uuid.New(), tc.body)
		r.Header.Set("X-Idempotency-Key", uuid.NewString())
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("got%d want%d", w.Code, tc.code)
		}
	}
}
