// Command reconcile-result-ready reconstructs missing result-ready outbox
// events in one bounded, operator-invoked pass.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/resultreadyreconciliation"
	"vk-ai-aggregator/internal/service/resultservice"
)

const maxReconcileLimit = 1000

type reconciliationPageReconciler interface {
	Reconcile(context.Context, int) (resultreadyreconciliation.Result, error)
}

// reconciliationObservation is the durable, privacy-safe result of one
// bounded reconciliation page. It intentionally contains no identifiers or
// event payload fields so stdout can be retained as rollout/load evidence.
type reconciliationObservation struct {
	DurationSeconds float64 `json:"duration_seconds"`
	Candidates      int     `json:"candidates"`
	Eligible        int     `json:"eligible"`
	Existing        int     `json:"existing"`
	Created         int     `json:"created"`
	Blocked         int     `json:"blocked"`
	HasMore         bool    `json:"has_more"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "reconcile-result-ready: failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	limit, err := parseLimit(args)
	if err != nil {
		return err
	}
	cfg := config.Load()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	reconciler := resultreadyreconciliation.New(
		postgres.NewResultReadyCandidateRepository(pool),
		resultservice.New(
			postgres.NewJobRepository(pool),
			postgres.NewArtifactRepository(pool),
			postgres.NewModerationResultRepository(pool),
		),
		postgres.NewUnitOfWork(pool),
	)
	result, err := reconcilePage(ctx, reconciler, limit, time.Now)
	if err != nil {
		return err
	}
	return writeResult(output, result)
}

func reconcilePage(
	ctx context.Context,
	reconciler reconciliationPageReconciler,
	limit int,
	now func() time.Time,
) (reconciliationObservation, error) {
	startedAt := now()
	result, err := reconciler.Reconcile(ctx, limit)
	if err != nil {
		return reconciliationObservation{}, err
	}
	duration := now().Sub(startedAt)
	if duration < 0 {
		duration = 0
	}
	return reconciliationObservation{
		DurationSeconds: duration.Seconds(),
		Candidates:      result.Candidates,
		Eligible:        result.Eligible,
		Existing:        result.Existing,
		Created:         result.Created,
		Blocked:         result.Blocked,
		HasMore:         result.HasMore,
	}, nil
}

func parseLimit(args []string) (int, error) {
	flags := flag.NewFlagSet("reconcile-result-ready", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	limit := flags.Int("limit", maxReconcileLimit, "maximum events to create in this pass")
	if err := flags.Parse(args); err != nil {
		return 0, err
	}
	if flags.NArg() != 0 {
		return 0, errors.New("unexpected positional arguments")
	}
	if *limit <= 0 || *limit > maxReconcileLimit {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxReconcileLimit)
	}
	return *limit, nil
}

func writeResult(output io.Writer, result reconciliationObservation) error {
	return json.NewEncoder(output).Encode(result)
}
