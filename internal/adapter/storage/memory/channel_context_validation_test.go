package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

func TestChannelContractsRejectNonPublishableTargetsInMemory(t *testing.T) {
	ctx := context.Background()
	jobs := NewJobRepo()

	legacy := &domain.Job{IdempotencyKey: "legacy-vk-without-target"}
	if err := jobs.Create(ctx, legacy); err != nil {
		t.Fatalf("legacy job without target rejected: %v", err)
	}
	if legacy.ResultMode != domain.ResultModeLegacyUnknown || legacy.DeliveryTarget != nil {
		t.Fatalf("legacy job contract = %q/%+v", legacy.ResultMode, legacy.DeliveryTarget)
	}

	invalidJob := &domain.Job{
		IdempotencyKey: "web-push-target",
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelWeb, RecipientRef: "account"},
	}
	if err := jobs.Create(ctx, invalidJob); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("web external-push job error = %v, want invalid result contract", err)
	}

	deliveries := NewDeliveryRepo()
	invalidDelivery := &domain.Delivery{
		JobID:  uuid.New(),
		Target: &domain.DeliveryTarget{Channel: domain.ChannelVKMiniApp, RecipientRef: "peer:1"},
	}
	if err := deliveries.Create(ctx, invalidDelivery); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("miniapp delivery target error = %v, want invalid result contract", err)
	}
}

func TestChannelContractsRejectInvalidMemoryUpdatesWithoutMutation(t *testing.T) {
	ctx := context.Background()
	jobs := NewJobRepo()
	job := &domain.Job{ID: uuid.New(), IdempotencyKey: "valid-push", ResultMode: domain.ResultModeExternalPush, DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "peer:1"}}
	if err := jobs.Create(ctx, job); err != nil {
		t.Fatalf("create valid job: %v", err)
	}
	changed := *job
	changed.DeliveryTarget = &domain.DeliveryTarget{Channel: domain.ChannelWeb, RecipientRef: "account"}
	if err := jobs.Update(ctx, &changed); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("web target update error = %v, want invalid result contract", err)
	}
	stored, err := jobs.GetByID(ctx, job.ID)
	if err != nil || stored.DeliveryTarget == nil || stored.DeliveryTarget.Channel != domain.ChannelVKBot {
		t.Fatalf("invalid update mutated job = %+v, %v", stored, err)
	}

	deliveries := NewDeliveryRepo()
	delivery := &domain.Delivery{ID: uuid.New(), JobID: job.ID}
	if err := deliveries.Create(ctx, delivery); err != nil {
		t.Fatalf("create legacy delivery: %v", err)
	}
	changedDelivery := *delivery
	changedDelivery.Target = &domain.DeliveryTarget{Channel: domain.ChannelWeb, RecipientRef: "account"}
	if err := deliveries.Update(ctx, &changedDelivery); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("web target delivery update error = %v, want invalid result contract", err)
	}
	storedDelivery, err := deliveries.GetByID(ctx, delivery.ID)
	if err != nil || storedDelivery.Target != nil {
		t.Fatalf("invalid update mutated delivery = %+v, %v", storedDelivery, err)
	}
}
