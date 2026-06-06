package vkdelivery

import (
	"context"
	"fmt"
	"sync"
)

// SentMessage records one message accepted by the mock client.
type SentMessage struct {
	PeerID     int64
	RandomID   int64
	Type       string // "text", "photo", "video" or "message"
	Text       string
	Attachment string
	Keyboard   string
	MessageID  int64
}

// EventAnswer records one callback-button acknowledgement.
type EventAnswer struct {
	EventID string
	UserID  int64
	PeerID  int64
}

// MockClient is an in-memory Client for tests and local development. It honors
// VK's random_id semantics: sending the same (peerID, randomID) twice returns
// the original result with Duplicate=true and does not record a second message.
type MockClient struct {
	mu        sync.Mutex
	sent      []SentMessage
	edits     []SentMessage
	answers   []EventAnswer
	byRandom  map[int64]SentMessage
	profiles  map[int64]UserProfile
	nextMsgID int64
	failNext  error
}

// NewMockClient builds an empty mock client.
func NewMockClient() *MockClient {
	return &MockClient{
		byRandom:  map[int64]SentMessage{},
		profiles:  map[int64]UserProfile{},
		nextMsgID: 1000,
	}
}

var (
	_ Client            = (*MockClient)(nil)
	_ ControlClient     = (*MockClient)(nil)
	_ UserProfileClient = (*MockClient)(nil)
)

// FailNext makes the next send return err (used to test retry safety).
func (c *MockClient) FailNext(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failNext = err
}

// SetUserProfile configures the profile returned by GetUserProfile.
func (c *MockClient) SetUserProfile(profile UserProfile) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.profiles[profile.UserID] = profile
}

// Sent returns a copy of all recorded sends.
func (c *MockClient) Sent() []SentMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]SentMessage, len(c.sent))
	copy(out, c.sent)
	return out
}

// Edits returns a copy of all recorded edit operations.
func (c *MockClient) Edits() []SentMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]SentMessage, len(c.edits))
	copy(out, c.edits)
	return out
}

// EventAnswers returns a copy of all recorded callback-button acknowledgements.
func (c *MockClient) EventAnswers() []EventAnswer {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]EventAnswer, len(c.answers))
	copy(out, c.answers)
	return out
}

// SendText implements Client.
func (c *MockClient) SendText(_ context.Context, peerID, randomID int64, text string) (SendResult, error) {
	return c.record(peerID, randomID, "text", text, "", "")
}

// SendPhoto implements Client.
func (c *MockClient) SendPhoto(_ context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error) {
	return c.record(peerID, randomID, "photo", caption, attachment, "")
}

// SendVideo implements Client.
func (c *MockClient) SendVideo(_ context.Context, peerID, randomID int64, attachment, caption string) (SendResult, error) {
	return c.record(peerID, randomID, "video", caption, attachment, "")
}

// SendMessage implements ControlClient.
func (c *MockClient) SendMessage(_ context.Context, peerID, randomID int64, msg Message) (SendResult, error) {
	keyboard := ""
	if msg.Keyboard != nil {
		keyboard, _ = encodeKeyboard(msg.Keyboard)
	}
	return c.record(peerID, randomID, "message", msg.Text, msg.Attachment, keyboard)
}

// EditMessage implements ControlClient.
func (c *MockClient) EditMessage(_ context.Context, peerID, messageID int64, msg Message) (SendResult, error) {
	keyboard := ""
	if msg.Keyboard != nil {
		keyboard, _ = encodeKeyboard(msg.Keyboard)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failNext != nil {
		err := c.failNext
		c.failNext = nil
		return SendResult{}, err
	}

	for i := range c.sent {
		if c.sent[i].PeerID != peerID || c.sent[i].MessageID != messageID {
			continue
		}
		c.sent[i].Text = msg.Text
		c.sent[i].Attachment = msg.Attachment
		c.sent[i].Keyboard = keyboard
		edit := SentMessage{
			PeerID:     peerID,
			Type:       "edit",
			Text:       msg.Text,
			Attachment: msg.Attachment,
			Keyboard:   keyboard,
			MessageID:  messageID,
		}
		c.edits = append(c.edits, edit)
		return SendResult{MessageID: messageID, PeerID: peerID}, nil
	}

	return SendResult{}, fmt.Errorf("vkdelivery: message %d not found for peer %d", messageID, peerID)
}

// AnswerMessageEvent implements ControlClient.
func (c *MockClient) AnswerMessageEvent(_ context.Context, eventID string, userID, peerID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failNext != nil {
		err := c.failNext
		c.failNext = nil
		return err
	}
	c.answers = append(c.answers, EventAnswer{EventID: eventID, UserID: userID, PeerID: peerID})
	return nil
}

// GetUserProfile implements UserProfileClient.
func (c *MockClient) GetUserProfile(_ context.Context, userID int64) (UserProfile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failNext != nil {
		err := c.failNext
		c.failNext = nil
		return UserProfile{}, err
	}
	profile, ok := c.profiles[userID]
	if !ok {
		return UserProfile{UserID: userID}, nil
	}
	return profile, nil
}

func (c *MockClient) record(peerID, randomID int64, kind, text, attachment, keyboard string) (SendResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failNext != nil {
		err := c.failNext
		c.failNext = nil
		return SendResult{}, err
	}

	// random_id dedup: VK would silently drop a repeat send.
	if prev, ok := c.byRandom[randomID]; ok {
		return SendResult{MessageID: prev.MessageID, PeerID: prev.PeerID, Duplicate: true}, nil
	}

	c.nextMsgID++
	msg := SentMessage{
		PeerID:     peerID,
		RandomID:   randomID,
		Type:       kind,
		Text:       text,
		Attachment: attachment,
		Keyboard:   keyboard,
		MessageID:  c.nextMsgID,
	}
	c.sent = append(c.sent, msg)
	c.byRandom[randomID] = msg
	return SendResult{MessageID: msg.MessageID, PeerID: peerID}, nil
}
