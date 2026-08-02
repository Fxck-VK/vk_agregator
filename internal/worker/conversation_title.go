package worker

import (
	"log/slog"
	"time"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
)

// conversationTitleReclaimIdle must remain longer than the bounded first-
// message wait plus the DeepInfra request timeout. Redis uses this value as the
// cross-consumer lease for a pending entry; reclaiming it earlier could make
// two title workers call the provider for the same conversation at once.
const conversationTitleReclaimIdle = 30 * time.Second

// NewConversationTitleEngine isolates best-effort title work from normal text
// generation. Its independent, safe recovery lease ensures a slow provider
// call cannot delay or duplicate normal user-visible chat work.
func NewConversationTitleEngine(reader Reader, handle Handler, logger *slog.Logger) *Engine {
	// A title task can spend most of its lease in a bounded provider call. Read
	// exactly one at a time so Redis never marks a later batch entry pending
	// before this worker is ready to process it; more worker replicas provide
	// throughput without weakening the per-task lease.
	opts := []EngineOption{WithCount(1), WithMinIdle(conversationTitleReclaimIdle)}
	if logger != nil {
		opts = append(opts, WithLogger(logger))
	}
	return NewEngine(reader, []string{redisqueue.StreamConversationTitle}, handle, opts...)
}
