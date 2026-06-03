package mock_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/adapter/provider/mock"
	"vk-ai-aggregator/internal/domain"
)

func req(op domain.OperationType, mod domain.Modality, prompt string) domain.ProviderRequest {
	return domain.ProviderRequest{
		JobID:     uuid.New(),
		Operation: op,
		Modality:  mod,
		Prompt:    prompt,
	}
}

func TestEstimateSupportedOperations(t *testing.T) {
	p := mock.New()
	ctx := context.Background()
	cases := map[domain.OperationType]int64{
		domain.OperationTextGenerate:  1,
		domain.OperationImageGenerate: 10,
		domain.OperationVideoGenerate: 50,
	}
	for op, want := range cases {
		est, err := p.Estimate(ctx, req(op, domain.ModalityText, ""))
		if err != nil {
			t.Fatalf("estimate %s: %v", op, err)
		}
		if est.AmountCredits != want {
			t.Errorf("estimate %s = %d, want %d", op, est.AmountCredits, want)
		}
	}
}

func TestSubmitUnsupportedOperation(t *testing.T) {
	p := mock.New()
	_, err := p.Submit(context.Background(), req(domain.OperationAudioTTS, domain.ModalityAudio, "hi"))
	var perr *mock.Error
	if !asMockError(err, &perr) || perr.Class != domain.ProviderErrUnsupportedCapab {
		t.Fatalf("expected unsupported_capability error, got %v", err)
	}
}

func TestSubmitPollSuccess(t *testing.T) {
	p := mock.New() // completes after 1 poll
	ctx := context.Background()

	task, err := p.Submit(ctx, req(domain.OperationImageGenerate, domain.ModalityImage, "neon cat"))
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if task.Status != domain.ProviderTaskPending || task.ExternalID == "" {
		t.Fatalf("unexpected submitted task: %+v", task)
	}

	res, err := p.Poll(ctx, domain.ProviderTaskRef{Provider: domain.ProviderMock, ExternalID: task.ExternalID})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if res.Status != domain.ProviderTaskSucceeded {
		t.Fatalf("status = %q, want succeeded", res.Status)
	}
	if len(res.OutputURLs) != 1 {
		t.Fatalf("expected one output url, got %v", res.OutputURLs)
	}
}

func TestPollProcessingThenSuccess(t *testing.T) {
	p := mock.New(mock.WithCompleteAfterPolls(2))
	ctx := context.Background()
	task, _ := p.Submit(ctx, req(domain.OperationTextGenerate, domain.ModalityText, "post"))
	ref := domain.ProviderTaskRef{Provider: domain.ProviderMock, ExternalID: task.ExternalID}

	if res, _ := p.Poll(ctx, ref); res.Status != domain.ProviderTaskProcessing {
		t.Fatalf("first poll = %q, want processing", res.Status)
	}
	if res, _ := p.Poll(ctx, ref); res.Status != domain.ProviderTaskSucceeded {
		t.Fatalf("second poll = %q, want succeeded", res.Status)
	}
}

func TestErrorTriggers(t *testing.T) {
	ctx := context.Background()
	cases := map[string]domain.ProviderErrorClass{
		mock.TriggerTimeout:       domain.ProviderErrTimeout,
		mock.TriggerRateLimit:     domain.ProviderErrRateLimited,
		mock.TriggerProviderError: domain.ProviderErrInternal,
	}
	for trigger, wantClass := range cases {
		p := mock.New()
		task, err := p.Submit(ctx, req(domain.OperationVideoGenerate, domain.ModalityVideo, "make "+trigger+" please"))
		if err != nil {
			t.Fatalf("submit %s: %v", trigger, err)
		}
		res, err := p.Poll(ctx, domain.ProviderTaskRef{Provider: domain.ProviderMock, ExternalID: task.ExternalID})
		if err != nil {
			t.Fatalf("poll %s: %v", trigger, err)
		}
		if res.Status != domain.ProviderTaskFailed || res.ErrorClass != wantClass {
			t.Fatalf("trigger %s: status=%q class=%q, want failed/%q", trigger, res.Status, res.ErrorClass, wantClass)
		}
	}
}

func TestCancel(t *testing.T) {
	p := mock.New()
	ctx := context.Background()
	task, _ := p.Submit(ctx, req(domain.OperationVideoGenerate, domain.ModalityVideo, "long video"))
	ref := domain.ProviderTaskRef{Provider: domain.ProviderMock, ExternalID: task.ExternalID}
	if err := p.Cancel(ctx, ref); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	res, _ := p.Poll(ctx, ref)
	if res.Status != domain.ProviderTaskCancelled {
		t.Fatalf("status = %q, want cancelled", res.Status)
	}
}

func asMockError(err error, target **mock.Error) bool {
	if e, ok := err.(*mock.Error); ok {
		*target = e
		return true
	}
	return false
}
