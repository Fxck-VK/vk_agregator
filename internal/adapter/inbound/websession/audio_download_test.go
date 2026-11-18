package websession

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/resultservice"
)

const audioDownloadJobsPath = "/web/v1/audio-download-jobs"

func TestAudioDownloadRequiresSessionAndCSRF(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	routes := h.Routes()
	for _, path := range []string{audioDownloadJobsPath + "/" + uuid.NewString(), "/web/v1/audio-download-artifacts/" + uuid.NewString()} {
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s: status %d, want 401", path, w.Code)
		}
	}
	r := safeConversationManagementRequest(t, http.MethodPost, audioDownloadJobsPath, sessions, uuid.New(), `{"url":"https://youtu.be/abcdefghijk"}`)
	r.Header.Del("X-CSRF-Token")
	w := httptest.NewRecorder()
	routes.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF: status %d, want 403", w.Code)
	}
	r = safeConversationManagementRequest(t, http.MethodPost, audioDownloadJobsPath, sessions, uuid.New(), `{"url":"https://youtu.be/abcdefghijk"}`)
	r.Header.Del("Cookie")
	r.AddCookie(&http.Cookie{Name: "nh_csrf", Value: r.Header.Get("X-CSRF-Token")})
	w = httptest.NewRecorder()
	routes.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("POST without an access cookie: status %d, want 401", w.Code)
	}
}

func TestAudioDownloadCreationNormalizesURLAndScopesReplay(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	jobs := &audioDownloadJobsStub{jobs: map[string]*domain.Job{}}
	h.deps.WebChatJobs = jobs
	h.deps.ImageJobIdempotency = jobs
	h.deps.ImageJobPrepareLimiter = audioDownloadLimiter(true)
	routes := h.Routes()
	account := uuid.New()
	key := uuid.NewString()
	post := func(body string) *httptest.ResponseRecorder {
		t.Helper()
		r := safeConversationManagementRequest(t, http.MethodPost, audioDownloadJobsPath, sessions, account, body)
		r.Header.Set("X-Idempotency-Key", key)
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, r)
		return w
	}

	fresh := post(`{"url":"https://youtu.be/abcdefghijk?si=tracking"}`)
	if fresh.Code != http.StatusCreated || jobs.creates != 1 {
		t.Fatalf("create: status=%d, queued=%d, body=%s", fresh.Code, jobs.creates, fresh.Body.String())
	}
	in := jobs.last
	if in.AccountID != account || in.UserID != uuid.Nil || in.Source != "web" ||
		in.Operation != domain.OperationType("audio_download") || in.Modality != domain.ModalityAudio ||
		in.ResultMode != domain.ResultModeAccountHistory || in.ChannelContext == nil ||
		in.ChannelContext.Channel != domain.ChannelWeb || in.DeliveryTarget != nil || in.CostEstimateCredits != 0 ||
		!strings.Contains(in.IdempotencyKey, account.String()) {
		t.Fatalf("incorrect persisted contract: %+v", in)
	}
	if string(in.Params) != `{"source_url":"https://www.youtube.com/watch?v=abcdefghijk"}` || strings.Contains(fresh.Body.String(), "youtube") {
		t.Fatalf("unsafe or unnormalized source URL in input/response")
	}

	if w := post(`{"url":"https://www.youtube.com/watch?v=abcdefghijk"}`); w.Code != http.StatusOK || w.Body.String() != fresh.Body.String() || jobs.creates != 1 {
		t.Fatalf("canonical replay: status=%d, queued=%d", w.Code, jobs.creates)
	}
	if w := post(`{"url":"https://soundcloud.com/example-artist/example-track"}`); w.Code != http.StatusConflict || jobs.creates != 1 {
		t.Fatalf("changed replay: status=%d, queued=%d", w.Code, jobs.creates)
	}
	for _, body := range []string{
		`{"url":"https://www.youtube.com.evil.example/watch?v=abcdefghijk"}`,
		`{"url":"https://youtu.be/abcdefghijk","cost_estimate":10}`,
	} {
		if w := post(body); w.Code != http.StatusBadRequest || jobs.creates != 1 {
			t.Fatalf("invalid request: status=%d, queued=%d", w.Code, jobs.creates)
		}
	}
	account = uuid.New()
	if w := post(`{"url":"https://soundcloud.com/example-artist/example-track"}`); w.Code != http.StatusCreated || jobs.creates != 2 {
		t.Fatalf("account-scoped key: status=%d, queued=%d", w.Code, jobs.creates)
	}
}

func TestAudioDownloadRequiresOwnedModeratedMP3(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	account := uuid.New()
	job := &domain.Job{
		ID: uuid.New(), AccountID: account, Source: "web", Status: domain.JobStatusSucceeded,
		OperationType: domain.OperationType("audio_download"), Modality: domain.ModalityAudio,
		ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
	}
	artifact := &domain.Artifact{
		ID: uuid.New(), JobID: &job.ID, OwnerAccountID: account, Status: domain.ArtifactStatusReady, Kind: domain.ArtifactKindOutput,
		MediaType: domain.MediaTypeAudio, MimeType: "audio/mpeg", SizeBytes: 10, StorageBucket: "private", StorageKey: "file.mp3",
	}
	job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
	store := &imageArtifactObjectStoreStub{data: []byte("0123456789")}
	results := &imageResultReaderStub{result: resultservice.Result{ID: job.ID, Artifacts: []resultservice.ArtifactMetadata{{
		ID: artifact.ID, MediaType: artifact.MediaType, MIMEType: artifact.MimeType,
	}}}}
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: artifact}
	h.deps.ImageResults = results
	h.deps.ImageArtifactURLSigner = store
	routes := h.Routes()
	get := func(path string, owner uuid.UUID) *httptest.ResponseRecorder {
		t.Helper()
		r := safeConversationManagementRequest(t, http.MethodGet, path, sessions, owner, "")
		w := httptest.NewRecorder()
		routes.ServeHTTP(w, r)
		return w
	}
	jobPath := audioDownloadJobsPath + "/" + job.ID.String()
	artifactPath := "/web/v1/audio-download-artifacts/" + artifact.ID.String()
	if w := get(jobPath, account); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), artifact.ID.String()) {
		t.Fatalf("owned status: %d %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct{ params, filename string }{
		{`{}`, "audio epifera.com.mp3"},
		{`{"download_filename":"Мой трек - Исполнитель epifera.com.mp3"}`, "Мой трек - Исполнитель epifera.com.mp3"},
		{`{"download_filename":"Название ролика epifera.com.mp3"}`, "Название ролика epifera.com.mp3"},
	} {
		job.Params = json.RawMessage(tc.params)
		w := get(artifactPath, account)
		disposition, parameters, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "audio/mpeg" || w.Body.String() != "0123456789" ||
			err != nil || disposition != "attachment" || parameters["filename"] != tc.filename {
			t.Fatalf("owned MP3: %d %q, %s", w.Code, w.Header().Get("Content-Disposition"), w.Body.String())
		}
	}
	other := uuid.New()
	if w := get(jobPath, other); w.Code != http.StatusNotFound {
		t.Fatalf("foreign status: %d", w.Code)
	}
	if w := get(artifactPath, other); w.Code != http.StatusNotFound {
		t.Fatalf("foreign MP3: %d", w.Code)
	}
	results.err = domain.ErrNotFound
	if w := get(jobPath, account); w.Code != http.StatusOK || strings.Contains(w.Body.String(), artifact.ID.String()) {
		t.Fatalf("unmoderated artifact disclosed in job status: %d %s", w.Code, w.Body.String())
	}
	if w := get(artifactPath, account); w.Code != http.StatusNotFound {
		t.Fatalf("unmoderated artifact: %d", w.Code)
	}
	results.err = nil
	artifact.MimeType = "audio/wav"
	if w := get(artifactPath, account); w.Code != http.StatusNotFound {
		t.Fatalf("non-MP3 artifact: %d", w.Code)
	}
	job.Status = domain.JobStatusFailedTerminal
	for _, pair := range []struct{ stored, public string }{
		{"download_failed", "download_failed"},
		{"download_unavailable", "download_unavailable"},
		{"download_too_large", "download_too_large"},
		{"download_timeout", "download_timeout"},
		{"content_rejected", "content_rejected"},
		{"private_details", "download_failed"},
	} {
		job.ErrorCode = pair.stored
		w := get(jobPath, account)
		var dto struct {
			ErrorCode string `json:"error_code"`
		}
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &dto) != nil || dto.ErrorCode != pair.public {
			t.Fatalf("terminal status: %d %s, want public error %q", w.Code, w.Body.String(), pair.public)
		}
	}
}

type audioDownloadJobsStub struct {
	jobs    map[string]*domain.Job
	last    joborchestrator.CreateJobInput
	creates int
}

func (s *audioDownloadJobsStub) CreateJob(_ context.Context, in joborchestrator.CreateJobInput) (*domain.Job, error) {
	s.last = in
	s.creates++
	job := &domain.Job{
		ID: uuid.New(), AccountID: in.AccountID, Source: in.Source, Status: domain.JobStatusQueued,
		OperationType: in.Operation, Modality: in.Modality, Params: json.RawMessage(in.Params),
		ChannelContext: in.ChannelContext, ResultMode: in.ResultMode, IdempotencyKey: in.IdempotencyKey,
	}
	s.jobs[in.IdempotencyKey] = job
	return job, nil
}

func (s *audioDownloadJobsStub) GetByIdempotencyKeyForAccount(_ context.Context, account uuid.UUID, key string) (*domain.Job, error) {
	if job := s.jobs[key]; job != nil && job.AccountID == account {
		return job, nil
	}
	return nil, domain.ErrNotFound
}

type audioDownloadLimiter bool

func (l audioDownloadLimiter) Allow(context.Context, string) (bool, error) { return bool(l), nil }
