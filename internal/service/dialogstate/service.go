// Package dialogstate stores short-lived VK conversation mode state outside the
// API process. It keeps product-mode choices alive across restarts and multiple
// API instances without turning them into jobs.
package dialogstate

import (
	"context"
	"fmt"
	"time"
)

const (
	// ModeGPT means ordinary peer text should be routed to text generation.
	ModeGPT = "gpt"

	defaultTTL = time.Hour
)

// Store is the key-value contract used by the dialog state service.
type Store interface {
	Get(ctx context.Context, key string) (value string, ok bool, err error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// Config tunes dialog state retention.
type Config struct {
	TTL time.Duration
}

// Service stores per-peer dialog mode state.
type Service struct {
	store Store
	ttl   time.Duration
}

// New builds a Service. A nil store makes the service a no-op.
func New(store Store, cfg Config) *Service {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Service{store: store, ttl: ttl}
}

// Get returns the current dialog mode for a VK peer. A successful read refreshes
// the TTL so active conversations do not expire mid-dialog.
func (s *Service) Get(ctx context.Context, peerID int64) (string, bool, error) {
	if s.store == nil || peerID == 0 {
		return "", false, nil
	}
	key := modeKey(peerID)
	mode, ok, err := s.store.Get(ctx, key)
	if err != nil || !ok {
		return mode, ok, err
	}
	// Best-effort TTL refresh. The mode is still usable even if refresh fails.
	_ = s.store.Set(ctx, key, mode, s.ttl)
	return mode, true, nil
}

// Set stores the current dialog mode for a VK peer.
func (s *Service) Set(ctx context.Context, peerID int64, mode string) error {
	if s.store == nil || peerID == 0 {
		return nil
	}
	if mode == "" {
		return s.Clear(ctx, peerID)
	}
	return s.store.Set(ctx, modeKey(peerID), mode, s.ttl)
}

// Clear removes dialog mode state for a VK peer.
func (s *Service) Clear(ctx context.Context, peerID int64) error {
	if s.store == nil || peerID == 0 {
		return nil
	}
	return s.store.Delete(ctx, modeKey(peerID))
}

func modeKey(peerID int64) string {
	return fmt.Sprintf("vk:peer:%d:dialog_mode", peerID)
}
