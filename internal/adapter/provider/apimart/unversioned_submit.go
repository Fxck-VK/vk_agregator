package apimart

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

// These endpoints have no documented replay contract. Keep accepted and ambiguous
// outcomes under the same key; workers persist accepted tasks for polling.
type unversionedSubmission struct {
	done chan struct{}
	task domain.ProviderTask
	err  error
}

func (p *Provider) submitUnversionedOnce(ctx context.Context, req domain.ProviderRequest, submit func(context.Context, domain.ProviderRequest) (domain.ProviderTask, error)) (domain.ProviderTask, error) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return domain.ProviderTask{}, &Error{Class: domain.ProviderErrInvalidRequest, Message: "idempotency key is required"}
	}
	p.mu.Lock()
	if existing, ok := p.unversionedSubmits[req.IdempotencyKey]; ok {
		p.mu.Unlock()
		select {
		case <-existing.done:
			return existing.task, existing.err
		case <-ctx.Done():
			return domain.ProviderTask{}, unversionedSubmitIndeterminate()
		}
	}
	state := &unversionedSubmission{done: make(chan struct{})}
	p.unversionedSubmits[req.IdempotencyKey] = state
	p.mu.Unlock()
	state.task, state.err = submit(ctx, req)
	close(state.done)
	return state.task, state.err
}

func (p *Provider) postUnversionedTask(ctx context.Context, req domain.ProviderRequest, path string, body []byte) (domain.ProviderTask, error) {
	httpReq, err := p.request(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return domain.ProviderTask{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	resp, err := p.http.Do(httpReq)
	if err != nil {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	defer resp.Body.Close()
	if uncertainUnversionedStatus(resp.StatusCode) {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.ProviderTask{}, decodeHTTPError(resp)
	}
	var decoded submitResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&decoded); err != nil {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	if decoded.Code != 200 {
		if decoded.Code < 400 || uncertainUnversionedStatus(decoded.Code) {
			return domain.ProviderTask{}, unversionedSubmitIndeterminate()
		}
		return domain.ProviderTask{}, apiEnvelopeError(decoded.Code, decoded.Error, decoded.Message)
	}
	if len(decoded.Data) != 1 || strings.TrimSpace(decoded.Data[0].TaskID) == "" {
		return domain.ProviderTask{}, unversionedSubmitIndeterminate()
	}
	now := p.now()
	return domain.ProviderTask{JobID: req.JobID, Provider: domain.ProviderAPIMart, ModelCode: req.ModelCode, ExternalID: strings.TrimSpace(decoded.Data[0].TaskID),
		AttemptNo: 1, Status: domain.ProviderTaskPending, Request: req.Params, SubmittedAt: &now, CreatedAt: now, UpdatedAt: now, IdempotencyKey: req.IdempotencyKey}, nil
}

func uncertainUnversionedStatus(status int) bool {
	return status == 408 || status == 409 || status >= 500
}

func unversionedSubmitIndeterminate() error {
	return &Error{Class: domain.ProviderErrSubmitIndeterminate, Message: "APIMart submission outcome unresolved; automatic resubmission stopped"}
}
