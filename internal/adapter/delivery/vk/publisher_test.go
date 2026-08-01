package vkdelivery

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
)

func TestPublisherBuildDeliveryUsesAuthoritativeTargetAndDeterministicRandomID(t *testing.T) {
	ctx := context.Background()
	deliveries := memory.NewDeliveryRepo()
	artifacts := memory.NewArtifactRepo()
	objects := memory.NewObjectStore()
	client := NewMockClient()
	publisher := NewPublisher(PublisherDeps{
		Deliveries: deliveries,
		Artifacts:  artifacts,
		Objects:    objects,
		Client:     client,
	})
	accountID := uuid.New()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		UserID:         accountID,
		VKPeerID:       999,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusResultReady,
	}
	artifact := &domain.Artifact{
		ID:             uuid.New(),
		OwnerUserID:    accountID,
		OwnerAccountID: accountID,
		JobID:          &job.ID,
		Kind:           domain.ArtifactKindOutput,
		MediaType:      domain.MediaTypeText,
		Status:         domain.ArtifactStatusReady,
	}
	if err := artifacts.Create(ctx, artifact); err != nil {
		t.Fatalf("create output artifact: %v", err)
	}
	job.OutputArtifactIDs = []uuid.UUID{artifact.ID}
	key := "delivery:" + job.ID.String()

	delivery, err := publisher.BuildDelivery(ctx, job, key)
	if err != nil {
		t.Fatalf("build delivery: %v", err)
	}
	if delivery.VKPeerID != 555 ||
		delivery.Target == nil ||
		delivery.Target.RecipientRef != "555" ||
		delivery.VKRandomID != DeterministicRandomID(key) {
		t.Fatalf("delivery did not use authoritative target: %+v", delivery)
	}
}

func TestPublisherBuildDeliveryRejectsSuccessfulExternalResultWithoutOutputs(t *testing.T) {
	publisher := NewPublisher(PublisherDeps{Deliveries: memory.NewDeliveryRepo()})
	accountID := uuid.New()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		UserID:         accountID,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusResultReady,
	}

	if _, err := publisher.BuildDelivery(context.Background(), job, "delivery:"+job.ID.String()); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("build error = %v, want invalid result contract", err)
	}
}

func TestPublisherBuildDeliveryRejectsSuccessfulReplayWithoutOutputsBeforeMutation(t *testing.T) {
	ctx := context.Background()
	deliveries := memory.NewDeliveryRepo()
	client := NewMockClient()
	publisher := NewPublisher(PublisherDeps{Deliveries: deliveries, Client: client})
	accountID := uuid.New()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		UserID:         accountID,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusDelivering,
	}
	key := "delivery:" + job.ID.String()
	existing := &domain.Delivery{
		JobID:          job.ID,
		UserID:         accountID,
		VKPeerID:       555,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
		Text:           "must not send",
	}
	if err := deliveries.Create(ctx, existing); err != nil {
		t.Fatalf("create legacy delivery replay: %v", err)
	}

	if _, err := publisher.BuildDelivery(ctx, job, key); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("build error = %v, want invalid result contract", err)
	}
	stored, err := deliveries.GetByID(ctx, existing.ID)
	if err != nil {
		t.Fatalf("reload delivery: %v", err)
	}
	if stored.AccountID != uuid.Nil ||
		stored.Target != nil ||
		stored.Status != domain.DeliveryStatusPending ||
		stored.AttemptNo != 1 ||
		len(client.Sent()) != 0 {
		t.Fatalf("no-output replay mutated delivery: delivery=%+v sends=%+v", stored, client.Sent())
	}
}

func TestPublisherReplayBackfillsMatchingLegacyTargetBeforePublish(t *testing.T) {
	ctx := context.Background()
	deliveries := memory.NewDeliveryRepo()
	client := NewMockClient()
	publisher := NewPublisher(PublisherDeps{Deliveries: deliveries, Client: client})
	accountID := uuid.New()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		UserID:         accountID,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusDelivering,
	}
	key := "delivery:" + job.ID.String()
	delivery := &domain.Delivery{
		JobID:          job.ID,
		UserID:         accountID,
		VKPeerID:       555,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
		Text:           "ready",
	}
	if err := deliveries.Create(ctx, delivery); err != nil {
		t.Fatalf("create legacy delivery: %v", err)
	}

	if err := publisher.Publish(ctx, job, delivery); err != nil {
		t.Fatalf("publish: %v", err)
	}
	stored, err := deliveries.GetByID(ctx, delivery.ID)
	if err != nil {
		t.Fatalf("reload delivery: %v", err)
	}
	if stored.Target == nil ||
		stored.Target.RecipientRef != "555" ||
		stored.Status != domain.DeliveryStatusSent ||
		len(client.Sent()) != 1 {
		t.Fatalf("legacy replay was not safely backfilled and sent: delivery=%+v sends=%+v", stored, client.Sent())
	}
}

func TestPublisherReplayTargetMismatchFailsClosed(t *testing.T) {
	ctx := context.Background()
	deliveries := memory.NewDeliveryRepo()
	client := NewMockClient()
	publisher := NewPublisher(PublisherDeps{Deliveries: deliveries, Client: client})
	accountID := uuid.New()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      accountID,
		UserID:         accountID,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusDelivering,
	}
	key := "delivery:" + job.ID.String()
	delivery := &domain.Delivery{
		JobID:          job.ID,
		UserID:         accountID,
		AccountID:      accountID,
		VKPeerID:       556,
		Target:         &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "556"},
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
		Text:           "must not send",
	}
	if err := deliveries.Create(ctx, delivery); err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	err := publisher.Publish(ctx, job, delivery)
	if !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("publish error = %v, want invalid result contract", err)
	}
	stored, _ := deliveries.GetByID(ctx, delivery.ID)
	if len(client.Sent()) != 0 || stored.Status != domain.DeliveryStatusPending {
		t.Fatalf("mismatched replay mutated publication: delivery=%+v sends=%+v", stored, client.Sent())
	}
}

func TestPublisherReplayOwnershipMismatchDoesNotBackfill(t *testing.T) {
	ctx := context.Background()
	deliveries := memory.NewDeliveryRepo()
	client := NewMockClient()
	publisher := NewPublisher(PublisherDeps{Deliveries: deliveries, Client: client})
	jobAccountID := uuid.New()
	persistedAccountID := uuid.New()
	job := &domain.Job{
		ID:             uuid.New(),
		AccountID:      jobAccountID,
		UserID:         jobAccountID,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "555"},
		Modality:       domain.ModalityText,
		Status:         domain.JobStatusResultReady,
	}
	key := "delivery:" + job.ID.String()
	delivery := &domain.Delivery{
		JobID:          job.ID,
		UserID:         jobAccountID,
		AccountID:      persistedAccountID,
		VKPeerID:       555,
		Type:           domain.DeliveryTypeMessage,
		Status:         domain.DeliveryStatusPending,
		VKRandomID:     DeterministicRandomID(key),
		IdempotencyKey: key,
		AttemptNo:      1,
		Text:           "must not send",
	}
	if err := deliveries.Create(ctx, delivery); err != nil {
		t.Fatalf("create legacy delivery: %v", err)
	}

	if _, err := publisher.BuildDelivery(ctx, job, key); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("build error = %v, want invalid result contract", err)
	}
	stored, err := deliveries.GetByID(ctx, delivery.ID)
	if err != nil {
		t.Fatalf("reload delivery: %v", err)
	}
	if stored.AccountID != persistedAccountID || stored.Target != nil || len(client.Sent()) != 0 {
		t.Fatalf("ownership mismatch mutated replay: delivery=%+v sends=%+v", stored, client.Sent())
	}
}

func TestPublisherRejectsMalformedTargetWithoutLegacyPeerFallback(t *testing.T) {
	publisher := NewPublisher(PublisherDeps{})
	job := &domain.Job{
		ID:             uuid.New(),
		VKPeerID:       555,
		ResultMode:     domain.ResultModeExternalPush,
		DeliveryTarget: &domain.DeliveryTarget{Channel: domain.ChannelVKBot, RecipientRef: "not-a-peer"},
	}

	if _, err := publisher.BuildDelivery(context.Background(), job, "delivery:"+job.ID.String()); !errors.Is(err, domain.ErrInvalidResultContract) {
		t.Fatalf("build error = %v, want invalid result contract", err)
	}
}
