package websession

import (
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMusicRoutesRequireSessionAndCSRF(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	for _, path := range []string{"/web/v1/music-jobs", "/web/v1/music-jobs/" + uuid.NewString(), "/web/v1/music-artifacts/" + uuid.NewString()} {
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 401 {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
	for _, path := range []string{"/web/v1/music-jobs/prepare", "/web/v1/music-jobs/" + uuid.NewString() + "/activate"} {
		req := safeConversationManagementRequest(t, "POST", path, sessions, uuid.New(), `{"model_id":"suno_v6","music":{"action":"generate","prompt":"Instrumental"}}`)
		req.Header.Del("X-CSRF-Token")
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

func TestMusicRejectsRawProviderFieldsBeforeUnavailable(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	for _, body := range []string{
		`{"model_id":"suno_v6","music":{"action":"generate","source_task_id":"foreign"}}`,
		`{"model_id":"suno_v6","music":{"action":"generate","audio_url":"https://private.test/audio.mp3"}}`,
		`{"model_id":"suno_v6","cost":1,"music":{"action":"generate"}}`,
	} {
		rec := httptest.NewRecorder()
		req := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/music-jobs/prepare", sessions, uuid.New(), body)
		req.Header.Set("X-Idempotency-Key", uuid.NewString())
		h.Routes().ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatalf("invalid body got %d", rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	req := safeConversationManagementRequest(t, http.MethodPost, "/web/v1/music-jobs/prepare", sessions, uuid.New(), `{"model_id":"suno_v6","music":{"action":"generate","prompt":"Instrumental"}}`)
	req.Header.Set("X-Idempotency-Key", uuid.NewString())
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != 503 {
		t.Fatalf("unavailable dependencies accepted: %d", rec.Code)
	}
}
