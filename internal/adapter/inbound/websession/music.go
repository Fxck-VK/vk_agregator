package websession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/mediaprobe"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/providermodels"
)

type safeMusicJob struct {
	ID           uuid.UUID          `json:"id"`
	Status       domain.JobStatus   `json:"status"`
	ModelID      string             `json:"model_id"`
	Action       domain.MusicAction `json:"action"`
	CostEstimate int64              `json:"cost_estimate"`
	CreatedAt    time.Time          `json:"created_at"`
	ExpiresAt    *time.Time         `json:"expires_at,omitempty"`
}

func musicJobDTO(job *domain.Job, account uuid.UUID) (safeMusicJob, bool) {
	if job == nil || job.AccountID != account || job.Source != "web" || job.UserID != uuid.Nil || job.VKPeerID != 0 || job.ResultMode != domain.ResultModeAccountHistory || job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb || job.ChannelContext.RecipientRef != "" || job.ChannelContext.ThreadRef != "" || job.DeliveryTarget != nil {
		return safeMusicJob{}, false
	}
	p, err := musicgeneration.DecodeJob(job)
	if err != nil {
		return safeMusicJob{}, false
	}
	return safeMusicJob{ID: job.ID, Status: job.Status, ModelID: p.ModelID, Action: p.Music.Action, CostEstimate: job.CostEstimate, CreatedAt: job.CreatedAt, ExpiresAt: job.ExpiresAt}, true
}

func (h *Handler) prepareMusicJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := PrincipalFromContext(r.Context())
	var request musicgeneration.Request
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Validate() != nil {
		writeError(w, 400, "invalid music request")
		return
	}
	key, err := uuid.Parse(r.Header.Get("X-Idempotency-Key"))
	if err != nil || key == uuid.Nil {
		writeError(w, 400, "invalid idempotency key")
		return
	}
	if h.deps.ImageJobs == nil || h.deps.ImageBalance == nil || h.deps.ImageJobIdempotency == nil || h.deps.ImageJobPrepareLimiter == nil {
		writeError(w, 503, "music unavailable")
		return
	}
	if job, err := h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(r.Context(), principal.AccountID, key.String()); err == nil {
		if !musicReplayMatches(job, principal.AccountID, key.String(), request) {
			writeError(w, 409, "music preparation conflict")
			return
		}
		h.writeMusicPreparation(w, r, job, principal.AccountID)
		return
	} else if !errors.Is(err, domain.ErrNotFound) {
		writeError(w, 503, "music unavailable")
		return
	}
	params, price, err := musicgeneration.Resolve(request, providermodels.StaticRegistry())
	if err != nil {
		writeError(w, 503, "music operation pending verification")
		return
	}
	if err := h.validateMusicSources(r.Context(), principal.AccountID, request); err != nil {
		writeError(w, 400, "invalid music source")
		return
	}
	allowed, err := h.deps.ImageJobPrepareLimiter.Allow(r.Context(), "music:"+principal.AccountID.String())
	if err != nil {
		writeError(w, 503, "music unavailable")
		return
	}
	if !allowed {
		writeError(w, 429, "music preparation rate limited")
		return
	}
	raw, err := json.Marshal(params)
	if err != nil {
		writeError(w, 503, "music unavailable")
		return
	}
	job, err := h.deps.ImageJobs.PrepareAccountJob(r.Context(), joborchestrator.PrepareAccountJobInput{AccountID: principal.AccountID, Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio, IdempotencyKey: key.String(), CorrelationID: "web-music:" + key.String(), Params: raw, PricingSnapshot: price, CostEstimateCredits: price.InternalCredits, InputArtifactIDs: request.AudioArtifactIDs})
	if errors.Is(err, domain.ErrConflict) {
		job, err = h.deps.ImageJobIdempotency.GetByIdempotencyKeyForAccount(r.Context(), principal.AccountID, key.String())
		if err != nil || !musicReplayMatches(job, principal.AccountID, key.String(), request) {
			writeError(w, 409, "music preparation conflict")
			return
		}
	}
	if errors.Is(err, domain.ErrPreparedJobLimitExceeded) {
		writeError(w, 429, "music preparation rate limited")
		return
	}
	if err != nil || !musicReplayMatches(job, principal.AccountID, key.String(), request) {
		writeError(w, 503, "music unavailable")
		return
	}
	h.writeMusicPreparation(w, r, job, principal.AccountID)
}

func musicReplayMatches(job *domain.Job, account uuid.UUID, key string, request musicgeneration.Request) bool {
	if _, ok := musicJobDTO(job, account); !ok || job.IdempotencyKey != key {
		return false
	}
	p, err := musicgeneration.DecodeJob(job)
	return err == nil && reflect.DeepEqual(p.Request, request)
}
func (h *Handler) writeMusicPreparation(w http.ResponseWriter, r *http.Request, job *domain.Job, account uuid.UUID) {
	dto, ok := musicJobDTO(job, account)
	if !ok {
		writeError(w, 503, "music unavailable")
		return
	}
	balance, err := h.deps.ImageBalance.BalanceForEstimate(r.Context(), account)
	if err != nil {
		writeError(w, 503, "music unavailable")
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		Job       safeMusicJob `json:"job"`
		Balance   int64        `json:"balance"`
		CanAfford bool         `json:"can_afford"`
	}{dto, balance, balance >= dto.CostEstimate})
}

func (h *Handler) validateMusicSources(ctx context.Context, account uuid.UUID, request musicgeneration.Request) error {
	for _, id := range request.AudioArtifactIDs {
		if h.deps.ImageArtifacts == nil {
			return domain.ErrNotFound
		}
		a, err := h.deps.ImageArtifacts.GetByIDForAccount(ctx, account, id)
		if err != nil || mediaprobe.ValidateMusicInputArtifact(a, account) != nil {
			return domain.ErrNotFound
		}
		if (request.Music.Action == domain.MusicActionUploadCover || request.Music.Action == domain.MusicActionUploadExtend) && a.DurationMS >= 480000 {
			return domain.ErrNotFound
		}
		if request.Music.Action == domain.MusicActionVoice && a.MimeType != "audio/mpeg" && a.MimeType != "audio/wav" && a.MimeType != "audio/x-wav" {
			return domain.ErrNotFound
		}
	}
	ids := []uuid.UUID{}
	for _, s := range request.Sources {
		ids = append(ids, s.JobID)
	}
	for _, id := range []uuid.UUID{request.PersonaJobID, request.CustomModelJobID} {
		if id != uuid.Nil {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 && (h.deps.ImageJobReader == nil || h.deps.ImageResults == nil) {
		return domain.ErrNotFound
	}
	for _, id := range ids {
		job, err := h.deps.ImageJobReader.GetByIDForAccount(ctx, account, id)
		if err != nil || job == nil || job.AccountID != account || job.Status != domain.JobStatusSucceeded {
			return domain.ErrNotFound
		}
		p, err := musicgeneration.DecodeJob(job)
		if err != nil || p.Result == nil || !p.Result.Complete {
			return domain.ErrNotFound
		}
		if _, err := h.deps.ImageResults.GetResult(ctx, account, id); err != nil {
			return domain.ErrNotFound
		}
		if id == request.PersonaJobID && (p.Result.Music.Persona == nil || p.Result.Music.Persona.ID == "") {
			return domain.ErrNotFound
		}
		if id == request.CustomModelJobID && (p.Result.Music.Model == nil || p.Result.Music.Model.ID == "") {
			return domain.ErrNotFound
		}
		for _, s := range request.Sources {
			if s.JobID != id {
				continue
			}
			if (request.Music.Action == domain.MusicActionSample || request.Music.Action == domain.MusicActionAddVocals || request.Music.Action == domain.MusicActionAddInstrumental) && p.Music.Action != domain.MusicActionUpload {
				return domain.ErrNotFound
			}
			found := false
			for _, track := range p.Result.Music.Tracks {
				if track.OriginalAudioIndex == s.AudioIndex {
					found = true
				}
			}
			if !found {
				return domain.ErrNotFound
			}
		}
	}
	return nil
}

func (h *Handler) loadMusicJob(w http.ResponseWriter, r *http.Request) (*domain.Job, uuid.UUID, bool) {
	w.Header().Set("Cache-Control", "no-store")
	p, _ := PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("jobID"))
	if err != nil || id == uuid.Nil {
		writeError(w, 400, "invalid music job id")
		return nil, p.AccountID, false
	}
	if h.deps.ImageJobReader == nil {
		writeError(w, 503, "music unavailable")
		return nil, p.AccountID, false
	}
	if h.deps.ImageJobExpiry != nil {
		if _, err := h.deps.ImageJobExpiry.ReconcileJob(r.Context(), p.AccountID, id); err != nil {
			writeError(w, 503, "music unavailable")
			return nil, p.AccountID, false
		}
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), p.AccountID, id)
	if err != nil {
		writeError(w, 404, "music job not found")
		return nil, p.AccountID, false
	}
	if _, ok := musicJobDTO(job, p.AccountID); !ok {
		writeError(w, 404, "music job not found")
		return nil, p.AccountID, false
	}
	return job, p.AccountID, true
}
func (h *Handler) getMusicJob(w http.ResponseWriter, r *http.Request) {
	job, account, ok := h.loadMusicJob(w, r)
	if !ok {
		return
	}
	dto, _ := musicJobDTO(job, account)
	writeJSON(w, 200, struct {
		Job safeMusicJob `json:"job"`
	}{dto})
}
func (h *Handler) activateMusicJob(w http.ResponseWriter, r *http.Request) {
	job, account, ok := h.loadMusicJob(w, r)
	if !ok {
		return
	}
	p, _ := musicgeneration.DecodeJob(job)
	if h.deps.ImageJobs == nil || !providermodels.StaticRegistry().MediaCandidateAdmitted(p.ModelID, string(p.Music.Action)) {
		writeError(w, 503, "music operation pending verification")
		return
	}
	if err := h.validateMusicSources(r.Context(), account, p.Request); err != nil {
		writeError(w, 400, "invalid music source")
		return
	}
	job, err := h.deps.ImageJobs.ActivatePreparedAccountJob(r.Context(), account, job.ID)
	if errors.Is(err, domain.ErrInsufficientCredits) {
		writeError(w, 402, "insufficient credits")
		return
	}
	if errors.Is(err, domain.ErrConflict) {
		writeError(w, 409, "music activation conflict")
		return
	}
	if err != nil {
		writeError(w, 503, "music unavailable")
		return
	}
	dto, ok := musicJobDTO(job, account)
	if !ok {
		writeError(w, 503, "music unavailable")
		return
	}
	writeJSON(w, 200, struct {
		Job safeMusicJob `json:"job"`
	}{dto})
}
func (h *Handler) listMusicJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	p, _ := PrincipalFromContext(r.Context())
	if h.deps.ImageJobHistory == nil {
		writeError(w, 503, "music unavailable")
		return
	}
	if h.deps.ImageJobExpiry != nil {
		if _, err := h.deps.ImageJobExpiry.ReconcileAccount(r.Context(), p.AccountID, 10); err != nil {
			writeError(w, 503, "music unavailable")
			return
		}
	}
	v, present := r.URL.Query()["limit"]
	limit, err := imageJobHistoryLimit(v, present)
	if err != nil {
		writeError(w, 400, "invalid limit")
		return
	}
	v, present = r.URL.Query()["cursor"]
	after, err := imageJobHistoryCursor(v, present)
	if err != nil {
		writeError(w, 400, "invalid cursor")
		return
	}
	jobs, err := h.deps.ImageJobHistory.ListCursor(r.Context(), domain.JobFilter{AccountID: &p.AccountID, Source: "web", Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio}, limit+1, after)
	if err != nil {
		writeError(w, 503, "music unavailable")
		return
	}
	hasMore := len(jobs) > limit
	if hasMore {
		jobs = jobs[:limit]
	}
	items := []safeMusicJob{}
	for _, job := range jobs {
		dto, ok := musicJobDTO(job, p.AccountID)
		if !ok {
			writeError(w, 503, "music unavailable")
			return
		}
		items = append(items, dto)
	}
	var cursor *string
	if hasMore {
		v, ok := encodeImageJobHistoryCursor(domain.JobCursor{CreatedAt: jobs[len(jobs)-1].CreatedAt, ID: jobs[len(jobs)-1].ID})
		if !ok {
			writeError(w, 503, "music unavailable")
			return
		}
		cursor = &v
	}
	writeJSON(w, 200, struct {
		Items      []safeMusicJob `json:"items"`
		HasMore    bool           `json:"has_more"`
		NextCursor *string        `json:"next_cursor"`
	}{items, hasMore, cursor})
}

type safeMusicTrack struct {
	JobID       uuid.UUID `json:"job_id"`
	AudioIndex  int       `json:"audio_index"`
	Title       string    `json:"title"`
	DurationSec float64   `json:"duration_sec"`
	AudioURL    string    `json:"audio_url,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	VideoURL    string    `json:"video_url,omitempty"`
}
type safeMusicArtifact struct {
	ID     uuid.UUID `json:"id"`
	Kind   string    `json:"kind"`
	Format string    `json:"format,omitempty"`
	URL    string    `json:"url"`
}

// Assets are referenced by owned application jobs, never native provider IDs.
type safeMusicAsset struct {
	JobID uuid.UUID `json:"job_id"`
	Name  string    `json:"name"`
}

func (h *Handler) getMusicJobResult(w http.ResponseWriter, r *http.Request) {
	job, account, ok := h.loadMusicJob(w, r)
	if !ok {
		return
	}
	p, _ := musicgeneration.DecodeJob(job)
	if h.deps.ImageResults == nil {
		writeError(w, 503, "music unavailable")
		return
	}
	result, err := h.deps.ImageResults.GetResult(r.Context(), account, job.ID)
	if err != nil || result.ID != job.ID || p.Result == nil || !p.Result.Complete {
		writeError(w, 404, "music result not found")
		return
	}
	allowed := map[uuid.UUID]bool{}
	for _, a := range result.Artifacts {
		allowed[a.ID] = true
	}
	tracks := []safeMusicTrack{}
	artifacts := []safeMusicArtifact{}
	for _, a := range p.Result.Artifacts {
		if !allowed[a.ID] {
			writeError(w, 404, "music result not found")
			return
		}
		artifacts = append(artifacts, safeMusicArtifact{ID: a.ID, Kind: a.Kind, Format: a.Format, URL: "/web/v1/music-artifacts/" + a.ID.String()})
	}
	for _, t := range p.Result.Music.Tracks {
		track := safeMusicTrack{JobID: job.ID, AudioIndex: t.OriginalAudioIndex, Title: t.Title, DurationSec: t.DurationSec}
		for _, a := range p.Result.Artifacts {
			if a.AudioIndex != t.OriginalAudioIndex {
				continue
			}
			url := "/web/v1/music-artifacts/" + a.ID.String()
			switch a.Kind {
			case "audio":
				track.AudioURL = url
			case "image":
				track.ImageURL = url
			case "video":
				track.VideoURL = url
			}
		}
		tracks = append(tracks, track)
	}
	var persona, customModel, voice *safeMusicAsset
	if p.Result.Music.Persona != nil && p.Result.Music.Persona.ID != "" {
		persona = &safeMusicAsset{JobID: job.ID, Name: musicAssetName(p.Result.Music.Persona.Name, "Моя персона")}
	}
	if p.Result.Music.Model != nil && p.Result.Music.Model.ID != "" {
		customModel = &safeMusicAsset{JobID: job.ID, Name: musicAssetName(p.Result.Music.Model.Name, "Моя модель")}
	}
	if p.Result.Music.Voice != nil && p.Result.Music.Voice.ID != "" {
		voice = &safeMusicAsset{JobID: job.ID, Name: musicAssetName(p.Result.Music.Voice.Name, "Мой голос")}
	}
	writeJSON(w, 200, struct {
		JobID     uuid.UUID                  `json:"job_id"`
		Tracks    []safeMusicTrack           `json:"tracks"`
		Artifacts []safeMusicArtifact        `json:"artifacts"`
		Lyrics    []domain.MusicLyricsResult `json:"lyrics,omitempty"`
		Tags      string                     `json:"tags,omitempty"`
		BPM       *domain.MusicBPMResult     `json:"bpm,omitempty"`
		Persona   *safeMusicAsset            `json:"persona,omitempty"`
		Model     *safeMusicAsset            `json:"model,omitempty"`
		Voice     *safeMusicAsset            `json:"voice,omitempty"`
	}{job.ID, tracks, artifacts, p.Result.Music.Lyrics, p.Result.Music.UpsampledTags, p.Result.Music.BPM, persona, customModel, voice})
}

func musicAssetName(name, fallback string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return fallback
}

func (h *Handler) getMusicArtifact(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p, _ := PrincipalFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("artifactID"))
	if err != nil {
		writeError(w, 404, "music artifact not found")
		return
	}
	reader, ok := h.deps.ImageArtifactURLSigner.(ImageArtifactObjectReader)
	if !ok || h.deps.ImageArtifacts == nil || h.deps.ImageJobReader == nil || h.deps.ImageResults == nil {
		writeError(w, 503, "music unavailable")
		return
	}
	a, err := h.deps.ImageArtifacts.GetByIDForAccount(r.Context(), p.AccountID, id)
	if err != nil || a == nil || a.OwnerAccountID != p.AccountID || a.JobID == nil || a.Status != domain.ArtifactStatusReady || a.Kind != domain.ArtifactKindOutput || a.SizeBytes <= 0 || a.SizeBytes > 128<<20 {
		writeError(w, 404, "music artifact not found")
		return
	}
	job, err := h.deps.ImageJobReader.GetByIDForAccount(r.Context(), p.AccountID, *a.JobID)
	if err != nil {
		writeError(w, 404, "music artifact not found")
		return
	}
	if _, ok := musicJobDTO(job, p.AccountID); !ok || !slices.Contains(job.OutputArtifactIDs, id) {
		writeError(w, 404, "music artifact not found")
		return
	}
	result, err := h.deps.ImageResults.GetResult(r.Context(), p.AccountID, job.ID)
	if err != nil || result.ID != job.ID {
		writeError(w, 404, "music artifact not found")
		return
	}
	allowed := false
	for _, output := range result.Artifacts {
		if output.ID == id {
			allowed = true
		}
	}
	if !allowed {
		writeError(w, 404, "music artifact not found")
		return
	}
	data, err := reader.GetObject(r.Context(), a.StorageBucket, a.StorageKey)
	if err != nil || int64(len(data)) != a.SizeBytes {
		writeError(w, 503, "music unavailable")
		return
	}
	// No redirect to private storage/provider URLs, and no executable document
	// rendering. Same-origin byte serving supports the player's Range requests.
	if strings.HasPrefix(a.MimeType, "audio/") || a.MimeType == "video/mp4" || a.MimeType == "image/png" || a.MimeType == "image/jpeg" || strings.HasPrefix(a.MimeType, "text/plain") {
		w.Header().Set("Content-Type", a.MimeType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeContent(w, r, id.String(), a.UpdatedAt, bytes.NewReader(data))
}
