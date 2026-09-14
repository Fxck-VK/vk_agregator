package websession

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
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
	body := `{"prompt":"Synthetic crane","model_id":"nano_banana_2","image_quality":"2K","aspect_ratio":"4:5","output_count":2}`
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, safeWebConversationMessageRequest(t, sessions, owner, conversation.ID, uuid.New(), body))
	if rec.Code != http.StatusCreated || len(jobs.inputs) != 1 {
		t.Fatalf("image submit: %d %s", rec.Code, rec.Body.String())
	}
	in := jobs.inputs[0]
	var params map[string]any
	_ = json.Unmarshal(in.Params, &params)
	if in.Operation != domain.OperationImageGenerate || in.Modality != domain.ModalityImage || in.PricingSnapshot.InternalCredits != 120 || params["conversation_id"] != conversation.ID.String() || params["output_count"] != float64(2) {
		t.Fatal("image price, operation or conversation lost")
	}
	for _, invalid := range []string{
		`{"prompt":"Synthetic","model_id":"nano_banana_2","image_quality":"unknown"}`,
		`{"prompt":"Synthetic","model_id":"nano_banana_2","duration_sec":5}`,
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
