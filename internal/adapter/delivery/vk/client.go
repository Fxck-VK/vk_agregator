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

// UserProfile is the subset of VK user data needed by product/control UX.
type UserProfile struct {
	UserID    int64
	FirstName string
	LastName  string
}

// Message is a full VK outbound message. It is used for product/control
// responses that need a keyboard or a pre-uploaded attachment.
type Message struct {
	Text       string
	Attachment string
	Keyboard   *Keyboard
}

// Keyboard is the subset of VK bot keyboard JSON the product menu needs.
type Keyboard struct {
	OneTime bool
	Inline  bool
	Buttons [][]KeyboardButton
}

// KeyboardButton is a VK keyboard button. Payload must be a JSON string because
// VK sends it back on button clicks (`message.payload` for text buttons,
// `message_event.object.payload` for callback buttons).
type KeyboardButton struct {
	Label      string
	Payload    string
	Color      string
	ActionType string
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

// ControlClient sends non-job control/product messages such as the /start menu.
type ControlClient interface {
	SendMessage(ctx context.Context, peerID, randomID int64, msg Message) (SendResult, error)
	EditMessage(ctx context.Context, peerID, messageID int64, msg Message) (SendResult, error)
	AnswerMessageEvent(ctx context.Context, eventID string, userID, peerID int64) error
}

// UserProfileClient fetches VK user profile fields for personalization.
type UserProfileClient interface {
	GetUserProfile(ctx context.Context, userID int64) (UserProfile, error)
}

// MediaUploader uploads raw artifact bytes to VK and returns the canonical
// attachment string accepted by messages.send, e.g. photo123_456_accesskey.
type MediaUploader interface {
	UploadPhoto(ctx context.Context, peerID int64, filename string, data []byte, mimeType string) (string, error)
	UploadVideo(ctx context.Context, peerID int64, filename string, data []byte, mimeType string) (string, error)
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
