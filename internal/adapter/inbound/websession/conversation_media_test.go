package websession

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/resultservice"
)

func TestConversationImageUsesOwnedJobAndServerPrice(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.cfg.ImageModels, h.deps.ImagePricing = images.cfg.ImageModels, images.deps.ImagePricing
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	body := `{"prompt":"Synthetic crane","model_id":"nano_banana_2","image_quality":"2K","aspect_ratio":"4:5","output_count":1}`
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body))
	if rec.Code != http.StatusCreated || len(jobs.inputs) != 1 {
		t.Fatalf("image submit: %d %s", rec.Code, rec.Body.String())
	}
	in := jobs.inputs[0]
	var params map[string]any
	_ = json.Unmarshal(in.Params, &params)
	if in.Operation != domain.OperationImageGenerate || in.Modality != domain.ModalityImage || in.PricingSnapshot.InternalCredits != 60 || params["conversation_id"] != conversation.ID.String() || params["output_count"] != float64(1) {
		t.Fatal("image price, operation or conversation lost")
	}
	for _, invalid := range []string{
		`{"prompt":"Synthetic","model_id":"nano_banana_2","image_quality":"unknown"}`,
		`{"prompt":"Synthetic","model_id":"nano_banana_2","duration_sec":5}`,
		`{"prompt":"Synthetic","model_id":"nano_banana_2","output_count":2}`,
		`{"prompt":"Synthetic","model_id":"nano_banana_2","output_count":500}`,
		`{"prompt":"Synthetic","model_id":"nano_banana_2","cost_estimate":1}`,
	} {
		rec = httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), invalid))
		if rec.Code != 400 || len(jobs.inputs) != 1 {
			t.Fatal("invalid media intent executed")
		}
	}
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, uuid.New(), conversation.ID, uuid.New(), body))
	if rec.Code != 404 || len(jobs.inputs) != 1 {
		t.Fatal("foreign conversation accepted")
	}
}

func TestConversationSeedreamPromptBoundaryRejectsBeforeJobs(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.deps.ImagePricing = images.deps.ImagePricing
	h.cfg.ImageModels = []imagegeneration.PublicModel{seedream45PublicImageModel(t)}
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}

	rec := submitConversationMedia(t, h, sessions, owner, conversation.ID, map[string]any{
		"prompt":        strings.Repeat("я", 3001),
		"model_id":      modelcatalog.MiniAppImageSeedream45,
		"image_quality": modelcatalog.ImageQuality2K,
		"aspect_ratio":  "1:1",
	})
	if rec.Code != http.StatusBadRequest || len(jobs.inputs) != 0 {
		t.Fatalf("overlong Seedream prompt executed: status=%d jobs=%d body=%s", rec.Code, len(jobs.inputs), rec.Body.String())
	}

	rec = submitConversationMedia(t, h, sessions, owner, conversation.ID, map[string]any{
		"prompt":        strings.Repeat("я", 3000),
		"model_id":      modelcatalog.MiniAppImageSeedream45,
		"image_quality": modelcatalog.ImageQuality2K,
		"aspect_ratio":  "1:1",
	})
	if rec.Code != http.StatusCreated || len(jobs.inputs) != 1 {
		t.Fatalf("boundary Seedream prompt rejected: status=%d jobs=%d body=%s", rec.Code, len(jobs.inputs), rec.Body.String())
	}
	if jobs.inputs[0].PricingSnapshot.InternalCredits != 30 {
		t.Fatalf("Seedream boundary price = %d, want 30", jobs.inputs[0].PricingSnapshot.InternalCredits)
	}
}

func TestConversationVideoPromptBoundariesRejectBeforeJobs(t *testing.T) {
	cases := []struct {
		name       string
		alias      domain.VideoRouteAlias
		prompt     string
		wantStatus int
		wantJobs   int
	}{
		{name: "runway over max", alias: domain.VideoRouteRunwayGen45, prompt: strings.Repeat("я", 1801), wantStatus: http.StatusBadRequest},
		{name: "runway max", alias: domain.VideoRouteRunwayGen45, prompt: strings.Repeat("я", 1800), wantStatus: http.StatusCreated, wantJobs: 1},
		{name: "seedance one char", alias: domain.VideoRouteSeedance20Fast, prompt: "я", wantStatus: http.StatusBadRequest},
		{name: "seedance two chars", alias: domain.VideoRouteSeedance20Fast, prompt: "яя", wantStatus: http.StatusBadRequest},
		{name: "seedance min", alias: domain.VideoRouteSeedance20Fast, prompt: "яяя", wantStatus: http.StatusCreated, wantJobs: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
			images, _, _ := newImageJobTestHandler(t)
			h.deps.ImagePricing = images.deps.ImagePricing
			h.cfg.VideoRoutes = []productcatalog.VideoRoute{conversationTestVideoRoute(tc.alias)}
			owner := uuid.New()
			conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
			jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}

			rec := submitConversationMedia(t, h, sessions, owner, conversation.ID, map[string]any{
				"prompt":       tc.prompt,
				"model_id":     string(tc.alias),
				"duration_sec": 5,
				"aspect_ratio": "16:9",
			})
			if rec.Code != tc.wantStatus || len(jobs.inputs) != tc.wantJobs {
				t.Fatalf("status=%d jobs=%d body=%s", rec.Code, len(jobs.inputs), rec.Body.String())
			}
		})
	}
}

func TestConversationRunwayExplicit1080RejectsBeforeJobs(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.deps.ImagePricing = images.deps.ImagePricing
	h.cfg.VideoRoutes = []productcatalog.VideoRoute{conversationTestVideoRoute(domain.VideoRouteRunwayGen45)}
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}

	rec := submitConversationMedia(t, h, sessions, owner, conversation.ID, map[string]any{
		"prompt":       "Synthetic motion prompt",
		"model_id":     string(domain.VideoRouteRunwayGen45),
		"resolution":   "1080p",
		"duration_sec": 5,
		"aspect_ratio": "16:9",
	})
	if rec.Code != http.StatusBadRequest || len(jobs.inputs) != 0 {
		t.Fatalf("explicit Runway 1080 executed: status=%d jobs=%d body=%s", rec.Code, len(jobs.inputs), rec.Body.String())
	}
}

func TestConversationVideoModelsExposeOperationalControls(t *testing.T) {
	h, _, sessions, _, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.deps.ImagePricing = images.deps.ImagePricing
	h.cfg.VideoRoutes = []productcatalog.VideoRoute{
		conversationTestVideoRoute(domain.VideoRouteRunwayGen45),
		conversationTestVideoRoute(domain.VideoRouteSeedance20Fast),
	}

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-models", sessions, uuid.New()))
	if rec.Code != http.StatusOK {
		t.Fatalf("video models: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []struct {
			ID                  string `json:"id"`
			AutomaticResolution bool   `json:"automatic_resolution,omitempty"`
			MinPromptChars      int    `json:"min_prompt_chars,omitempty"`
			MaxPromptChars      int    `json:"max_prompt_chars,omitempty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode video models: %v", err)
	}
	byID := map[string]struct {
		AutomaticResolution bool
		MinPromptChars      int
		MaxPromptChars      int
	}{}
	for _, item := range response.Items {
		byID[item.ID] = struct {
			AutomaticResolution bool
			MinPromptChars      int
			MaxPromptChars      int
		}{item.AutomaticResolution, item.MinPromptChars, item.MaxPromptChars}
	}
	runway := byID[string(domain.VideoRouteRunwayGen45)]
	if !runway.AutomaticResolution || runway.MinPromptChars != 1 || runway.MaxPromptChars != 1800 {
		t.Fatalf("Runway operational controls = %+v", runway)
	}
	seedance := byID[string(domain.VideoRouteSeedance20Fast)]
	if seedance.AutomaticResolution || seedance.MinPromptChars != 3 || seedance.MaxPromptChars != 2000 {
		t.Fatalf("Seedance operational controls = %+v", seedance)
	}
}

func submitConversationMedia(t *testing.T, h *Handler, sessions *sessionStub, accountID, conversationID uuid.UUID, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal conversation media request: %v", err)
	}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, accountID, conversationID, uuid.New(), string(raw)))
	return rec
}

func conversationTestVideoRoute(alias domain.VideoRouteAlias) productcatalog.VideoRoute {
	return productcatalog.VideoRoute{
		Alias:               string(alias),
		Name:                string(alias),
		Enabled:             true,
		AllowedResolutions:  []string{"720p"},
		AllowedDurationsSec: []int{5},
		AllowedAspectRatios: []string{"16:9"},
		DefaultResolution:   "720p",
		DefaultDurationSec:  5,
		DefaultAspectRatio:  "16:9",
	}
}

func TestConversationVideoCatalogAndSubmissionFailClosed(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.deps.ImagePricing = images.deps.ImagePricing
	h.cfg.VideoRoutes = []productcatalog.VideoRoute{{Alias: string(domain.VideoRouteVeo31Fast), Name: "Veo 3.1 Fast", Enabled: true, AllowedResolutions: []string{"720p"}, AllowedDurationsSec: []int{8}, AllowedAspectRatios: []string{"16:9", "9:16"}, DefaultResolution: "720p", DefaultDurationSec: 8, DefaultAspectRatio: "16:9"}}
	owner := uuid.New()
	conversation := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-models", sessions, owner))
	var catalog struct {
		Items []safeVideoModel `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &catalog)
	if rec.Code != 200 || len(catalog.Items) != 1 || catalog.Items[0].PriceByOption["720p:8"] <= 0 {
		t.Fatalf("video catalog: %d %s", rec.Code, rec.Body.String())
	}
	body := `{"prompt":"Synthetic moving crane","model_id":"video_veo_3_1_fast","resolution":"720p","duration_sec":8,"aspect_ratio":"9:16"}`
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body))
	if rec.Code != 201 || len(jobs.inputs) != 1 {
		t.Fatalf("video submit: %d %s", rec.Code, rec.Body.String())
	}
	in := jobs.inputs[0]
	if in.Operation != domain.OperationVideoGenerate || in.Modality != domain.ModalityVideo || in.PricingSnapshot.InternalCredits != catalog.Items[0].PriceByOption["720p:8"] {
		t.Fatal("video routing or billing lost")
	}
	h.cfg.VideoRoutes[0].RequiresStartImage = true
	if len(h.conversationVideoRoutes()) != 0 {
		t.Fatal("reference-only video advertised without web upload support")
	}
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body))
	if rec.Code != 400 || len(jobs.inputs) != 1 {
		t.Fatal("unavailable video submitted")
	}
}

func TestConversationAttachmentsRequireCompletedOwnedModeratedResults(t *testing.T) {
	h, conversations, sessions, jobs, _ := newWebConversationMessageTestHandler(t)
	images, _, _ := newImageJobTestHandler(t)
	h.cfg.ImageModels, h.deps.ImagePricing = images.cfg.ImageModels, images.deps.ImagePricing
	owner := uuid.New()
	conv := seedWebMessageConversation(t, conversations, owner, domain.ConversationSourceWeb)
	jobs.job = &domain.Job{ID: uuid.New(), Status: domain.JobStatusQueued}
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conv.ID, uuid.New(), `{"prompt":"Synthetic","model_id":"nano_banana_2"}`))
	if rec.Code != 201 {
		t.Fatalf("prepare: %d", rec.Code)
	}
	job := newPersistedWebChatJob(jobs.inputs[0], jobs.job.ID, domain.JobStatusSucceeded)
	job.CostEstimate = jobs.inputs[0].PricingSnapshot.InternalCredits
	job.CreatedAt, job.UpdatedAt = time.Now().UTC(), time.Now().UTC()
	artifactID := uuid.New()
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{ID: job.ID, Status: domain.JobStatusSucceeded, Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, Artifacts: []resultservice.ArtifactMetadata{{ID: artifactID, MediaType: domain.MediaTypeImage, MIMEType: "image/png", SizeBytes: 128}}}}
	message := safeConversationMessage{}
	if !h.attachConversationMedia(context.Background(), owner, conv.ID, job.ID, &message) || len(message.Images) != 1 || message.Images[0].Artifact.ID != artifactID {
		t.Fatal("owned media missing")
	}
	if h.attachConversationMedia(context.Background(), owner, uuid.New(), job.ID, &safeConversationMessage{}) {
		t.Fatal("cross-conversation media attached")
	}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{ID: job.ID, Status: domain.JobStatusResultReady}}
	if h.attachConversationMedia(context.Background(), owner, conv.ID, job.ID, &safeConversationMessage{}) {
		t.Fatal("pre-capture result exposed")
	}
}

func TestWebVideoArtifactRangeAndForeignOwnership(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	owner, jobID, artifactID, conversationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	params, _ := json.Marshal(map[string]string{"conversation_id": conversationID.String(), "conversation_source": "web"})
	job := &domain.Job{ID: jobID, AccountID: owner, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, ResultMode: domain.ResultModeAccountHistory, OperationType: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Status: domain.JobStatusSucceeded, Params: params}
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{ID: jobID, Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, Status: domain.JobStatusSucceeded, Artifacts: []resultservice.ArtifactMetadata{{ID: artifactID, MediaType: domain.MediaTypeVideo, MIMEType: "video/mp4", SizeBytes: 10}}}}
	artifact := &domain.Artifact{ID: artifactID, OwnerAccountID: owner, JobID: &jobID, Kind: domain.ArtifactKindOutput, MediaType: domain.MediaTypeVideo, MimeType: "video/mp4", SizeBytes: 10, StorageBucket: "private", StorageKey: "video.mp4", Status: domain.ArtifactStatusReady}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: artifact}
	h.deps.ImageArtifactURLSigner = &imageArtifactObjectStoreStub{data: []byte("0123456789")}
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-artifacts/"+artifactID.String(), sessions, owner)
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "2345" || rec.Header().Get("Content-Range") != "bytes 2-5/10" || rec.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("range response: %d %s %v", rec.Code, rec.Body.String(), rec.Header())
	}
	req = authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-artifacts/"+artifactID.String(), sessions, uuid.New())
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatal("foreign video artifact exposed")
	}
	h.deps.ImageResults = &imageResultReaderStub{err: domain.ErrNotFound}
	req = authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-artifacts/"+artifactID.String(), sessions, owner)
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatal("unmoderated video exposed")
	}
}
