package joborchestrator_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/musicgeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// This exercises the local job/ledger/outbox contract with explicit test
// dependencies. It does not admit a candidate or contact a provider.
func TestMusicPreparationReservesOnlyOnOwnedActivationAndQueuesOnce(t *testing.T) {
	f := newFixture(billingservice.WithStartingBalance(100))
	ctx := context.Background()
	account := uuid.New()
	quote, err := pricingcatalog.MusicCandidateQuote("suno_v6", "generate", false)
	if err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(musicgeneration.JobParams{
		Request:  musicgeneration.Request{ModelID: "suno_v6", Music: domain.MusicRequest{Action: domain.MusicActionGenerate, Prompt: "synthetic music"}},
		Provider: domain.ProviderAPIMart, ModelCode: "suno-v6",
	})
	if err != nil {
		t.Fatal(err)
	}
	input := joborchestrator.PrepareAccountJobInput{
		AccountID: account, Operation: domain.OperationAudioMusic, Modality: domain.ModalityAudio,
		IdempotencyKey: uuid.NewString(), Params: params, PricingSnapshot: quote,
	}
	prepared, err := f.orch.PrepareAccountJob(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Status != domain.JobStatusPrepared || prepared.CostEstimate != quote.InternalCredits {
		t.Fatal("music preparation did not preserve the server quote")
	}
	replay, err := f.orch.PrepareAccountJob(ctx, input)
	if err != nil || replay.ID != prepared.ID {
		t.Fatalf("preparation replay: %v", err)
	}
	if _, err := f.orch.ActivatePreparedAccountJob(ctx, uuid.New(), prepared.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign activation: %v", err)
	}
	if _, err := f.bill.GetReservationByJob(ctx, prepared.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("credits reserved before owned confirmation: %v", err)
	}
	f.drain(t)
	if f.pub.Len() != 0 {
		t.Fatal("unconfirmed music reached the worker queue")
	}
	for range 2 {
		job, err := f.orch.ActivatePreparedAccountJob(ctx, account, prepared.ID)
		if err != nil || job.Status != domain.JobStatusQueued || job.CostReserved != quote.InternalCredits || job.CostCaptured != 0 {
			t.Fatalf("owned activation failed or billed incorrectly: %v", err)
		}
	}
	reservation, err := f.bill.GetReservationByJob(ctx, prepared.ID)
	if err != nil || reservation.OwnerAccountID != account || reservation.Amount != quote.InternalCredits {
		t.Fatalf("music reservation: %v", err)
	}
	f.drain(t)
	if tasks := f.pub.Tasks("queue.audio.generate"); len(tasks) != 1 || tasks[0].JobID != prepared.ID || redisqueue.StreamForOperation(tasks[0].Operation) != redisqueue.StreamAudio {
		t.Fatal("music activation did not produce exactly one audio task")
	}
}
