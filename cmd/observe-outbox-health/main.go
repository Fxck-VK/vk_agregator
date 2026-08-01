// Command observe-outbox-health emits one read-only, count-only outbox health
// snapshot for operator scale gates. It never reads payload or identity fields.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
)

// Result is the privacy-safe operator snapshot.
type Result struct {
	Pending                 int64   `json:"pending"`
	Processing              int64   `json:"processing"`
	Failed                  int64   `json:"failed"`
	OldestPendingAgeSeconds float64 `json:"oldest_pending_age_seconds"`
	ExpiredLeases           int64   `json:"expired_leases"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "observe-outbox-health: failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, output io.Writer) error {
	cfg := config.Load()
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	result, err := observeOutboxHealth(ctx, postgres.NewOutboxRepository(pool), time.Now().UTC())
	if err != nil {
		return err
	}
	return writeResult(output, result)
}

func observeOutboxHealth(
	ctx context.Context,
	repository domain.OutboxHealthRepository,
	now time.Time,
) (Result, error) {
	snapshot, err := repository.OutboxHealthSnapshot(ctx, now)
	if err != nil {
		return Result{}, err
	}
	oldestPendingAge := time.Duration(0)
	if snapshot.OldestPendingCreatedAt != nil && snapshot.OldestPendingCreatedAt.Before(now) {
		oldestPendingAge = now.Sub(*snapshot.OldestPendingCreatedAt)
	}
	return Result{
		Pending:                 snapshot.Pending,
		Processing:              snapshot.Processing,
		Failed:                  snapshot.Failed,
		OldestPendingAgeSeconds: oldestPendingAge.Seconds(),
		ExpiredLeases:           snapshot.ExpiredLeases,
	}, nil
}

func writeResult(output io.Writer, result Result) error {
	return json.NewEncoder(output).Encode(result)
}
