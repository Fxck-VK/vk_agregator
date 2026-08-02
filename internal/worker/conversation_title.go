package worker

import (
	"log/slog"
	"time"

	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
)

const conversationTitleReclaimIdle = 2 * time.Second

// NewConversationTitleEngine isolates best-effort title work from normal text
// generation. Its short reclaim interval is used only for the brief race in
// which the title event arrives before the normal worker has stored the first
// user message.
func NewConversationTitleEngine(reader Reader, handle Handler, logger *slog.Logger) *Engine {
	opts := []EngineOption{WithMinIdle(conversationTitleReclaimIdle)}
	if logger != nil {
		opts = append(opts, WithLogger(logger))
	}
	return NewEngine(reader, []string{redisqueue.StreamConversationTitle}, handle, opts...)
}
