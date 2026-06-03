// Package vk implements the VK inbound gateway: a thin HTTP handler that accepts
// VK Callback API events, persists them idempotently and turns them into
// commands and jobs. It deliberately knows nothing about AI providers, billing
// internals or delivery — it only translates VK events into the platform's
// normalized intake flow (invariant: VK handlers never call providers).
package vk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
)

// Config holds the per-deployment VK callback settings.
type Config struct {
	// ConfirmationToken is returned verbatim for "confirmation" events so VK can
	// verify the callback server.
	ConfirmationToken string
	// Secret, when non-empty, must match the secret on every incoming event.
	Secret string
}

// Deps are the collaborators the handler needs. All are interfaces or services
// so the handler stays storage- and provider-agnostic.
type Deps struct {
	Idempotency  domain.IdempotencyRepository
	Inbound      domain.InboundEventRepository
	Users        domain.UserRepository
	Commands     domain.CommandRepository
	Billing      *billingservice.Service
	Orchestrator *joborchestrator.Orchestrator
	Router       *commandrouter.Router
	Logger       *slog.Logger
}

// Handler serves the POST /webhooks/vk endpoint.
type Handler struct {
	cfg    Config
	deps   Deps
	logger *slog.Logger
}

// NewHandler builds a VK callback handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{cfg: cfg, deps: deps, logger: logger}
}

// callback is the common VK Callback API envelope.
type callback struct {
	Type    string          `json:"type"`
	GroupID int64           `json:"group_id"`
	EventID string          `json:"event_id"`
	Secret  string          `json:"secret"`
	Object  json.RawMessage `json:"object"`
}

// messageNew is the object payload for a "message_new" event. VK nests the
// message under "message" in modern API versions; the flat fields cover older
// versions.
type messageNew struct {
	Message *vkMessage `json:"message"`
	FromID  int64      `json:"from_id"`
	PeerID  int64      `json:"peer_id"`
	Text    string     `json:"text"`
}

type vkMessage struct {
	FromID                int64  `json:"from_id"`
	PeerID                int64  `json:"peer_id"`
	Text                  string `json:"text"`
	ConversationMessageID int64  `json:"conversation_message_id"`
}

func (m messageNew) resolve() (fromID, peerID int64, text string) {
	if m.Message != nil {
		return m.Message.FromID, m.Message.PeerID, m.Message.Text
	}
	return m.FromID, m.PeerID, m.Text
}

// ServeHTTP implements http.Handler. It always responds quickly: "confirmation"
// returns the token, recognized events return "ok", and only genuine server
// failures return 5xx so VK retries.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	var cb callback
	if err := json.Unmarshal(body, &cb); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if h.cfg.Secret != "" && cb.Secret != h.cfg.Secret {
		http.Error(w, "invalid secret", http.StatusForbidden)
		return
	}

	switch cb.Type {
	case "confirmation":
		writeText(w, http.StatusOK, h.cfg.ConfirmationToken)
	case "message_new":
		if err := h.handleMessageNew(r.Context(), cb, body); err != nil {
			h.logger.Error("vk message_new processing failed",
				slog.Int64("group_id", cb.GroupID), slog.String("error", err.Error()))
			http.Error(w, "processing error", http.StatusInternalServerError)
			return
		}
		writeText(w, http.StatusOK, "ok")
	default:
		// Acknowledge unhandled event types so VK does not retry them.
		writeText(w, http.StatusOK, "ok")
	}
}

func (h *Handler) handleMessageNew(ctx context.Context, cb callback, rawBody []byte) error {
	eventID := cb.EventID

	var obj messageNew
	if len(cb.Object) > 0 {
		if err := json.Unmarshal(cb.Object, &obj); err != nil {
			return fmt.Errorf("decode object: %w", err)
		}
	}
	fromID, peerID, text := obj.resolve()
	if fromID == 0 {
		return errors.New("message has no from_id")
	}
	if eventID == "" {
		// Fall back to a stable synthetic id when VK omits event_id.
		eventID = fmt.Sprintf("%d:%d:%x", peerID, fromID, hashText(text))
	}

	idemKey := fmt.Sprintf("vk_event:%d:%s", cb.GroupID, eventID)
	record := &domain.IdempotencyRecord{
		Key:          idemKey,
		Scope:        "inbound_event",
		ResourceType: "command",
	}
	existing, created, err := h.deps.Idempotency.GetOrCreate(ctx, record)
	if err != nil {
		return fmt.Errorf("idempotency: %w", err)
	}
	if !created && existing.Status == domain.IdempotencyCompleted {
		// Already fully processed: this is a duplicate delivery.
		return nil
	}

	if err := h.process(ctx, cb, rawBody, eventID, idemKey, fromID, peerID, text); err != nil {
		_ = h.deps.Idempotency.MarkFailed(ctx, idemKey)
		return err
	}
	return nil
}

// process runs the InboundEvent -> User -> Command -> Job flow.
func (h *Handler) process(ctx context.Context, cb callback, rawBody []byte, eventID, idemKey string, fromID, peerID int64, text string) error {
	// InboundEvent: persist the raw event for audit and reprocessing.
	inbound := &domain.InboundEvent{
		Source:         "vk",
		EventType:      cb.Type,
		GroupID:        cb.GroupID,
		VKEventID:      eventID,
		PeerID:         peerID,
		VKUserID:       fromID,
		Payload:        json.RawMessage(rawBody),
		Status:         domain.InboundReceived,
		IdempotencyKey: idemKey,
	}
	if err := h.deps.Inbound.Create(ctx, inbound); err != nil && !errors.Is(err, domain.ErrConflict) {
		return fmt.Errorf("save inbound: %w", err)
	}

	// User: get or create, granting the starting balance to brand-new users.
	user, err := h.ensureUser(ctx, fromID)
	if err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}

	// Command: classify the message into a normalized command.
	parsed := h.deps.Router.Parse(text)
	cmd := &domain.Command{
		UserID:         user.ID,
		VKPeerID:       peerID,
		InboundEventID: inbound.ID,
		Type:           parsed.Type,
		RawText:        text,
		IdempotencyKey: "vk_cmd:" + strconv.FormatInt(cb.GroupID, 10) + ":" + eventID,
		CorrelationID:  idemKey,
	}
	if err := h.deps.Commands.Create(ctx, cmd); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			if existing, getErr := h.deps.Commands.GetByIdempotencyKey(ctx, cmd.IdempotencyKey); getErr == nil {
				cmd = existing
			} else {
				return fmt.Errorf("load existing command: %w", getErr)
			}
		} else {
			return fmt.Errorf("save command: %w", err)
		}
	}

	resourceID := cmd.ID

	// Job: only commands that map to an AI operation become jobs. Control
	// commands (balance/status/cancel/help) are recorded but produce no job.
	if parsed.CreatesJob() {
		job, err := h.deps.Orchestrator.CreateJob(ctx, joborchestrator.CreateJobInput{
			UserID:         user.ID,
			VKPeerID:       peerID,
			CommandID:      cmd.ID,
			Operation:      parsed.Operation,
			Modality:       parsed.Modality,
			IdempotencyKey: "vk_job:" + strconv.FormatInt(cb.GroupID, 10) + ":" + eventID,
			CorrelationID:  idemKey,
		})
		switch {
		case err == nil:
			resourceID = job.ID
		case errors.Is(err, domain.ErrInsufficientCredits):
			// Expected business outcome: job is parked in awaiting_payment.
			resourceID = job.ID
		default:
			return fmt.Errorf("create job: %w", err)
		}
	}

	if err := h.deps.Inbound.SetStatus(ctx, inbound.ID, domain.InboundProcessed); err != nil {
		return fmt.Errorf("mark inbound processed: %w", err)
	}
	if err := h.deps.Idempotency.MarkCompleted(ctx, idemKey, resourceID); err != nil {
		return fmt.Errorf("mark idempotency completed: %w", err)
	}
	return nil
}

func (h *Handler) ensureUser(ctx context.Context, vkUserID int64) (*domain.User, error) {
	user, err := h.deps.Users.GetByVKUserID(ctx, vkUserID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	user = &domain.User{
		VKUserID: vkUserID,
		Role:     domain.RoleUser,
		Status:   domain.StatusActive,
		Locale:   "ru",
		Timezone: "Europe/Moscow",
	}
	if err := h.deps.Users.Create(ctx, user); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return h.deps.Users.GetByVKUserID(ctx, vkUserID)
		}
		return nil, err
	}
	if _, err := h.deps.Billing.EnsureAccount(ctx, user.ID); err != nil {
		return nil, fmt.Errorf("ensure account: %w", err)
	}
	return user, nil
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// hashText is a tiny FNV-1a hash used only to synthesize a stable event id when
// VK does not provide one.
func hashText(s string) uint32 {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	h := uint32(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime
	}
	return h
}
