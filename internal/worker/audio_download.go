package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/queue"
	"vk-ai-aggregator/internal/service/artifactservice"
	"vk-ai-aggregator/internal/service/audiodownload"
	"vk-ai-aggregator/internal/service/mediaprobe"
)

type audioDownloader interface {
	Download(context.Context, string) ([]byte, error)
}

type audioArtifactSaver interface {
	SaveBytesArtifactForAccount(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID, domain.ArtifactKind, domain.MediaType, string, []byte) (*domain.Artifact, error)
}

// AudioDownloadWorker runs link extraction outside the provider worker pool.
type AudioDownloadWorker struct {
	processor
	downloader audioDownloader
	outputs    audioArtifactSaver
	findOutput func(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error)
	leases     redis.Cmdable
}

func NewAudioDownloadWorker(
	deps Deps, downloader audioDownloader, outputs audioArtifactSaver,
	findOutput func(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error), leases redis.Cmdable,
) *AudioDownloadWorker {
	return &AudioDownloadWorker{
		processor: newProcessor(deps), downloader: downloader, outputs: outputs, findOutput: findOutput, leases: leases,
	}
}

const releaseAudioDownloadLease = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`

func (w *AudioDownloadWorker) Process(ctx context.Context, task queue.Task) error {
	if w.leases == nil {
		return errors.New("worker: audio download leases unavailable")
	}
	// Redis Streams may redeliver a long-running entry to another worker.
	// Use a unique lease token, a shorter work deadline, and checked release.
	key := "lock:jobs:audio_download:" + task.JobID.String()
	token := uuid.NewString()
	claimed, err := w.leases.SetNX(ctx, key, token, 5*time.Minute).Result()
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("worker: audio download is already leased")
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		_ = w.leases.Eval(releaseCtx, releaseAudioDownloadLease, []string{key}, token).Err()
	}()
	workCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	return w.process(workCtx, task.JobID)
}

func (w *AudioDownloadWorker) process(ctx context.Context, jobID uuid.UUID) error {
	job, err := w.jobs.GetByID(ctx, jobID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if job.OperationType != domain.OperationAudioDownload {
		return nil
	}
	switch job.Status {
	case domain.JobStatusQueued:
		if err := w.setStatus(ctx, job, domain.JobStatusPostprocessing, "", ""); err != nil {
			return err
		}
	case domain.JobStatusPostprocessing:
	default:
		return nil
	}

	if job.AccountID == uuid.Nil || job.Source != "web" || job.UserID != uuid.Nil ||
		job.VKPeerID != 0 || job.CommandID != uuid.Nil || job.Modality != domain.ModalityAudio ||
		job.ResultMode != domain.ResultModeAccountHistory || job.DeliveryTarget != nil ||
		job.ChannelContext == nil || job.ChannelContext.Channel != domain.ChannelWeb ||
		job.ChannelContext.RecipientRef != "" || job.ChannelContext.ThreadRef != "" ||
		job.CostEstimate != 0 || job.CostReserved != 0 || job.CostCaptured != 0 {
		return w.fail(ctx, job, "invalid_request")
	}

	if len(job.OutputArtifactIDs) == 0 {
		if w.findOutput == nil || w.downloader == nil || w.outputs == nil {
			return w.fail(ctx, job, "download_unavailable")
		}
		// The artifact may have persisted before a crash or a failed job update.
		artifactID, err := w.findOutput(ctx, job.AccountID, job.ID)
		if errors.Is(err, domain.ErrNotFound) {
			artifactID, err = w.saveOutput(ctx, job)
		}
		if err != nil || artifactID == uuid.Nil {
			return err
		}
		job.OutputArtifactIDs = []uuid.UUID{artifactID}
		if err := w.jobs.Update(ctx, job); err != nil {
			return err
		}
	}

	// The artifact scanner inspected the bytes before storage. Record the
	// normal output verdict as well before publishing the result to the site.
	blocked, _, err := w.moderateOutputs(ctx, job, "MP3 audio")
	if err != nil || blocked {
		return err
	}
	return w.markResultReady(ctx, job)
}

func (w *AudioDownloadWorker) saveOutput(ctx context.Context, job *domain.Job) (uuid.UUID, error) {
	var params struct {
		SourceURL string `json:"source_url"`
	}
	if json.Unmarshal(job.Params, &params) != nil || params.SourceURL == "" {
		return uuid.Nil, w.fail(ctx, job, "invalid_request")
	}
	data, err := w.downloader.Download(ctx, params.SourceURL)
	if ctx.Err() != nil {
		return uuid.Nil, ctx.Err()
	}
	switch {
	case errors.Is(err, audiodownload.ErrUnavailable):
		return uuid.Nil, w.fail(ctx, job, "download_unavailable")
	case errors.Is(err, audiodownload.ErrTooLarge):
		return uuid.Nil, w.fail(ctx, job, "download_too_large")
	case errors.Is(err, audiodownload.ErrTimeout):
		return uuid.Nil, w.fail(ctx, job, "download_timeout")
	case err != nil:
		return uuid.Nil, w.fail(ctx, job, "download_failed")
	}
	const mp3 = "audio/mpeg"
	if detected, ok := mediaprobe.SniffMusicInputMIME(data); !ok || detected != mp3 {
		return uuid.Nil, w.fail(ctx, job, "download_failed")
	}
	artifact, err := w.outputs.SaveBytesArtifactForAccount(ctx, uuid.Nil, job.AccountID, &job.ID,
		domain.ArtifactKindOutput, domain.MediaTypeAudio, mp3, data)
	if err != nil {
		if ctx.Err() != nil {
			return uuid.Nil, ctx.Err()
		}
		var scanErr artifactservice.ContentScanError
		if errors.As(err, &scanErr) {
			if classOf(scanErr.Unwrap()) == domain.ProviderErrContentRejected {
				return uuid.Nil, w.fail(ctx, job, "content_rejected")
			}
			return uuid.Nil, w.fail(ctx, job, "download_unavailable")
		}
		return uuid.Nil, err
	}
	return artifact.ID, nil
}

func (w *AudioDownloadWorker) fail(ctx context.Context, job *domain.Job, code string) error {
	status := domain.JobStatusFailedTerminal
	if code == "content_rejected" {
		status = domain.JobStatusRejected
	}
	if err := w.setStatus(ctx, job, status, code, "could not prepare MP3"); err != nil {
		return err
	}
	metrics.JobsTerminal.WithLabelValues(string(status)).Inc()
	return nil
}
