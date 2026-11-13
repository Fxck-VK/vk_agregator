package worker_test

import (
	"context"
	"testing"

	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/worker"
)

func TestVideoRoundedAspectResultReadinessAndReplay(t *testing.T) {
	for _, rounded := range []bool{false, true} {
		name := "strict"
		if rounded {
			name = "verified rounding"
		}
		t.Run(name, func(t *testing.T) {
			metadata := safeVideoMetadata("h264")
			metadata.Width, metadata.Height = 960, 528
			prober := &fakeVideoProber{metadata: metadata}
			contract := validVideoContract(domain.ProviderMock, "mock-video")
			contract.OutputMayRoundAspect = rounded
			h := newHarnessWithProvider(t, mock.New(), func(d *worker.Deps) {
				d.VideoModel = "mock-video"
				d.VideoDurationSec, d.VideoResolution, d.VideoAspectRatio = 5, "720p", "16:9"
				d.VideoProber, d.RequireVideoProbe = prober, true
				d.VideoTranscodePolicy = "never"
				d.RawVideoDeliveryPolicy = "if_probe_passed"
				d.ProviderMediaContracts = []domain.ProviderMediaContract{contract}
			})
			ctx := context.Background()
			job := h.queueJob(t, domain.OperationVideoGenerate, domain.ModalityVideo, "test clip")
			for attempt := 0; attempt < 2; attempt++ {
				if err := h.gen.Process(ctx, taskFor(job)); err != nil {
					t.Fatalf("process/replay: %v", err)
				}
			}
			got := h.reload(t, job.ID)
			if !rounded {
				if got.Status != domain.JobStatusFailedTerminal || got.ErrorCode != domain.JobErrMediaProviderOutputInvalid {
					t.Fatalf("unverified output not rejected: status=%s error=%s", got.Status, got.ErrorCode)
				}
				if resultReadyEventCount(h.outbox, job.ID) != 0 || len(h.releaser.released) != 1 {
					t.Fatal("failed output must release once and never publish result-ready")
				}
				return
			}
			if got.Status != domain.JobStatusResultReady || len(got.OutputArtifactIDs) != 1 {
				t.Fatalf("verified rounded output not ready: status=%s artifacts=%d", got.Status, len(got.OutputArtifactIDs))
			}
			artifact, err := h.artRepo.GetByID(ctx, got.OutputArtifactIDs[0])
			if err != nil {
				t.Fatal(err)
			}
			if artifact.Status != domain.ArtifactStatusReady || artifact.ProbeStatus != domain.MediaProbePassed || artifact.Width != 960 || artifact.Height != 528 {
				t.Fatal("ready artifact must retain verified actual dimensions")
			}
			if resultReadyEventCount(h.outbox, job.ID) != 1 || len(h.releaser.released) != 0 || prober.calls != 1 {
				t.Fatal("replay duplicated output processing or released a successful reservation")
			}
			tasks, err := h.tasks.ListByJob(ctx, job.ID)
			if err != nil || len(tasks) != 1 {
				t.Fatal("replay duplicated provider task")
			}
		})
	}
}
