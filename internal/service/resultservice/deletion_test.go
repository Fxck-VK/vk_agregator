package resultservice

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"vk-ai-aggregator/internal/domain"
)

func TestDeletedOutputIsATombstoneWithoutHidingOtherOutputs(t *testing.T) {
	ctx := context.Background()
	owner, job, artifacts, jobs, repo, moderation := createValidResultFixture(t, ctx)
	artifacts[0].Status = domain.ArtifactStatusDeleted
	if err := repo.Update(ctx, artifacts[0]); err != nil {
		t.Fatal(err)
	}
	result, err := New(jobs, repo, moderation).GetResult(ctx, owner, job.ID)
	if err != nil {
		t.Fatalf("partial deletion hid the result: %v", err)
	}
	if len(result.Artifacts) != len(artifacts) {
		t.Fatal("other outputs or tombstone lost")
	}
	raw, _ := json.Marshal(result.Artifacts[0])
	var tombstone map[string]any
	_ = json.Unmarshal(raw, &tombstone)
	if tombstone["deleted"] != true || tombstone["size_bytes"] != float64(0) {
		t.Fatalf("unsafe tombstone: %s", raw)
	}
	if result.Artifacts[1].ID != artifacts[1].ID || result.Artifacts[1].SizeBytes == 0 {
		t.Fatal("remaining output was altered")
	}
	scenario := newResultScenario()
	scenario.job.Status = domain.JobStatusResultReady
	scenario.artifacts[0].Status = domain.ArtifactStatusDeleted
	if err := persistResultScenario(t, ctx, scenario).RequireCompletionReady(ctx, scenario.owner, scenario.job.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted output allowed capture: %v", err)
	}
}
