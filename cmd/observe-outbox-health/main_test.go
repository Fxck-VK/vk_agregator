package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

type staticOutboxHealthRepository struct {
	snapshot domain.OutboxHealth
	err      error
}

func (r staticOutboxHealthRepository) OutboxHealthSnapshot(context.Context, time.Time) (domain.OutboxHealth, error) {
	return r.snapshot, r.err
}

func TestObserveOutboxHealthReturnsCountOnlyOperatorSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	oldest := now.Add(-90 * time.Second)
	result, err := observeOutboxHealth(context.Background(), staticOutboxHealthRepository{
		snapshot: domain.OutboxHealth{
			Pending:                4,
			Processing:             3,
			Failed:                 2,
			OldestPendingCreatedAt: &oldest,
			ExpiredLeases:          1,
		},
	}, now)
	if err != nil {
		t.Fatalf("observe outbox health: %v", err)
	}
	if result != (Result{
		Pending:                 4,
		Processing:              3,
		Failed:                  2,
		OldestPendingAgeSeconds: 90,
		ExpiredLeases:           1,
	}) {
		t.Fatalf("result = %+v, want count-only snapshot", result)
	}

	var output bytes.Buffer
	if err := writeResult(&output, result); err != nil {
		t.Fatalf("write result: %v", err)
	}
	for _, forbidden := range []string{"aggregate", "payload", "claim_owner", "error_code"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("operator snapshot leaked %q: %s", forbidden, output.String())
		}
	}
}
