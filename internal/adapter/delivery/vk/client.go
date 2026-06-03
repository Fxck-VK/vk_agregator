// Package vkdelivery is the outbound VK gateway: it sends job results back into
// VK conversations. It is the only place VK messages.send is called, keeping VK
// API details out of the delivery worker. Every send carries a random_id so VK
// itself deduplicates retried sends (no duplicate messages).
package vkdelivery

import (
	"context"
	"hash/fnv"
)

// SendResult is the normalized outcome of a successful send.
type SendResult struct {
	// MessageID is the VK message id assigned to the sent message.
	MessageID int64
	// PeerID echoes the conversation the message went to.
	PeerID int64
	// Duplicate is true when VK (or the mock) recognized the random_id as an
	// already-sent message and did not deliver a second copy.
	Duplicate bool
}

// Client sends messages and media to VK conversations. Implementations must
// pass random_id through to messages.send so retries are deduplicated.
type Client interface {
	// SendText sends a plain text message.
	SendText(ctx context.Context, peerID, randomID int64, text string) (SendResult, error)
	// SendPhoto sends a photo (referenced by a VK attachment string or URL) with
	// an optional caption.
	SendPhoto(ctx context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error)
	// SendVideo sends a video (referenced by a VK attachment string or URL) with
	// an optional caption.
	SendVideo(ctx context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error)
}

// DeterministicRandomID derives a stable, non-negative random_id from a key
// (e.g. a delivery idempotency key). The same key always yields the same id, so
// a retried delivery re-sends with the same random_id and VK suppresses the
// duplicate.
func DeterministicRandomID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	// Mask the sign bit to keep the id non-negative (VK random_id is int32/int64
	// positive in practice).
	return int64(h.Sum64() & 0x7fffffffffffffff)
}
