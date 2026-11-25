package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/adapter/storage/memory"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/dialogcontext"
	"vk-ai-aggregator/internal/service/presentationintake"
	"vk-ai-aggregator/internal/worker"
)

func TestPresentationIntakeMalformedOutputSettlesWithoutAnotherProviderCall(t *testing.T) {
	ctx := context.Background()
	owner := uuid.New()
	conversations := memory.NewConversationRepo()
	chat := &domain.Conversation{
		AccountID: owner, Source: domain.ConversationSourceWeb, Status: domain.ConversationActive,
		ExternalThreadID: domain.WebPresentationChatReference(uuid.New()),
	}
	if err := conversations.CreateConversation(ctx, chat); err != nil {
		t.Fatal(err)
	}
	provider := &countedPresentationProvider{Provider: &internalMarkerTextProvider{Provider: mock.New(), output: "truncated opaque provider response"}}
	h := newHarnessWithProvider(t, provider, func(d *worker.Deps) {
		d.TextContext = dialogcontext.New(conversations, dialogcontext.Config{})
	})
	prompt := "Тема — керамика. Нужно 10 слайдов."
	params, _ := json.Marshal(map[string]string{"prompt": prompt, "conversation_id": chat.ID.String(), "conversation_source": "web"})
	job := &domain.Job{
		ID: uuid.New(), AccountID: owner, Source: "web", OperationType: domain.OperationTextGenerate, Modality: domain.ModalityText,
		Status: domain.JobStatusQueued, ResultMode: domain.ResultModeAccountHistory, ChannelContext: &domain.ChannelContext{Channel: domain.ChannelWeb},
		IdempotencyKey: domain.WebPresentationIntakeJobKeyPrefix(owner) + uuid.NewString(), CostReserved: 1, Params: params,
	}
	if err := h.jobs.Create(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := h.gen.Process(ctx, taskFor(job)); err != nil {
		t.Fatalf("malformed result did not settle: %v", err)
	}
	got := h.reload(t, job.ID)
	if got.Status != domain.JobStatusRejected || got.ErrorCode != "presentation_intake_invalid_output" || len(h.releaser.released) != 1 || h.releaser.released[0] != job.ID {
		t.Fatalf("unexpected malformed-result settlement: state=%s code=%s released=%v", got.Status, got.ErrorCode, h.releaser.released)
	}
	if _, err := conversations.GetAssistantMessageByJobID(ctx, job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("provider output entered assistant history: %v", err)
	}
	if err := h.gen.Process(ctx, taskFor(job)); err != nil || provider.submits != 1 || len(h.releaser.released) != 1 {
		t.Fatalf("redelivery changed terminal outcome: submits=%d released=%v err=%v", provider.submits, h.releaser.released, err)
	}
	_, rendered, err := dialogcontext.PresentationIntakeEvidence(ctx, conversations, chat.ID, job.ID)
	if err != nil || !strings.Contains(rendered, prompt) || len(rendered)+len(presentationintake.SystemPrompt) > domain.PaidTextMaxInputTokens-512 {
		t.Fatalf("unexpected provider evidence envelope: %q %v", rendered, err)
	}
}
