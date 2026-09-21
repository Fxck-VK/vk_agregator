package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/mediaprobe"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/providermodels"
)

var musicCandidateAdmitted = func(modelID string, action domain.MusicAction) bool {
	return providermodels.StaticRegistry().MediaCandidateAdmitted(modelID, string(action))
}

func musicInvalidRequestError(message string) error {
	return musicBuildError(domain.ProviderErrInvalidRequest, message)
}

func musicBuildError(class domain.ProviderErrorClass, message string) error {
	return providerResultError{class: class, message: message}
}

type musicSourceFact struct {
	taskID     string
	audioIndex int
	duration   float64
	action     domain.MusicAction
}

type musicOutputSpec struct {
	kind       string
	format     string
	audioIndex int
	mediaType  domain.MediaType
	url        string
	text       string
}

func (p *processor) buildMusicRequest(ctx context.Context, job *domain.Job, attempt int) (domain.ProviderRequest, error) {
	params, err := musicgeneration.DecodeJob(job)
	if err != nil {
		return domain.ProviderRequest{}, musicInvalidRequestError("invalid music request")
	}
	if !musicCandidateAdmitted(params.ModelID, params.Music.Action) {
		return domain.ProviderRequest{}, musicBuildError(domain.ProviderErrModelUnavailable, "model admission pending")
	}
	musicReq := sanitizeMusicRequestForProvider(params.Music)
	if err := p.applyMusicAudioArtifacts(ctx, job, params, &musicReq); err != nil {
		return domain.ProviderRequest{}, err
	}
	sources, err := p.musicSourceFacts(ctx, job, params)
	if err != nil {
		return domain.ProviderRequest{}, err
	}
	if err := validateMusicSourceKind(params.Music.Action, sources); err != nil {
		return domain.ProviderRequest{}, err
	}
	if err := validateMusicDurationBounds(musicReq, sources); err != nil {
		return domain.ProviderRequest{}, err
	}
	applyMusicSources(&musicReq, sources)

	if params.PersonaJobID != uuid.Nil {
		personaID, err := p.musicPersonaID(ctx, job, params)
		if err != nil {
			return domain.ProviderRequest{}, err
		}
		musicReq.PersonaID = personaID
	}
	if params.CustomModelJobID != uuid.Nil {
		modelID, err := p.musicCustomModelID(ctx, job, params)
		if err != nil {
			return domain.ProviderRequest{}, err
		}
		musicReq.CustomModelID = modelID
	}

	return domain.ProviderRequest{
		JobID:          job.ID,
		UserID:         workerJobOwnerID(job),
		Operation:      domain.OperationAudioMusic,
		Modality:       domain.ModalityAudio,
		Provider:       params.Provider,
		ModelCode:      params.ModelCode,
		Music:          &musicReq,
		Params:         domain.DurableProviderTaskRequestJSON(),
		IdempotencyKey: fmt.Sprintf("provider_submit:%s:%d", job.ID, attempt),
		AttemptNo:      attempt,
	}, nil
}

func (p *processor) applyMusicAudioArtifacts(ctx context.Context, job *domain.Job, params musicgeneration.JobParams, req *domain.MusicRequest) error {
	if len(params.AudioArtifactIDs) == 0 {
		return nil
	}
	if p == nil || p.providerReferences == nil || p.artifactRepo == nil {
		return musicBuildError(domain.ProviderErrUnsupportedCapab, "music audio artifact reference gateway is unavailable")
	}
	urls := make([]string, 0, len(params.AudioArtifactIDs))
	for _, id := range params.AudioArtifactIDs {
		if !slices.Contains(job.InputArtifactIDs, id) {
			return musicInvalidRequestError("music audio artifact is not bound to job input")
		}
		artifact, err := p.artifactRepo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return musicInvalidRequestError("music audio artifact is unavailable")
			}
			return err
		}
		if err := mediaprobe.ValidateMusicInputArtifact(artifact, workerJobOwnerID(job)); err != nil {
			return musicInvalidRequestError("music audio artifact is invalid")
		}
		if err := validateMusicUploadArtifactDuration(params.Music.Action, artifact); err != nil {
			return err
		}
		signedURL, err := p.providerReferences.URL(job.ID, artifact.ID)
		if err != nil {
			return musicBuildError(domain.ProviderErrUnsupportedCapab, "music audio artifact reference gateway is unavailable")
		}
		urls = append(urls, signedURL)
	}
	if params.Music.Action != domain.MusicActionInspo && params.Music.Action != domain.MusicActionCreateModel {
		req.AudioURL = urls[0]
		return nil
	}
	req.AudioURLs = urls
	return nil
}

func validateMusicUploadArtifactDuration(action domain.MusicAction, artifact *domain.Artifact) error {
	if action == domain.MusicActionVoice && artifact.MimeType != "audio/mpeg" && artifact.MimeType != "audio/wav" && artifact.MimeType != "audio/x-wav" {
		return musicInvalidRequestError("voice requires MP3 or WAV input")
	}
	if action != domain.MusicActionUploadCover && action != domain.MusicActionUploadExtend {
		return nil
	}
	if artifact.DurationMS >= int64(mediaprobe.MaxMusicInputDurationSec)*1000 {
		return musicInvalidRequestError("music audio artifact duration exceeds provider limit")
	}
	return nil
}

func (p *processor) musicSourceFacts(ctx context.Context, job *domain.Job, params musicgeneration.JobParams) ([]musicSourceFact, error) {
	if len(params.Sources) == 0 {
		return nil, nil
	}
	if p == nil || p.jobs == nil || p.tasks == nil {
		return nil, musicBuildError(domain.ProviderErrInternal, "music source repositories are unavailable")
	}
	facts := make([]musicSourceFact, 0, len(params.Sources))
	for _, src := range params.Sources {
		sourceJob, err := p.jobs.GetByID(ctx, src.JobID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, musicInvalidRequestError("music source job is unavailable")
			}
			return nil, err
		}
		if workerJobOwnerID(sourceJob) != workerJobOwnerID(job) {
			return nil, musicInvalidRequestError("music source job is not owned by account")
		}
		if !musicSourceStatusComplete(sourceJob.Status) {
			return nil, musicInvalidRequestError("music source job is not complete")
		}
		sourceParams, err := musicgeneration.DecodeJob(sourceJob)
		if err != nil {
			return nil, musicInvalidRequestError("music source job is invalid")
		}
		if sourceParams.Provider != params.Provider || !musicgeneration.SameSourceFamily(params.ModelID, sourceParams.ModelID) {
			return nil, musicInvalidRequestError("music source provider does not match request provider")
		}
		if sourceParams.Result == nil || !sourceParams.Result.Complete {
			return nil, musicInvalidRequestError("music source result is unavailable")
		}
		track, ok := musicTrackByOriginalIndex(sourceParams.Result.Music.Tracks, src.AudioIndex)
		if !ok {
			return nil, musicInvalidRequestError("music source audio index is unavailable")
		}
		latest, err := p.latestTask(ctx, sourceJob.ID)
		if err != nil {
			return nil, err
		}
		if latest == nil || strings.TrimSpace(latest.ExternalID) == "" {
			return nil, musicInvalidRequestError("music source provider task is unavailable")
		}
		if latest.Provider != params.Provider {
			return nil, musicInvalidRequestError("music source provider task does not match request provider")
		}
		facts = append(facts, musicSourceFact{
			taskID:     latest.ExternalID,
			audioIndex: src.AudioIndex,
			duration:   track.DurationSec,
			action:     sourceParams.Music.Action,
		})
	}
	return facts, nil
}

func musicSourceStatusComplete(status domain.JobStatus) bool {
	return status == domain.JobStatusSucceeded
}

func musicTrackByOriginalIndex(tracks []domain.MusicTrackResult, index int) (domain.MusicTrackResult, bool) {
	for _, track := range tracks {
		if track.OriginalAudioIndex == index {
			return track, true
		}
	}
	return domain.MusicTrackResult{}, false
}

func validateMusicSourceKind(action domain.MusicAction, sources []musicSourceFact) error {
	if action != domain.MusicActionAddVocals && action != domain.MusicActionAddInstrumental && action != domain.MusicActionSample {
		return nil
	}
	for _, source := range sources {
		if source.action != domain.MusicActionUpload {
			return musicInvalidRequestError("music action requires uploaded source")
		}
	}
	return nil
}

func validateMusicDurationBounds(req domain.MusicRequest, sources []musicSourceFact) error {
	if len(sources) == 0 {
		return nil
	}
	duration := sources[0].duration
	needsDuration := req.ContinueAtSec != nil || req.StartSec != nil || req.EndSec != nil || req.VocalStartSec != nil || req.VocalEndSec != nil
	if !needsDuration {
		return nil
	}
	if duration <= 0 {
		return musicInvalidRequestError("music source duration is unavailable")
	}
	if req.ContinueAtSec != nil && *req.ContinueAtSec >= duration {
		return musicInvalidRequestError("music source duration bound exceeded")
	}
	if err := validateMusicRange(req.StartSec, req.EndSec, duration); err != nil {
		return err
	}
	if err := validateMusicRange(req.VocalStartSec, req.VocalEndSec, duration); err != nil {
		return err
	}
	return nil
}

func validateMusicRange(start, end *float64, duration float64) error {
	if start != nil && *start >= duration {
		return musicInvalidRequestError("music source duration bound exceeded")
	}
	if end != nil && *end > duration {
		return musicInvalidRequestError("music source duration bound exceeded")
	}
	if start != nil && end != nil && *end <= *start {
		return musicInvalidRequestError("music source duration range is invalid")
	}
	return nil
}

func applyMusicSources(req *domain.MusicRequest, sources []musicSourceFact) {
	req.SourceTaskID = ""
	req.SourceAudioIndex = 0
	req.SourceTaskIDs = nil
	req.SourceAudioIndexes = nil
	switch len(sources) {
	case 0:
		return
	case 1:
		req.SourceTaskID = sources[0].taskID
		req.SourceAudioIndex = sources[0].audioIndex
	default:
		req.SourceTaskIDs = make([]string, 0, len(sources))
		req.SourceAudioIndexes = make([]int, 0, len(sources))
		for _, source := range sources {
			req.SourceTaskIDs = append(req.SourceTaskIDs, source.taskID)
			req.SourceAudioIndexes = append(req.SourceAudioIndexes, source.audioIndex)
		}
	}
}

func (p *processor) musicPersonaID(ctx context.Context, job *domain.Job, params musicgeneration.JobParams) (string, error) {
	sourceParams, err := p.musicTypedResult(ctx, job, params.PersonaJobID, params.Provider, domain.MusicActionPersona)
	if err != nil {
		return "", err
	}
	if sourceParams.Result.Music.Persona == nil || strings.TrimSpace(sourceParams.Result.Music.Persona.ID) == "" {
		return "", musicInvalidRequestError("music persona result is unavailable")
	}
	return sourceParams.Result.Music.Persona.ID, nil
}

func (p *processor) musicCustomModelID(ctx context.Context, job *domain.Job, params musicgeneration.JobParams) (string, error) {
	sourceParams, err := p.musicTypedResult(ctx, job, params.CustomModelJobID, params.Provider, domain.MusicActionCreateModel)
	if err != nil {
		return "", err
	}
	if sourceParams.Result.Music.Model == nil || strings.TrimSpace(sourceParams.Result.Music.Model.ID) == "" {
		return "", musicInvalidRequestError("music custom model result is unavailable")
	}
	return sourceParams.Result.Music.Model.ID, nil
}

func (p *processor) musicTypedResult(ctx context.Context, job *domain.Job, id uuid.UUID, provider domain.ProviderName, action domain.MusicAction) (musicgeneration.JobParams, error) {
	if p == nil || p.jobs == nil {
		return musicgeneration.JobParams{}, musicBuildError(domain.ProviderErrInternal, "music source repository is unavailable")
	}
	sourceJob, err := p.jobs.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source job is unavailable")
		}
		return musicgeneration.JobParams{}, err
	}
	if workerJobOwnerID(sourceJob) != workerJobOwnerID(job) {
		return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source job is not owned by account")
	}
	if !musicSourceStatusComplete(sourceJob.Status) {
		return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source job is not complete")
	}
	sourceParams, err := musicgeneration.DecodeJob(sourceJob)
	if err != nil {
		return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source job is invalid")
	}
	if sourceParams.Provider != provider {
		return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source provider does not match request provider")
	}
	if sourceParams.Music.Action != action {
		return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source has wrong action")
	}
	if sourceParams.Result == nil || !sourceParams.Result.Complete {
		return musicgeneration.JobParams{}, musicInvalidRequestError("music typed source result is unavailable")
	}
	return sourceParams, nil
}

func sanitizeMusicRequestForProvider(req domain.MusicRequest) domain.MusicRequest {
	req.SourceTaskID = ""
	req.SourceAudioIndex = 0
	req.SourceTaskIDs = nil
	req.SourceAudioIndexes = nil
	req.PersonaID = ""
	req.CustomModelID = ""
	req.AudioURL = ""
	req.AudioURLs = nil
	return req
}

func (p *processor) saveMusicOutputs(ctx context.Context, job *domain.Job, res domain.ProviderTaskResult) error {
	if p == nil || p.jobs == nil || p.artifacts == nil {
		return errors.New("worker: music output dependencies are unavailable")
	}
	params, err := musicgeneration.DecodeJob(job)
	if err != nil {
		return err
	}
	// Persisted complete artifacts are authoritative during recovery. Durable
	// provider results intentionally contain no download URLs.
	if params.Result != nil && params.Result.Complete {
		linkStoredMusicArtifacts(job, params.Result.Artifacts)
		return p.persistMusicJobParams(ctx, job, params)
	}
	if res.Music == nil {
		return errors.New("worker: music provider output is unavailable")
	}
	ownerID, err := workerOutputOwnerID(job)
	if err != nil {
		return err
	}

	safeMusic := sanitizeMusicResultForCheckpoint(*res.Music)
	if err := p.hydrateUploadedMusicDuration(ctx, job, params, &safeMusic); err != nil {
		return err
	}
	specs, err := musicOutputSpecs(safeMusic, *res.Music, res.Text)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return errors.New("worker: music provider output is unavailable")
	}
	if params.Result == nil {
		params.Result = &musicgeneration.StoredResult{}
	}
	params.Result.Music = safeMusic
	params.Result.Complete = false
	linkStoredMusicArtifacts(job, params.Result.Artifacts)
	if err := validateMusicCheckpointPrefix(params.Result.Artifacts, specs); err != nil {
		return err
	}
	if len(params.Result.Artifacts) >= len(specs) {
		params.Result.Complete = true
		return p.persistMusicJobParams(ctx, job, params)
	}

	for i := len(params.Result.Artifacts); i < len(specs); i++ {
		spec := specs[i]
		artifact, err := p.saveMusicArtifact(ctx, job, ownerID, spec)
		if err != nil {
			return err
		}
		params.Result.Artifacts = append(params.Result.Artifacts, musicgeneration.StoredArtifact{
			ID:         artifact.ID,
			AudioIndex: spec.audioIndex,
			Kind:       spec.kind,
			Format:     spec.format,
		})
		if !containsID(job.OutputArtifactIDs, artifact.ID) {
			job.OutputArtifactIDs = append(job.OutputArtifactIDs, artifact.ID)
		}
		if err := p.persistMusicJobParams(ctx, job, params); err != nil {
			return err
		}
	}
	params.Result.Complete = true
	return p.persistMusicJobParams(ctx, job, params)
}

func (p *processor) saveMusicArtifact(ctx context.Context, job *domain.Job, ownerID uuid.UUID, spec musicOutputSpec) (*domain.Artifact, error) {
	if spec.url != "" {
		return p.artifacts.SaveRemoteArtifactForAccount(ctx, job.UserID, ownerID, &job.ID, domain.ArtifactKindOutput, spec.mediaType, spec.url)
	}
	return p.artifacts.SaveTextArtifactForAccount(ctx, job.UserID, ownerID, &job.ID, domain.ArtifactKindOutput, spec.text)
}

func (p *processor) persistMusicJobParams(ctx context.Context, job *domain.Job, params musicgeneration.JobParams) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	job.Params = raw
	linkStoredMusicArtifacts(job, params.Result.Artifacts)
	return p.jobs.Update(ctx, job)
}

func (p *processor) hydrateUploadedMusicDuration(ctx context.Context, job *domain.Job, params musicgeneration.JobParams, music *domain.MusicResult) error {
	if music == nil || params.Music.Action != domain.MusicActionUpload || len(music.Tracks) == 0 {
		return nil
	}
	needsDuration := false
	for _, track := range music.Tracks {
		if track.DurationSec <= 0 {
			needsDuration = true
			break
		}
	}
	if !needsDuration {
		return nil
	}
	if len(params.AudioArtifactIDs) != 1 {
		return musicInvalidRequestError("music upload source duration is unavailable")
	}
	id := params.AudioArtifactIDs[0]
	if !slices.Contains(job.InputArtifactIDs, id) {
		return musicInvalidRequestError("music upload source is not bound to job input")
	}
	if p == nil || p.artifactRepo == nil {
		return musicBuildError(domain.ProviderErrInternal, "music upload source duration repository is unavailable")
	}
	artifact, err := p.artifactRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return musicInvalidRequestError("music upload source is unavailable")
		}
		return err
	}
	if err := mediaprobe.ValidateMusicInputArtifact(artifact, workerJobOwnerID(job)); err != nil {
		return musicInvalidRequestError("music upload source is invalid")
	}
	duration := float64(artifact.DurationMS) / 1000
	if duration <= 0 {
		return musicInvalidRequestError("music upload source duration is unavailable")
	}
	for i := range music.Tracks {
		if music.Tracks[i].DurationSec <= 0 {
			music.Tracks[i].DurationSec = duration
		}
	}
	return nil
}

func durableMusicTaskResult(job *domain.Job) (domain.ProviderTaskResult, bool) {
	params, err := musicgeneration.DecodeJob(job)
	if err != nil || params.Result == nil || !params.Result.Complete {
		return domain.ProviderTaskResult{}, false
	}
	music := sanitizeMusicResultForCheckpoint(params.Result.Music)
	return domain.ProviderTaskResult{
		Status: domain.ProviderTaskSucceeded,
		Music:  &music,
	}, true
}

func linkStoredMusicArtifacts(job *domain.Job, artifacts []musicgeneration.StoredArtifact) {
	for _, artifact := range artifacts {
		if artifact.ID != uuid.Nil && !containsID(job.OutputArtifactIDs, artifact.ID) {
			job.OutputArtifactIDs = append(job.OutputArtifactIDs, artifact.ID)
		}
	}
}

func sanitizeMusicResultForCheckpoint(in domain.MusicResult) domain.MusicResult {
	out := in
	out.Tracks = append([]domain.MusicTrackResult(nil), in.Tracks...)
	for i := range out.Tracks {
		out.Tracks[i].Title = sanitizeMusicArtifactText(out.Tracks[i].Title)
		out.Tracks[i].Lyrics = sanitizeMusicArtifactText(out.Tracks[i].Lyrics)
		out.Tracks[i].Tags = sanitizeMusicArtifactText(out.Tracks[i].Tags)
		out.Tracks[i].AudioURL = ""
		out.Tracks[i].ImageURL = ""
		out.Tracks[i].ImageLargeURL = ""
		out.Tracks[i].VideoURL = ""
	}
	out.Lyrics = append([]domain.MusicLyricsResult(nil), in.Lyrics...)
	for i := range out.Lyrics {
		out.Lyrics[i].Title = sanitizeMusicArtifactText(out.Lyrics[i].Title)
		out.Lyrics[i].Text = sanitizeMusicArtifactText(out.Lyrics[i].Text)
		out.Lyrics[i].Tags = sanitizeMusicArtifactText(out.Lyrics[i].Tags)
	}
	out.UpsampledTags = sanitizeMusicArtifactText(out.UpsampledTags)
	if in.Persona != nil {
		v := *in.Persona
		v.Name = sanitizeMusicArtifactText(v.Name)
		v.Description = sanitizeMusicArtifactText(v.Description)
		v.Styles = sanitizeMusicArtifactText(v.Styles)
		out.Persona = &v
	}
	if in.Model != nil {
		v := *in.Model
		v.Name = sanitizeMusicArtifactText(v.Name)
		out.Model = &v
	}
	if in.Voice != nil {
		v := *in.Voice
		v.Name = sanitizeMusicArtifactText(v.Name)
		out.Voice = &v
	}
	out.Artifacts = append([]domain.MusicArtifactResult(nil), in.Artifacts...)
	for i := range out.Artifacts {
		out.Artifacts[i].URL = ""
	}
	if in.MIDI != nil {
		midi := *in.MIDI
		midi.Instruments = sanitizeMusicMIDIRawMessage(midi.Instruments)
		out.MIDI = &midi
	}
	out.Alignment = sanitizeMusicRawMessage(out.Alignment)
	out.Waveform = sanitizeMusicRawMessage(out.Waveform)
	return out
}

func validateMusicCheckpointPrefix(stored []musicgeneration.StoredArtifact, specs []musicOutputSpec) error {
	if len(stored) > len(specs) {
		return musicBuildError(domain.ProviderErrInternal, "music stored artifact checkpoint does not match provider result")
	}
	for i, artifact := range stored {
		spec := specs[i]
		if artifact.AudioIndex != spec.audioIndex || artifact.Kind != spec.kind || strings.TrimSpace(artifact.Format) != strings.TrimSpace(spec.format) {
			return musicBuildError(domain.ProviderErrInternal, "music stored artifact checkpoint does not match provider result")
		}
	}
	return nil
}

func sanitizeMusicRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	value = sanitizeMusicJSONValue(value)
	out, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return out
}

func sanitizeMusicJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, v := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "url") || strings.Contains(lower, "uri") || lower == "id" || strings.HasSuffix(lower, "_id") {
				continue
			}
			out[key] = sanitizeMusicJSONValue(v)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeMusicJSONValue(item))
		}
		return out
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(typed))
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			return ""
		}
		return typed
	default:
		return value
	}
}

func sanitizeMusicMIDIRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	sanitized, ok := sanitizeMusicMIDIValue(value)
	if !ok {
		return nil
	}
	out, err := json.Marshal(sanitized)
	if err != nil {
		return nil
	}
	if string(out) == "null" || string(out) == "[]" || string(out) == "{}" {
		return nil
	}
	return out
}

func sanitizeMusicMIDIValue(value any) (any, bool) {
	switch typed := value.(type) {
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if sanitized, ok := sanitizeMusicMIDIValue(item); ok {
				out = append(out, sanitized)
			}
		}
		return out, len(out) > 0
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			if !musicMIDIKeyAllowed(strings.ToLower(strings.TrimSpace(key))) {
				continue
			}
			if sanitized, ok := sanitizeMusicMIDIValue(item); ok {
				out[key] = sanitized
			}
		}
		return out, len(out) > 0
	case float64, bool:
		return typed, true
	default:
		return nil, false
	}
}

func musicMIDIKeyAllowed(key string) bool {
	switch key {
	case "instrument", "instrument_index", "program", "channel", "track",
		"note", "note_number", "pitch", "velocity",
		"start", "start_time", "end", "end_time", "duration", "time",
		"tick", "ticks", "tempo", "bpm", "notes", "events":
		return true
	default:
		return false
	}
}

func musicOutputSpecs(safeMusic, transient domain.MusicResult, text string) ([]musicOutputSpec, error) {
	var specs []musicOutputSpec
	for _, track := range transient.Tracks {
		audioIndex := track.OriginalAudioIndex
		specs = appendMusicURLSpec(specs, "audio", audioIndex, domain.MediaTypeAudio, track.AudioURL)
		specs = appendMusicURLSpec(specs, "image", audioIndex, domain.MediaTypeImage, track.ImageURL)
		specs = appendMusicURLSpec(specs, "image", audioIndex, domain.MediaTypeImage, track.ImageLargeURL)
		specs = appendMusicURLSpec(specs, "video", audioIndex, domain.MediaTypeVideo, track.VideoURL)
	}
	for _, artifact := range transient.Artifacts {
		kind := normalizedMusicArtifactKind(artifact.Kind, artifact.Format)
		specs = appendMusicURLSpecWithFormat(specs, kind, artifact.OriginalAudioIndex, musicMediaTypeForKind(kind, artifact.Format), artifact.URL, artifact.Format)
	}
	helper, ok, err := musicHelperOutputSpec(safeMusic, text)
	if err != nil {
		return nil, err
	}
	if ok {
		specs = append(specs, helper)
	}
	return specs, nil
}

func appendMusicURLSpec(specs []musicOutputSpec, kind string, audioIndex int, mediaType domain.MediaType, rawURL string) []musicOutputSpec {
	return appendMusicURLSpecWithFormat(specs, kind, audioIndex, mediaType, rawURL, "")
}

func appendMusicURLSpecWithFormat(specs []musicOutputSpec, kind string, audioIndex int, mediaType domain.MediaType, rawURL, format string) []musicOutputSpec {
	if strings.TrimSpace(rawURL) == "" {
		return specs
	}
	if strings.TrimSpace(format) == "" {
		format = musicFormatFromURL(rawURL)
	}
	return append(specs, musicOutputSpec{
		kind:       kind,
		format:     format,
		audioIndex: audioIndex,
		mediaType:  mediaType,
		url:        rawURL,
	})
}

func normalizedMusicArtifactKind(kind, format string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "" {
		return kind
	}
	return string(musicMediaTypeForKind(kind, format))
}

func musicMediaTypeForKind(kind, format string) domain.MediaType {
	value := strings.ToLower(strings.TrimSpace(kind))
	ext := strings.ToLower(strings.Trim(strings.TrimSpace(format), "."))
	switch value {
	case "audio", "vocals", "instrumental", "stem", "stems", "export":
		return domain.MediaTypeAudio
	case "image", "cover", "thumbnail":
		return domain.MediaTypeImage
	case "video":
		return domain.MediaTypeVideo
	case "text", "lyrics", "tags", "bpm", "persona", "model", "voice", "metadata":
		return domain.MediaTypeText
	case "midi", "document":
		return domain.MediaTypeDocument
	}
	switch ext {
	case "mp3", "m4a", "wav", "flac", "ogg":
		return domain.MediaTypeAudio
	case "jpg", "jpeg", "png", "webp", "gif":
		return domain.MediaTypeImage
	case "mp4", "mov", "webm":
		return domain.MediaTypeVideo
	case "mid", "midi", "pdf", "json":
		return domain.MediaTypeDocument
	default:
		return domain.MediaTypeDocument
	}
}

func musicFormatFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(parsed.Path), "."))
	return strings.TrimSpace(ext)
}

func musicHelperOutputSpec(music domain.MusicResult, text string) (musicOutputSpec, bool, error) {
	human := musicHelperHumanText(music, text)
	metadata := musicReadableMetadata(music, true)
	var metadataText string
	if len(metadata) > 0 {
		raw, err := json.Marshal(sanitizeMusicJSONValue(metadata))
		if err != nil {
			return musicOutputSpec{}, false, err
		}
		metadataText = strings.TrimSpace(string(raw))
	}
	if human == "" && metadataText == "" {
		return musicOutputSpec{}, false, nil
	}
	if human == "" {
		return musicOutputSpec{kind: "metadata", format: "json", mediaType: domain.MediaTypeText, text: metadataText}, true, nil
	}
	parts := []string{human}
	if metadataText != "" {
		parts = append(parts, "Metadata:\n"+metadataText)
	}
	return musicOutputSpec{kind: "text", format: "txt", mediaType: domain.MediaTypeText, text: strings.Join(parts, "\n\n")}, true, nil
}

func musicHelperHumanText(music domain.MusicResult, text string) string {
	var parts []string
	if helperText := sanitizeMusicArtifactText(text); helperText != "" {
		parts = append(parts, helperText)
	}
	if helperText := sanitizeMusicArtifactText(musicHumanText(music)); helperText != "" {
		parts = append(parts, helperText)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func sanitizeMusicArtifactText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "http://") ||
			strings.Contains(lower, "https://") ||
			strings.Contains(lower, "audio_id") ||
			strings.Contains(lower, "task_id") ||
			strings.Contains(lower, "source_task") ||
			strings.Contains(lower, "persona_id") ||
			strings.Contains(lower, "custom_model_id") {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func musicHumanText(music domain.MusicResult) string {
	var parts []string
	for _, lyrics := range music.Lyrics {
		var lines []string
		if strings.TrimSpace(lyrics.Title) != "" {
			lines = append(lines, strings.TrimSpace(lyrics.Title))
		}
		if strings.TrimSpace(lyrics.Text) != "" {
			lines = append(lines, strings.TrimSpace(lyrics.Text))
		}
		if strings.TrimSpace(lyrics.Tags) != "" {
			lines = append(lines, strings.TrimSpace(lyrics.Tags))
		}
		if len(lines) > 0 {
			parts = append(parts, strings.Join(lines, "\n"))
		}
	}
	if strings.TrimSpace(music.UpsampledTags) != "" {
		parts = append(parts, strings.TrimSpace(music.UpsampledTags))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func musicReadableMetadata(music domain.MusicResult, includeTracks bool) map[string]any {
	metadata := map[string]any{}
	if includeTracks && len(music.Tracks) > 0 {
		tracks := make([]map[string]any, 0, len(music.Tracks))
		for _, track := range music.Tracks {
			item := map[string]any{}
			if track.DurationSec > 0 {
				item["duration_sec"] = track.DurationSec
			}
			if strings.TrimSpace(track.Status) != "" {
				item["status"] = strings.TrimSpace(track.Status)
			}
			if strings.TrimSpace(track.Title) != "" {
				item["title"] = strings.TrimSpace(track.Title)
			}
			if strings.TrimSpace(track.Lyrics) != "" {
				item["lyrics"] = strings.TrimSpace(track.Lyrics)
			}
			if strings.TrimSpace(track.Tags) != "" {
				item["tags"] = strings.TrimSpace(track.Tags)
			}
			if strings.TrimSpace(track.DisplayTags) != "" {
				item["display_tags"] = strings.TrimSpace(track.DisplayTags)
			}
			if strings.TrimSpace(track.NegativeTags) != "" {
				item["negative_tags"] = strings.TrimSpace(track.NegativeTags)
			}
			if len(item) > 0 {
				if track.OriginalAudioIndex > 0 {
					item["original_audio_index"] = track.OriginalAudioIndex
				}
				tracks = append(tracks, item)
			}
		}
		if len(tracks) > 0 {
			metadata["tracks"] = tracks
		}
	}
	if music.BPM != nil {
		metadata["bpm"] = music.BPM
	}
	if music.MIDI != nil {
		metadata["midi"] = musicReadableMIDI(music.MIDI)
	}
	if music.Persona != nil {
		metadata["persona"] = map[string]string{
			"name":        music.Persona.Name,
			"description": music.Persona.Description,
			"styles":      music.Persona.Styles,
		}
	}
	if music.Model != nil {
		metadata["model"] = map[string]string{"name": music.Model.Name}
	}
	if music.Voice != nil {
		metadata["voice"] = map[string]string{"name": music.Voice.Name}
	}
	if len(music.Alignment) > 0 {
		metadata["alignment"] = music.Alignment
	}
	if len(music.Waveform) > 0 {
		metadata["waveform"] = music.Waveform
	}
	return metadata
}

func musicReadableMIDI(midi *domain.MusicMIDIResult) map[string]any {
	out := map[string]any{}
	if midi == nil {
		return out
	}
	if strings.TrimSpace(midi.State) != "" {
		out["state"] = strings.TrimSpace(midi.State)
	}
	instruments := sanitizeMusicMIDIRawMessage(midi.Instruments)
	if len(instruments) > 0 {
		out["instruments"] = instruments
	}
	return out
}
