package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/service/resultreadyreconciliation"
)

type staticReconciler struct {
	result resultreadyreconciliation.Result
	err    error
	limit  int
}

func (r *staticReconciler) Reconcile(_ context.Context, limit int) (resultreadyreconciliation.Result, error) {
	r.limit = limit
	return r.result, r.err
}

func TestParseLimitIsPositiveAndBounded(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		want    int
		wantErr bool
	}{
		{name: "default", want: 1000},
		{name: "explicit", args: []string{"--limit", "25"}, want: 25},
		{name: "zero", args: []string{"--limit", "0"}, wantErr: true},
		{name: "negative", args: []string{"--limit", "-1"}, wantErr: true},
		{name: "over max", args: []string{"--limit", "1001"}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseLimit(test.args)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseLimit(%v) = %d, nil; want error", test.args, got)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("parseLimit(%v) = %d, %v; want %d, nil", test.args, got, err, test.want)
			}
		})
	}
}

func TestWriteResultEmitsOneCountOnlyJSONDocument(t *testing.T) {
	result := reconciliationObservation{
		DurationSeconds: 2.75,
		Candidates:      9,
		Eligible:        8,
		Existing:        6,
		Created:         2,
		Blocked:         1,
		HasMore:         true,
	}
	var output bytes.Buffer
	if err := writeResult(&output, result); err != nil {
		t.Fatalf("writeResult: %v", err)
	}
	if strings.Count(output.String(), "\n") != 1 {
		t.Fatalf("output = %q, want exactly one JSON line", output.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	wantKeys := []string{"duration_seconds", "candidates", "eligible", "existing", "created", "blocked", "has_more"}
	if len(decoded) != len(wantKeys) {
		t.Fatalf("output fields = %#v, want exact count/page fields", decoded)
	}
	for _, key := range wantKeys {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("output missing count field %q: %#v", key, decoded)
		}
	}
	if got := decoded["has_more"]; got != true {
		t.Fatalf("has_more = %v, want true", got)
	}
	if got := decoded["duration_seconds"]; got != 2.75 {
		t.Fatalf("duration_seconds = %v, want 2.75", got)
	}
	for _, forbidden := range []string{
		"job_id",
		"account_id",
		"correlation_id",
		"prompt",
		"provider",
		"artifact",
		"payload",
	} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("output contains forbidden field %q: %s", forbidden, output.String())
		}
	}
}

func TestReconcilePageReturnsDurationAndPrivacySafeCountSummary(t *testing.T) {
	startedAt := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	clockValues := []time.Time{startedAt, startedAt.Add(2750 * time.Millisecond)}
	clockCalls := 0
	now := func() time.Time {
		value := clockValues[clockCalls]
		clockCalls++
		return value
	}
	reconciler := &staticReconciler{result: resultreadyreconciliation.Result{
		Candidates: 9,
		Eligible:   8,
		Existing:   6,
		Created:    2,
		Blocked:    1,
		HasMore:    true,
	}}

	got, err := reconcilePage(context.Background(), reconciler, 100, now)
	if err != nil {
		t.Fatalf("reconcile page: %v", err)
	}
	if reconciler.limit != 100 {
		t.Fatalf("reconcile limit = %d, want 100", reconciler.limit)
	}
	if got.DurationSeconds != 2.75 {
		t.Fatalf("duration_seconds = %v, want 2.75", got.DurationSeconds)
	}
	if got.Candidates != 9 || got.Eligible != 8 || got.Existing != 6 || got.Created != 2 || got.Blocked != 1 || !got.HasMore {
		t.Fatalf("count summary = %+v, want all result counts", got)
	}

	var output bytes.Buffer
	if err := writeResult(&output, got); err != nil {
		t.Fatalf("write result: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if gotDuration := decoded["duration_seconds"]; gotDuration != 2.75 {
		t.Fatalf("output duration_seconds = %v, want 2.75", gotDuration)
	}
	for _, forbidden := range []string{
		"job_id",
		"account_id",
		"correlation_id",
		"prompt",
		"provider",
		"artifact",
		"payload",
	} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("output contains forbidden field %q: %s", forbidden, output.String())
		}
	}
}
