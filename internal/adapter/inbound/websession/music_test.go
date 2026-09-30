package websession

import (
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestDEVSmokeMusicActivationRetainsOwnershipAndBilling(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	TestMusicRoutesRequireSessionAndCSRF(t)
	h, jobs, sessions := newImageJobTestHandler(t)
	accountID, jobID := uuid.New(), uuid.New()
	params, quote, err := musicgeneration.Resolve(musicgeneration.Request{ModelID: "suno_v6", Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "Instrumental"}}, providermodels.RuntimeRegistry())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(params)
	price, _ := json.Marshal(quote)
	job := &domain.Job{ID: jobID, AccountID: accountID, Source: "web", OperationType: domain.OperationAudioMusic, Modality: domain.ModalityAudio, Status: domain.JobStatusPrepared, CostEstimate: quote.InternalCredits, Params: raw, PricingSnapshot: price, ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}}
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	jobs.activateJob = job
	jobs.activateErr = domain.ErrInsufficientCredits
	req := safeImageMutationRequest(t, sessions, accountID, http.MethodPost, "/web/v1/music-jobs/"+jobID.String()+"/activate", nil)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusPaymentRequired || jobs.activateCalls != 1 {
		t.Fatalf("smoke did not reach billing activation: %d calls=%d", rec.Code, jobs.activateCalls)
	}
	req = safeImageMutationRequest(t, sessions, uuid.New(), http.MethodPost, "/web/v1/music-jobs/"+jobID.String()+"/activate", nil)
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || jobs.activateCalls != 1 {
		t.Fatal("foreign account reached activation")
	}
}

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
