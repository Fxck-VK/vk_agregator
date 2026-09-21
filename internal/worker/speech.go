package worker

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/mediaprobe"
	"vk-ai-aggregator/internal/service/providermodels"
	"vk-ai-aggregator/internal/service/speechgeneration"
)

func isSpeechJob(job *domain.Job) bool {
	return job != nil && (job.OperationType == domain.OperationAudioTTS || job.OperationType == domain.OperationAudioSTT)
}

func (p *processor) buildSpeechRequest(ctx context.Context, job *domain.Job, attempt int) (domain.ProviderRequest, error) {
	params, err := speechgeneration.DecodeJob(job)
	if err != nil {
		return domain.ProviderRequest{}, musicInvalidRequestError("invalid speech job")
	}
	if !providermodels.StaticRegistry().MediaCandidateAdmitted(params.ModelID, params.Action()) {
		return domain.ProviderRequest{}, musicBuildError(domain.ProviderErrModelUnavailable, "speech admission pending")
	}
	speech := params.Speech
	if params.Operation() == domain.OperationAudioSTT {
		if len(job.InputArtifactIDs) != 1 || !slices.Contains(job.InputArtifactIDs, params.AudioArtifactID) || p.artifactRepo == nil || p.objects == nil {
			return domain.ProviderRequest{}, musicInvalidRequestError("speech input unavailable")
		}
		a, err := p.artifactRepo.GetByID(ctx, params.AudioArtifactID)
		if err != nil {
			return domain.ProviderRequest{}, err
		}
		if mediaprobe.ValidateMusicInputArtifact(a, job.AccountID) != nil {
			return domain.ProviderRequest{}, musicInvalidRequestError("speech input is not owned or inspected")
		}
		switch a.MimeType {
		case "audio/mpeg":
			speech.FileExtension = "mp3"
		case "audio/wav", "audio/x-wav":
			speech.FileExtension = "wav"
		default:
			return domain.ProviderRequest{}, musicInvalidRequestError("speech input format unavailable")
		}
		speech.FileBytes, err = p.objects.GetObject(ctx, a.StorageBucket, a.StorageKey)
		if err != nil {
			return domain.ProviderRequest{}, err
		}
		mimeType, ok := mediaprobe.SniffMusicInputMIME(speech.FileBytes)
		if !ok || int64(len(speech.FileBytes)) != a.SizeBytes || (speech.FileExtension == "mp3" && mimeType != "audio/mpeg") || (speech.FileExtension == "wav" && mimeType != "audio/wav") || domain.ValidateSpeechRequest(speech, params.Operation(), true) != nil {
			return domain.ProviderRequest{}, musicInvalidRequestError("speech input bytes invalid")
		}
	} else if len(job.InputArtifactIDs) != 0 {
		return domain.ProviderRequest{}, musicInvalidRequestError("unexpected speech input")
	}
	return domain.ProviderRequest{JobID: job.ID, UserID: job.AccountID, Provider: params.Provider, ModelCode: params.ModelCode, Operation: params.Operation(), Modality: params.Modality(), Speech: &speech, Params: domain.DurableProviderTaskRequestJSON(), IdempotencyKey: fmt.Sprintf("speech:%s:%d", job.ID, attempt), AttemptNo: attempt}, nil
}

type speechAudioProber interface {
	ProbeAudio(context.Context, []byte, int64) (domain.ArtifactMediaMetadata, error)
}
type speechArtifactSaver interface {
	SaveBytesArtifactWithMetadataForAccount(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, domain.ArtifactKind, domain.MediaType, string, []byte, domain.ArtifactMediaMetadata) (*domain.Artifact, error)
}

func (p *processor) saveSynchronousResult(ctx context.Context, job *domain.Job, result domain.ProviderTaskResult) error {
	if job.Modality == domain.ModalityText && result.InlineAudio == nil {
		return p.saveOutputs(ctx, job, nil, result.Text)
	}
	if job.OperationType != domain.OperationAudioTTS || job.Modality != domain.ModalityAudio || result.InlineAudio == nil {
		return musicInvalidRequestError("unexpected synchronous result")
	}
	if len(job.OutputArtifactIDs) > 0 {
		return p.saveOutputs(ctx, job, nil, "")
	}
	a := result.InlineAudio
	prober, ok := p.videoProber.(speechAudioProber)
	saver, canSave := p.artifacts.(speechArtifactSaver)
	if !ok || !canSave {
		return musicBuildError(domain.ProviderErrUnsupportedCapab, "speech artifact pipeline unavailable")
	}
	if a.Extension != "wav" || a.MIME != "audio/wav" || int64(len(a.Bytes)) > mediaprobe.MaxMusicInputBytes {
		return musicInvalidRequestError("speech output format unavailable")
	}
	if mime, ok := mediaprobe.SniffMusicInputMIME(a.Bytes); !ok || mime != "audio/wav" {
		return musicInvalidRequestError("speech output bytes invalid")
	}
	metadata, err := prober.ProbeAudio(ctx, a.Bytes, int64(len(a.Bytes)))
	if err != nil {
		return err
	}
	owner, err := workerOutputOwnerID(job)
	if err != nil {
		return err
	}
	art, err := saver.SaveBytesArtifactWithMetadataForAccount(ctx, job.UserID, owner, &job.ID, domain.ArtifactKindOutput, domain.MediaTypeAudio, a.MIME, a.Bytes, metadata)
	if err != nil {
		return err
	}
	job.OutputArtifactIDs = append(job.OutputArtifactIDs, art.ID)
	return p.jobs.Update(ctx, job)
}
