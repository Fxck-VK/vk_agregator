package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/speechgeneration"
)

func TestReleaseCatalogProductionHidesCandidatesWithoutMutatingDEV(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	h, _, sessions := newImageJobTestHandler(t)
	for _, tc := range []struct {
		environment, requested string
		wantCandidates         bool
	}{
		{"development", "dev", true}, {"development", "prod", false}, {"production", "dev", false}, {"", "dev", false}, {"development", "dev", true},
	} {
		h.cfg.Environment = tc.environment
		req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/models", sessions, uuid.New())
		req.Header.Set("X-Epifera-Release-View", tc.requested)
		req.AddCookie(&http.Cookie{Name: "epifera_release_view", Value: "dev"})
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, req)
		var result productcatalog.WorkspaceModelList
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatalf("catalog status=%d", w.Code)
		}
		candidates := false
		for _, model := range result.Items {
			candidates = candidates || model.Verification == "dev-smoke" || model.Verification == "pending-verification"
		}
		if candidates != tc.wantCandidates || len(result.Items) == 0 {
			t.Fatalf("%s/%s: candidates=%v items=%d", tc.environment, tc.requested, candidates, len(result.Items))
		}
	}
}

func TestReleaseImageActivationAndRetryCheckStoredModelBeforeEffects(t *testing.T) {
	for _, action := range []string{"activate", "retry"} {
		for _, env := range []string{"development", "production", ""} {
			t.Run(action+"/"+env, func(t *testing.T) {
				h, jobs, sessions := newImageJobTestHandler(t)
				h.cfg.Environment = env
				account := uuid.New()
				job := newPreparedWebImageJobForReplay(t, account, uuid.New(), "fixture", nil)
				var params map[string]any
				_ = json.Unmarshal(job.Params, &params)
				params["model_id"] = "release-pending-fixture"
				for _, c := range providermodels.MediaCandidates() {
					if c.Kind == "image" {
						params["model_id"] = c.PublicID
						break
					}
				}
				job.Params, _ = json.Marshal(params)
				h.deps.ImageJobReader = &imageJobReaderStub{job: job}
				jobs.activateJob = job
				req := safeImageMutationRequest(t, sessions, account, "POST", "/web/v1/image-jobs/"+job.ID.String()+"/"+action, nil)
				view := "prod"
				if env != "development" {
					view = "dev"
				}
				req.Header.Set("X-Epifera-Release-View", view)
				w := httptest.NewRecorder()
				h.Routes().ServeHTTP(w, req)
				if w.Code != http.StatusForbidden || jobs.activateCalls != 0 || jobs.retryCalls != 0 {
					t.Fatalf("status=%d activate=%d retry=%d", w.Code, jobs.activateCalls, jobs.retryCalls)
				}
			})
		}
	}
}

func TestReleaseProductionRejectsMusicAndSpeechBeforePreparation(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"music-jobs", `{"model_id":"suno_v6","music":{"action":"generate","prompt":"fixture"}}`},
		{"speech-jobs", `{"model_id":"whisper_1","speech":{"format":"json"},"audio_artifact_id":"` + uuid.NewString() + `"}`},
	} {
		h, jobs, sessions := newImageJobTestHandler(t)
		h.cfg.Environment = "production"
		req := safeImageMutationRequest(t, sessions, uuid.New(), "POST", "/web/v1/"+tc.path+"/prepare", nil)
		req.Body = httptest.NewRequest("POST", "/", strings.NewReader(tc.body)).Body
		req.Header.Set("X-Epifera-Release-View", "dev")
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, req)
		if w.Code != http.StatusForbidden || jobs.prepareCalls != 0 {
			t.Fatalf("%s: status=%d prepare=%d", tc.path, w.Code, jobs.prepareCalls)
		}
	}
}

func TestReleaseDoesNotAffectOwnedHistory(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	h.cfg.Environment = "production"
	account := uuid.New()
	job := newPreparedWebImageJobForReplay(t, account, uuid.New(), "fixture", nil)
	job.Status = domain.JobStatusSucceeded
	var historicalParams map[string]any
	_ = json.Unmarshal(job.Params, &historicalParams)
	historicalParams["model_id"] = "seedream_5_0_flash"
	job.Params, _ = json.Marshal(historicalParams)
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	req := authenticatedConversationRequest(t, "GET", "/web/v1/image-jobs/"+job.ID.String(), sessions, account)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("owned history blocked: %d", w.Code)
	}
}

func configureReleaseCandidates(t *testing.T, h *Handler) {
	t.Helper()
	configureVideoExpansionTest(t, h)
	prices := h.deps.ImagePricing.(*pricingcatalog.Catalog)
	runtime, err := productcatalog.FromConfig(config.Config{Env: "development", FeatureDEVModelSmokeEnabled: true, FeatureVideoRouterEnabled: true, APIMartProviderEnabled: true, APIMartAPIKey: "fixture", APIMartBaseURL: "https://api.example.test"}, prices)
	if err != nil {
		t.Fatal(err)
	}
	h.cfg.TextModels = runtime.TextModels
	for _, m := range runtime.ImageModels() {
		h.cfg.ImageModels = append(h.cfg.ImageModels, imagegeneration.PublicModel{ID: m.ID, Name: m.Name, Enabled: m.Enabled, Ready: m.Enabled, QualityOptions: m.QualityOptions, DefaultQuality: m.DefaultQuality, MaxOutputCount: m.MaxOutputCount, AllowedAspectRatios: m.AllowedAspectRatios})
	}
}

func TestReleaseAllLegacyCatalogsExcludeDEVModels(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	configureReleaseCandidates(t, h)
	for _, path := range []string{"text-models", "chat-models", "image-models", "video-models"} {
		for _, view := range []string{"dev", "prod", "dev"} {
			req := authenticatedConversationRequest(t, "GET", "/web/v1/"+path, sessions, uuid.New())
			req.Header.Set("X-Epifera-Release-View", view)
			w := httptest.NewRecorder()
			h.Routes().ServeHTTP(w, req)
			var result struct {
				Items []struct {
					ID string `json:"id"`
				} `json:"items"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
				t.Fatalf("%s/%s status=%d", path, view, w.Code)
			}
			found := false
			for _, m := range result.Items {
				if providermodels.RuntimeRegistry().IsDEVSmokeModel(m.ID) {
					found = true
				}
			}
			if found != (view == "dev") {
				t.Fatalf("%s/%s DEV model present=%v", path, view, found)
			}
		}
	}
}

func TestReleaseConversationCandidatesCannotBypassCatalog(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	configureReleaseCandidates(t, h)
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	ids := map[string]string{}
	for _, m := range h.cfg.TextModels {
		if providermodels.RuntimeRegistry().IsDEVSmokeModel(m.ID) {
			ids["text"] = m.ID
			break
		}
	}
	for _, m := range h.cfg.ImageModels {
		if providermodels.RuntimeRegistry().IsDEVSmokeModel(m.ID) {
			ids["image"] = m.ID
			break
		}
	}
	for _, m := range h.cfg.VideoRoutes {
		if providermodels.RuntimeRegistry().IsDEVSmokeModel(m.Alias) {
			ids["video"] = m.Alias
			break
		}
	}
	if len(ids) != 3 {
		t.Fatalf("incomplete fixture: %v", ids)
	}
	for kind, id := range ids {
		r := safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), `{"prompt":"fixture","model_id":"`+id+`"}`)
		r.Header.Set("X-Epifera-Release-View", "prod")
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || len(jobs.inputs) != 0 {
			t.Fatalf("%s bypass: status=%d writes=%d", kind, w.Code, len(jobs.inputs))
		}
	}
}

func TestReleaseImagePrepareBlocksDEVModelAndPreservesRuntimeAdmission(t *testing.T) {
	h, jobs, sessions := newImageJobTestHandler(t)
	configureReleaseCandidates(t, h)
	var candidate imagegeneration.PublicModel
	for _, m := range h.cfg.ImageModels {
		if providermodels.RuntimeRegistry().IsDEVSmokeModel(m.ID) {
			candidate = m
			break
		}
	}
	if candidate.ID == "" {
		t.Fatal("missing DEV image fixture")
	}
	r := safeImageMutationRequest(t, sessions, uuid.New(), "POST", "/web/v1/image-jobs/prepare", map[string]string{"prompt": "fixture", "model_id": candidate.ID, "image_quality": candidate.DefaultQuality})
	r.Header.Set("X-Epifera-Release-View", "prod")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || jobs.prepareCalls != 0 {
		t.Fatalf("DEV image prepared: %d/%d", w.Code, jobs.prepareCalls)
	}
	h.cfg.ImageModels = nil
	r = safeImageMutationRequest(t, sessions, uuid.New(), "POST", "/web/v1/image-jobs/prepare", map[string]string{"prompt": "fixture", "model_id": "nano-banana-2"})
	r.Header.Set("X-Epifera-Release-View", "dev")
	w = httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code < 400 || jobs.prepareCalls != 0 {
		t.Fatal("release flag enabled runtime-disabled model")
	}
}

func TestReleaseProductionBaselineActivationPreservesBillingAndOwnership(t *testing.T) {
	for _, action := range []string{"activate", "retry"} {
		for _, foreign := range []bool{false, true} {
			h, jobs, sessions := newImageJobTestHandler(t)
			h.cfg.Environment = "production"
			owner := uuid.New()
			job := newPreparedWebImageJobForReplay(t, owner, uuid.New(), "fixture", nil)
			h.deps.ImageJobReader = &imageJobReaderStub{job: job}
			jobs.activateJob, jobs.retryJob = job, job
			jobs.activateErr, jobs.retryErr = domain.ErrInsufficientCredits, domain.ErrInsufficientCredits
			if foreign {
				owner = uuid.New()
			}
			r := safeImageMutationRequest(t, sessions, owner, "POST", "/web/v1/image-jobs/"+job.ID.String()+"/"+action, nil)
			w := httptest.NewRecorder()
			h.Routes().ServeHTTP(w, r)
			want, calls := http.StatusPaymentRequired, 1
			if foreign {
				want, calls = http.StatusNotFound, 0
			}
			if w.Code != want || jobs.activateCalls+jobs.retryCalls != calls {
				t.Fatalf("%s foreign=%v status=%d calls=%d", action, foreign, w.Code, jobs.activateCalls+jobs.retryCalls)
			}
		}
	}
}

func TestReleaseAudioActivationBlocksPreparedDEVJobs(t *testing.T) {
	if err := providermodels.ConfigureDEVSmoke("development", true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = providermodels.ConfigureDEVSmoke("development", false) })
	for _, kind := range []string{"music", "speech"} {
		h, jobs, sessions := newImageJobTestHandler(t)
		owner := uuid.New()
		job := &domain.Job{ID: uuid.New(), AccountID: owner, Source: "web", Status: domain.JobStatusPrepared, ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}}
		if kind == "music" {
			p, q, err := musicgeneration.Resolve(musicgeneration.Request{ModelID: "suno_v6", Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "fixture"}}, providermodels.RuntimeRegistry())
			if err != nil {
				t.Fatal(err)
			}
			job.Params, _ = json.Marshal(p)
			job.PricingSnapshot = mustMarshalPricingSnapshot(q)
			job.CostEstimate = q.InternalCredits
			job.OperationType = domain.OperationAudioMusic
			job.Modality = domain.ModalityAudio
		} else {
			prices, _ := pricingcatalog.NewStaticCatalog()
			if err := prices.AddSupplemental(providermodels.RuntimeRegistry().DEVSmokePrices()); err != nil {
				t.Fatal(err)
			}
			p, q, err := speechgeneration.Resolve(speechgeneration.Request{ModelID: "whisper_1", AudioArtifactID: uuid.New(), Speech: domain.SpeechRequest{Format: "json"}}, providermodels.RuntimeRegistry(), prices, 60000)
			if err != nil {
				t.Fatal(err)
			}
			job.Params, _ = json.Marshal(p)
			job.PricingSnapshot = mustMarshalPricingSnapshot(q)
			job.CostEstimate = q.InternalCredits
			job.OperationType = domain.OperationAudioSTT
			job.Modality = domain.ModalityText
		}
		h.deps.ImageJobReader = &imageJobReaderStub{job: job}
		jobs.activateJob = job
		r := safeImageMutationRequest(t, sessions, owner, "POST", "/web/v1/"+kind+"-jobs/"+job.ID.String()+"/activate", nil)
		r.Header.Set("X-Epifera-Release-View", "prod")
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || jobs.activateCalls != 0 {
			t.Fatalf("%s status=%d calls=%d", kind, w.Code, jobs.activateCalls)
		}
	}
}
