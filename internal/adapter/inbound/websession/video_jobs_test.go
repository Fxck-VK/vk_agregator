package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/resultservice"
)

func TestVideoJobHistoryListsOnlyAccountWebVideos(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobs := memory.NewJobRepo()
	visible := newWebVideoHistoryJob(t, accountID, uuid.New(), time.Now().UTC())
	visible.CostCaptured = 0
	mustCreateVideoHistoryJob(t, jobs, visible)
	foreign := newWebVideoHistoryJob(t, uuid.New(), uuid.New(), time.Now().UTC().Add(-time.Minute))
	mustCreateVideoHistoryJob(t, jobs, foreign)
	wrongSource := newWebVideoHistoryJob(t, accountID, uuid.New(), time.Now().UTC().Add(-2*time.Minute))
	wrongSource.Source = "miniapp"
	mustCreateVideoHistoryJob(t, jobs, wrongSource)
	wrongModality := newWebVideoHistoryJob(t, accountID, uuid.New(), time.Now().UTC().Add(-3*time.Minute))
	wrongModality.Modality = domain.ModalityImage
	mustCreateVideoHistoryJob(t, jobs, wrongModality)
	h.deps.ImageJobHistory = jobs

	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-jobs?limit=10", sessions, accountID)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []struct {
			ID           uuid.UUID        `json:"id"`
			ModelID      string           `json:"model_id"`
			ModelName    string           `json:"model_name"`
			Prompt       string           `json:"prompt"`
			DurationSec  int              `json:"duration_sec"`
			Resolution   string           `json:"resolution"`
			AspectRatio  string           `json:"aspect_ratio"`
			CostEstimate int64            `json:"cost_estimate"`
			Status       domain.JobStatus `json:"status"`
		} `json:"items"`
		HasMore bool `json:"has_more"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ID != visible.ID || response.HasMore {
		t.Fatalf("video history response = %+v", response)
	}
	item := response.Items[0]
	if item.ModelID != string(domain.VideoRouteKlingV3) || item.ModelName != "Kling v3" || item.Prompt != "video prompt" || item.DurationSec != 5 || item.Resolution != "720p" || item.AspectRatio != "16:9" || item.CostEstimate != visible.CostEstimate || item.Status != domain.JobStatusSucceeded {
		t.Fatalf("video history item = %+v", item)
	}
}

func TestVideoJobResultRequiresModeratedSucceededVideoOutput(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID := uuid.New()
	jobID := uuid.New()
	artifactID := uuid.New()
	jobs := memory.NewJobRepo()
	artifacts := memory.NewArtifactRepo()
	moderation := memory.NewModerationRepo()
	job := newWebVideoHistoryJob(t, accountID, jobID, time.Now().UTC())
	job.OutputArtifactIDs = []uuid.UUID{artifactID}
	mustCreateVideoHistoryJob(t, jobs, job)
	mustCreateVideoArtifact(t, artifacts, accountID, jobID, artifactID)
	h.deps.ImageJobReader = jobs
	h.deps.ImageResults = resultservice.New(jobs, artifacts, moderation)

	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-jobs/"+jobID.String()+"/result", sessions, accountID)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unmoderated status = %d, body = %s", rec.Code, rec.Body.String())
	}

	artifactRef := artifactID
	if err := moderation.Create(req.Context(), &domain.ModerationResult{JobID: jobID, ArtifactID: &artifactRef, Stage: domain.ModerationStageOutput, Decision: domain.ModerationAllow, Provider: "keyword"}); err != nil {
		t.Fatalf("create moderation result: %v", err)
	}
	rec = httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("moderated status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		JobID     uuid.UUID        `json:"job_id"`
		Status    domain.JobStatus `json:"status"`
		Artifacts []struct {
			ID         uuid.UUID `json:"id"`
			MIMEType   string    `json:"mime_type"`
			SizeBytes  int64     `json:"size_bytes"`
			Width      int       `json:"width"`
			Height     int       `json:"height"`
			DurationMS int64     `json:"duration_ms"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode video result: %v", err)
	}
	if response.JobID != jobID || response.Status != domain.JobStatusSucceeded || len(response.Artifacts) != 1 {
		t.Fatalf("video result response = %+v", response)
	}
	artifact := response.Artifacts[0]
	if artifact.ID != artifactID || artifact.MIMEType != "video/mp4" || artifact.SizeBytes != 123 || artifact.Width != 1280 || artifact.Height != 720 || artifact.DurationMS != 5000 {
		t.Fatalf("video result artifact = %+v", artifact)
	}
}

func TestVideoJobRoutesRejectInvalidWebHistoryContract(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.Job)
	}{
		{name: "VK user", mutate: func(job *domain.Job) { job.UserID = uuid.New() }},
		{name: "VK peer", mutate: func(job *domain.Job) { job.VKPeerID = 123 }},
		{name: "VK command", mutate: func(job *domain.Job) { job.CommandID = uuid.New() }},
		{name: "missing channel", mutate: func(job *domain.Job) { job.ChannelContext = nil }},
		{name: "miniapp channel", mutate: func(job *domain.Job) { job.ChannelContext.Channel = domain.ChannelVKMiniApp }},
		{name: "recipient reference", mutate: func(job *domain.Job) { job.ChannelContext.RecipientRef = "test-peer" }},
		{name: "thread reference", mutate: func(job *domain.Job) { job.ChannelContext.ThreadRef = "test-thread" }},
		{name: "external result", mutate: func(job *domain.Job) {
			job.ResultMode = domain.ResultModeExternalPush
			job.DeliveryTarget = &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "test-peer"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _, sessions := newImageJobTestHandler(t)
			accountID, jobID, artifactID := uuid.New(), uuid.New(), uuid.New()
			jobs := memory.NewJobRepo()
			artifacts := memory.NewArtifactRepo()
			moderation := memory.NewModerationRepo()
			job := newWebVideoHistoryJob(t, accountID, jobID, time.Now().UTC())
			job.OutputArtifactIDs = []uuid.UUID{artifactID}
			tc.mutate(job)
			mustCreateVideoHistoryJob(t, jobs, job)
			mustCreateVideoArtifact(t, artifacts, accountID, jobID, artifactID)
			if err := moderation.Create(t.Context(), &domain.ModerationResult{JobID: jobID, ArtifactID: &artifactID, Stage: domain.ModerationStageOutput, Decision: domain.ModerationAllow, Provider: "keyword"}); err != nil {
				t.Fatalf("create moderation result: %v", err)
			}
			h.deps.ImageJobHistory = jobs
			h.deps.ImageJobReader = jobs
			h.deps.ImageResults = resultservice.New(jobs, artifacts, moderation)
			for _, route := range []struct {
				path string
				want int
			}{
				{path: "/web/v1/video-jobs", want: http.StatusServiceUnavailable},
				{path: "/web/v1/video-jobs/" + jobID.String() + "/result", want: http.StatusNotFound},
			} {
				req := authenticatedConversationRequest(t, http.MethodGet, route.path, sessions, accountID)
				rec := httptest.NewRecorder()
				h.Routes().ServeHTTP(rec, req)
				if rec.Code != route.want {
					t.Errorf("status = %d, want %d for invalid web history contract", rec.Code, route.want)
				}
			}
		})
	}
}

func TestVideoJobProjectionRejectsInvalidResultContract(t *testing.T) {
	job := newWebVideoHistoryJob(t, uuid.New(), uuid.New(), time.Now().UTC())
	job.DeliveryTarget = &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "test-peer"}
	if _, ok := newSafeVideoJob(job); ok {
		t.Fatal("accepted an account history job with an external delivery target")
	}
}

func TestVideoJobResultRejectsForeignOwner(t *testing.T) {
	h, _, sessions := newImageJobTestHandler(t)
	accountID, jobID := uuid.New(), uuid.New()
	jobs := memory.NewJobRepo()
	mustCreateVideoHistoryJob(t, jobs, newWebVideoHistoryJob(t, uuid.New(), jobID, time.Now().UTC()))
	h.deps.ImageJobReader = jobs
	h.deps.ImageResults = resultservice.New(jobs, memory.NewArtifactRepo(), memory.NewModerationRepo())
	req := authenticatedConversationRequest(t, http.MethodGet, "/web/v1/video-jobs/"+jobID.String()+"/result", sessions, accountID)
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign owner status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func newWebVideoHistoryJob(t *testing.T, accountID, jobID uuid.UUID, createdAt time.Time) *domain.Job {
	t.Helper()
	return &domain.Job{
		ID:             jobID,
		AccountID:      accountID,
		Source:         "web",
		ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		ResultMode:     domain.ResultModeAccountHistory,
		OperationType:  domain.OperationVideoGenerate,
		Modality:       domain.ModalityVideo,
		Status:         domain.JobStatusSucceeded,
		CostEstimate:   290,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		IdempotencyKey: "video-history:" + jobID.String(),
		Params: mustMarshalWebVideoJobParams(t, map[string]any{
			"prompt":            "video prompt",
			"model_id":          string(domain.VideoRouteKlingV3),
			"model_name":        "Kling v3",
			"video_route_alias": string(domain.VideoRouteKlingV3),
			"duration_sec":      5,
			"resolution":        "720p",
			"aspect_ratio":      "16:9",
		}),
	}
}

func mustCreateVideoHistoryJob(t *testing.T, repo *memory.JobRepo, job *domain.Job) {
	t.Helper()
	if err := repo.Create(t.Context(), job); err != nil {
		t.Fatalf("create video history job: %v", err)
	}
}

func mustCreateVideoArtifact(t *testing.T, repo *memory.ArtifactRepo, accountID, jobID, artifactID uuid.UUID) {
	t.Helper()
	if err := repo.Create(t.Context(), &domain.Artifact{
		ID:             artifactID,
		OwnerAccountID: accountID,
		JobID:          &jobID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      domain.MediaTypeVideo,
		MimeType:       "video/mp4",
		StorageBucket:  "artifacts",
		StorageKey:     "video.mp4",
		SizeBytes:      123,
		Width:          1280,
		Height:         720,
		DurationMS:     5000,
		Status:         domain.ArtifactStatusReady,
	}); err != nil {
		t.Fatalf("create video artifact: %v", err)
	}
}

func mustMarshalWebVideoJobParams(t *testing.T, params map[string]any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal video job params: %v", err)
	}
	return payload
}
