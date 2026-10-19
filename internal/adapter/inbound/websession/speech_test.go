package websession

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http/httptest"
	"testing"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/speechgeneration"
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

func TestWhisperDEVPreparationReplayAndActivation(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	h, jobs, sessions := newImageJobTestHandler(t)
	prices, _ := pricingcatalog.NewStaticCatalog()
	if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
		t.Fatal(err)
	}
	h.deps.ImagePricing = prices
	owner, key := uuid.New(), uuid.NewString()
	a := &domain.Artifact{ID: uuid.New(), OwnerAccountID: owner, Kind: domain.ArtifactKindInput, MediaType: domain.MediaTypeAudio, Status: domain.ArtifactStatusReady, StorageBucket: "private", StorageKey: "synthetic.wav", MimeType: "audio/wav", DurationMS: 60001, SizeBytes: 100, Codec: "pcm_s16le", Container: "wav", ProbeStatus: domain.MediaProbePassed}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: a}
	r := speechgeneration.Request{ModelID: "whisper_1", AudioArtifactID: a.ID, Speech: domain.SpeechRequest{Format: "json", Language: "ru"}}
	p, q, err := speechgeneration.Resolve(r, providermodels.RuntimeRegistry(), prices, a.DurationMS)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	stored := &domain.Job{ID: uuid.New(), AccountID: owner, Source: "web", OperationType: domain.OperationAudioSTT, Modality: domain.ModalityText, IdempotencyKey: key, Status: domain.JobStatusPrepared, Params: raw, PricingSnapshot: mustMarshalPricingSnapshot(q), CostEstimate: 50, ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, InputArtifactIDs: []uuid.UUID{a.ID}}
	jobs.prepared = stored
	body := fmt.Sprintf(`{"model_id":"whisper_1","audio_artifact_id":%q,"speech":{"format":"json","language":"ru"}}`, a.ID.String())
	send := func(path, body string, account uuid.UUID) *httptest.ResponseRecorder {
		t.Helper()
		req := safeConversationManagementRequest(t, "POST", path, sessions, account, body)
		req.Header.Set("X-Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, req)
		return w
	}
	path := "/web/v1/speech-jobs/prepare"
	if w := send(path, body, owner); w.Code != 201 || jobs.prepareCalls != 1 || jobs.prepareInput.CostEstimateCredits != 50 || jobs.prepareInput.PricingSnapshot.Key.DurationSec != 120 {
		t.Fatalf("prepare %d, calls=%d, cost=%d", w.Code, jobs.prepareCalls, jobs.prepareInput.CostEstimateCredits)
	}
	h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{job: stored}
	h.deps.ImagePricing = nil
	if w := send(path, body, owner); w.Code != 201 || jobs.prepareCalls != 1 {
		t.Fatal("replay changed the prepared job or its price")
	}
	changed := fmt.Sprintf(`{"model_id":"whisper_1","audio_artifact_id":%q,"speech":{"format":"json","language":"en"}}`, a.ID.String())
	if w := send(path, changed, owner); w.Code != 409 {
		t.Fatal("changed replay accepted")
	}
	h.deps.ImageJobReader = &imageJobReaderStub{job: stored}
	jobs.activateErr = domain.ErrInsufficientCredits
	activate := "/web/v1/speech-jobs/" + stored.ID.String() + "/activate"
	if w := send(activate, "", owner); w.Code != 402 || jobs.activateCalls != 1 {
		t.Fatal("activation bypassed billing")
	}
	a.DurationMS++
	if w := send(activate, "", owner); w.Code != 400 || jobs.activateCalls != 1 {
		t.Fatal("changed input duration reached activation")
	}
	a.DurationMS--
	if w := send(activate, "", uuid.New()); w.Code != 404 || jobs.activateCalls != 1 {
		t.Fatal("foreign account reached activation")
	}
	for _, field := range []string{`"input_duration_ms":1`, `"duration_ms":1`, `"cost_estimate":1`} {
		bad := body[:len(body)-1] + "," + field + "}"
		if w := send(path, bad, owner); w.Code != 400 {
			t.Fatalf("client billing field accepted: %s", field)
		}
	}
	h.deps.ImageJobIdempotency = &imageJobIdempotencyReaderStub{}
	for _, change := range []func(*domain.Artifact){func(a *domain.Artifact) { a.OwnerAccountID = uuid.New() }, func(a *domain.Artifact) { a.SizeBytes = 25_000_001 }, func(a *domain.Artifact) { a.DurationMS = 480001 }, func(a *domain.Artifact) { a.ProbeStatus = "pending" }} {
		bad := *a
		change(&bad)
		h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: &bad}
		if w := send(path, body, owner); w.Code != 400 || jobs.prepareCalls != 1 {
			t.Fatal("untrusted input reached preparation")
		}
	}
}
