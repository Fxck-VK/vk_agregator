package joborchestrator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
)

// The fixed test price exercises ledger/queue boundaries, not APIMart pricing
// or candidate admission. No provider is contacted.
func TestSpeechPreparationOwnershipReservationReplayAndExpiry(t *testing.T) {
	for _, operation := range []domain.OperationType{domain.OperationAudioTTS, domain.OperationAudioSTT} {
		t.Run(string(operation), func(t *testing.T) {
			f := newFixtureWithOrchestratorOptions([]joborchestrator.Option{
				joborchestrator.WithMaxPreparedWebImageJobsPerAccount(3, time.Hour),
			}, billingservice.WithStartingBalance(100))
			ctx, account := context.Background(), uuid.New()
			input := joborchestrator.PrepareAccountJobInput{AccountID: account, Operation: operation,
				Modality: domain.ModalityAudio, IdempotencyKey: uuid.NewString(), CostEstimateCredits: 25}
			if operation == domain.OperationAudioSTT {
				input.Modality = domain.ModalityText
				a := &domain.Artifact{ID: uuid.New(), OwnerAccountID: account, Kind: domain.ArtifactKindInput,
					MediaType: domain.MediaTypeAudio, Status: domain.ArtifactStatusReady,
					StorageBucket: "private-test", StorageKey: "synthetic.wav", SizeBytes: 16044,
					MimeType: "audio/wav", DurationMS: 1000, Codec: "pcm_s16le", Container: "wav", ProbeStatus: domain.MediaProbePassed}
				if err := f.arts.Create(ctx, a); err != nil {
					t.Fatal(err)
				}
				input.InputArtifactIDs = []uuid.UUID{a.ID}
				foreign := input
				foreign.AccountID = uuid.New()
				if _, err := f.orch.PrepareAccountJob(ctx, foreign); !errors.Is(err, joborchestrator.ErrInvalidInputArtifact) {
					t.Fatalf("foreign speech input accepted: %v", err)
				}
			}
			prepared, err := f.orch.PrepareAccountJob(ctx, input)
			if err != nil || prepared.Status != domain.JobStatusPrepared || prepared.ExpiresAt == nil {
				t.Fatalf("prepare: %v", err)
			}
			replay, err := f.orch.PrepareAccountJob(ctx, input)
			if err != nil || replay.ID != prepared.ID {
				t.Fatalf("prepare replay: %v", err)
			}
			if _, err := f.orch.ActivatePreparedAccountJob(ctx, uuid.New(), prepared.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("foreign activation: %v", err)
			}
			if _, err := f.bill.GetReservationByJob(ctx, prepared.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("reserved before confirmation: %v", err)
			}
			f.drain(t)
			if f.pub.Len() != 0 {
				t.Fatal("queued before confirmation")
			}
			for range 2 {
				job, err := f.orch.ActivatePreparedAccountJob(ctx, account, prepared.ID)
				if err != nil || job.Status != domain.JobStatusQueued || job.CostReserved != 25 || job.CostCaptured != 0 {
					t.Fatalf("activation/replay: %v", err)
				}
			}
			f.drain(t)
			if tasks := f.pub.Tasks("queue.audio.generate"); len(tasks) != 1 || tasks[0].JobID != prepared.ID || redisqueue.StreamForOperation(tasks[0].Operation) != redisqueue.StreamAudio {
				t.Fatal("speech must enqueue one audio worker task, including transcription")
			}
			input.IdempotencyKey = uuid.NewString()
			expiring, err := f.orch.PrepareAccountJob(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			past := time.Now().Add(-time.Minute)
			expiring.ExpiresAt = &past
			if err := f.jobs.Update(ctx, expiring); err != nil {
				t.Fatal(err)
			}
			count, _, err := memory.NewPreparedWebImageExpiryRepository(f.jobs).ExpireDuePreparedWebImages(ctx, &account, time.Now(), 10)
			if err != nil || count != 1 {
				t.Fatalf("speech expiry: %d, %v", count, err)
			}
			if replay, err := f.orch.ActivatePreparedAccountJob(ctx, account, expiring.ID); err != nil || replay.Status != domain.JobStatusExpired {
				t.Fatalf("expired speech must replay its terminal state: %v", err)
			}
			if _, err := f.bill.GetReservationByJob(ctx, expiring.ID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("expired speech reserved: %v", err)
			}
		})
	}
}
