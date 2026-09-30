package websession

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/resultservice"
)

func TestMusicArtifactsRequireOwnerAndModeratedResultAndSupportRange(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	job, artifact := musicAccessFixture(t, owner)
	store := &imageArtifactObjectStoreStub{data: []byte("0123456789")}
	results := &imageResultReaderStub{result: resultservice.Result{ID: job.ID, Artifacts: []resultservice.ArtifactMetadata{{ID: artifact.ID}}}}
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	h.deps.ImageArtifacts = &imageArtifactReaderStub{artifact: artifact}
	h.deps.ImageResults = results
	h.deps.ImageArtifactURLSigner = store
	path := "/web/v1/music-artifacts/" + artifact.ID.String()
	get := func(account uuid.UUID) *httptest.ResponseRecorder {
		r := safeConversationManagementRequest(t, http.MethodGet, path, sessions, account, "")
		r.Header.Set("Range", "bytes=2-5")
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, r)
		return w
	}
	w := get(owner)
	if w.Code != 206 || w.Body.String() != "2345" || w.Header().Get("Content-Range") != "bytes 2-5/10" || w.Header().Get("Location") != "" {
		t.Fatalf("invalid range response: %d %q", w.Code, w.Body.String())
	}
	store.key = ""
	if get(uuid.New()).Code != 404 || store.key != "" {
		t.Fatal("foreign account accessed stored bytes")
	}
	results.err = domain.ErrNotFound
	if get(owner).Code != 404 || store.key != "" {
		t.Fatal("blocked moderation accessed stored bytes")
	}
}

func TestMusicResultUsesSameOriginArtifactPathsAndOriginalTrackIndex(t *testing.T) {
	h, _, sessions := newTestHandler(t)
	owner := uuid.New()
	job, artifact := musicAccessFixture(t, owner)
	h.deps.ImageJobReader = &imageJobReaderStub{job: job}
	h.deps.ImageResults = &imageResultReaderStub{result: resultservice.Result{ID: job.ID, Artifacts: []resultservice.ArtifactMetadata{{ID: artifact.ID}}}}
	var params musicgeneration.JobParams
	if err := json.Unmarshal(job.Params, &params); err != nil {
		t.Fatal(err)
	}
	params.Result.Music.Persona = &domain.MusicPersonaResult{ID: "private-native-id-persona", Name: "Saved persona"}
	params.Result.Music.Model = &domain.MusicModelResult{ID: "private-native-id-model"}
	params.Result.Music.Voice = &domain.MusicVoiceResult{ID: "private-native-id-voice", Name: " "}
	job.Params, _ = json.Marshal(params)
	r := safeConversationManagementRequest(t, http.MethodGet, "/web/v1/music-jobs/"+job.ID.String()+"/result", sessions, owner, "")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var got struct {
		Tracks  []safeMusicTrack `json:"tracks"`
		Persona *safeMusicAsset  `json:"persona"`
		Model   *safeMusicAsset  `json:"model"`
		Voice   *safeMusicAsset  `json:"voice"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Tracks) != 1 || got.Tracks[0].AudioIndex != 7 || got.Tracks[0].AudioURL != "/web/v1/music-artifacts/"+artifact.ID.String() {
		t.Fatalf("incorrect track projection: %+v", got.Tracks)
	}
	if strings.Contains(w.Body.String(), "private-native-id") || strings.Contains(w.Body.String(), "https://") {
		t.Fatal("private routing leaked")
	}
	if got.Persona == nil || got.Persona.JobID != job.ID || got.Persona.Name != "Saved persona" {
		t.Fatal("persona must be referenced by owned application job")
	}
	if got.Model == nil || got.Model.JobID != job.ID || got.Model.Name != "Моя модель" || got.Voice == nil || got.Voice.JobID != job.ID || got.Voice.Name != "Мой голос" {
		t.Fatal("provider receipts without names need safe display labels")
	}
}

func musicAccessFixture(t *testing.T, owner uuid.UUID) (*domain.Job, *domain.Artifact) {
	t.Helper()
	jobID, artifactID := uuid.New(), uuid.New()
	price, err := pricingcatalog.MusicCandidateQuote("suno_v6", "generate", false)
	if err != nil {
		t.Fatal(err)
	}
	params := musicgeneration.JobParams{Request: musicgeneration.Request{ModelID: "suno_v6", Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "Synthetic test music"}}, Provider: domain.ProviderAPIMart, ModelCode: "suno-v6", Result: &musicgeneration.StoredResult{Complete: true, Music: domain.MusicResult{Tracks: []domain.MusicTrackResult{{OriginalAudioIndex: 7, AudioID: "private-native-id", Title: "Track", DurationSec: 20}}}, Artifacts: []musicgeneration.StoredArtifact{{ID: artifactID, AudioIndex: 7, Kind: "audio"}}}}
	raw, _ := json.Marshal(params)
	snapshot, _ := json.Marshal(price)
	job := &domain.Job{ID: jobID, AccountID: owner, Source: "web", ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb}, ResultMode: domain.ResultModeAccountHistory, OperationType: domain.OperationAudioMusic, Modality: domain.ModalityAudio, Status: domain.JobStatusSucceeded, Params: raw, PricingSnapshot: snapshot, CostEstimate: price.InternalCredits, OutputArtifactIDs: []uuid.UUID{artifactID}}
	artifact := &domain.Artifact{ID: artifactID, OwnerAccountID: owner, JobID: &jobID, Kind: domain.ArtifactKindOutput, MediaType: domain.MediaTypeAudio, MimeType: "audio/mpeg", Status: domain.ArtifactStatusReady, SizeBytes: 10, StorageBucket: "private", StorageKey: "private", UpdatedAt: time.Now()}
	return job, artifact
}
