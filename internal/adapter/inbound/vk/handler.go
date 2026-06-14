// Package vk implements the VK inbound gateway: a thin HTTP handler that accepts
// VK Callback API events, persists them idempotently and turns them into
// commands and jobs. It deliberately knows nothing about AI providers. VK
// control responses, when enabled, are sent only through the outbound VK
// delivery adapter, preserving the "no direct VK API calls outside delivery"
// boundary.
package vk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/referralservice"
)

// Config holds the per-deployment VK callback settings.
type Config struct {
	// ConfirmationToken is returned verbatim for "confirmation" events so VK can
	// verify the callback server.
	ConfirmationToken string
	// Secret, when non-empty, must match the secret on every incoming event.
	Secret string
	// WelcomeAttachment is an optional pre-uploaded VK attachment string
	// (for example photo-239332376_123_accesskey) sent with the /start menu.
	WelcomeAttachment string
	// MenuButtonMode controls inline menu button action type: "callback" or
	// "text". Callback avoids user echo messages; text preserves legacy VK UX.
	MenuButtonMode string
	// UnroutedTextMode controls plain text outside an active text mode:
	// "reply" asks the user to choose a mode, "silent" does nothing, and
	// "gpt" preserves the legacy behavior where any text becomes a GPT job.
	UnroutedTextMode string
	// MenuFeatures controls which VK product-menu buttons are visible and
	// reachable. Empty means every known menu command is enabled.
	MenuFeatures MenuFeatureFlags
	// ReferralLinkBase builds a single VK referral link per user. If empty the
	// handler falls back to the current callback group id.
	ReferralLinkBase string
	// ReferralShareBase is reserved for future share/open-link flows.
	ReferralShareBase string
	// ReferralReferrerSignupRewardCredits is shown in the account referral copy.
	ReferralReferrerSignupRewardCredits int64
	// TopUpReceiptEmail/TopUpReceiptPhone are server-side receipt contacts used
	// by the VK bot quick top-up flow. They keep receipt collection out of chat
	// while preserving paymentservice/YooKassa receipt requirements.
	TopUpReceiptEmail string
	TopUpReceiptPhone string
	// TopUpReturnURL is the server-owned YooKassa redirect target for bot
	// top-up intents.
	TopUpReturnURL string
}

// MenuFeatureFlags allows deployments to hide VK menu buttons without deleting
// the menu screens or changing command parsing.
type MenuFeatureFlags struct {
	DisabledCommands map[domain.CommandType]bool
}

// AntiSpam checks per-user VK bot limits after command routing but before
// command/job persistence.
type AntiSpam interface {
	Check(ctx context.Context, input antispam.CheckInput) (antispam.Decision, error)
}

// DialogState stores per-peer VK mode state outside the API process.
type DialogState interface {
	Get(ctx context.Context, peerID int64) (mode string, ok bool, err error)
	Set(ctx context.Context, peerID int64, mode string) error
	Clear(ctx context.Context, peerID int64) error
}

// ReferralService is the shared backend referral service used by VK bot and
// future VK Mini App flows.
type ReferralService interface {
	Stats(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, int, error)
	StatsDetailed(ctx context.Context, userID uuid.UUID) (*domain.ReferralCode, domain.ReferralStats, error)
	Apply(ctx context.Context, input referralservice.ApplyInput) (referralservice.ApplyResult, error)
	Activate(ctx context.Context, input referralservice.ActivateInput) (referralservice.ActivateResult, error)
}

// Deps are the collaborators the handler needs. All are interfaces or services
// so the handler stays storage- and provider-agnostic.
type Deps struct {
	Idempotency  domain.IdempotencyRepository
	Inbound      domain.InboundEventRepository
	Users        domain.UserRepository
	Jobs         domain.JobRepository
	Commands     domain.CommandRepository
	Billing      *billingservice.Service
	Payment      *paymentservice.Service
	Referrals    ReferralService
	Orchestrator *joborchestrator.Orchestrator
	Router       *commandrouter.Router
	Control      vkdelivery.ControlClient
	Profile      vkdelivery.UserProfileClient
	DialogState  DialogState
	AntiSpam     AntiSpam
	Logger       *slog.Logger
}

// Handler serves the POST /webhooks/vk endpoint.
type Handler struct {
	cfg    Config
	deps   Deps
	logger *slog.Logger

	menuMu      sync.Mutex
	activeMenus map[int64]activeMenuMessage

	modeMu      sync.Mutex
	dialogModes map[int64]dialogMode
}

// NewHandler builds a VK callback handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	cfg.MenuButtonMode = normalizeMenuButtonMode(cfg.MenuButtonMode)
	cfg.UnroutedTextMode = normalizeUnroutedTextMode(cfg.UnroutedTextMode)
	return &Handler{
		cfg:         cfg,
		deps:        deps,
		logger:      logger,
		activeMenus: map[int64]activeMenuMessage{},
		dialogModes: map[int64]dialogMode{},
	}
}

func normalizeMenuButtonMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "text":
		return "text"
	default:
		return "callback"
	}
}

const (
	unroutedTextModeReply  = "reply"
	unroutedTextModeSilent = "silent"
	unroutedTextModeGPT    = "gpt"
)

func normalizeUnroutedTextMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case unroutedTextModeSilent:
		return unroutedTextModeSilent
	case unroutedTextModeGPT:
		return unroutedTextModeGPT
	default:
		return unroutedTextModeReply
	}
}

type activeMenuMessage struct {
	MessageID int64
}

type dialogMode string

const (
	dialogModeGPT       dialogMode = "gpt"
	dialogModePhotoText dialogMode = "photo_text"
)

const topUpActionNewPayment = "new_payment"

type jobParams struct {
	Prompt                 string `json:"prompt"`
	ModelID                string `json:"model_id,omitempty"`
	ModelName              string `json:"model_name,omitempty"`
	Provider               string `json:"provider,omitempty"`
	ModelCode              string `json:"model_code,omitempty"`
	DurationSec            int    `json:"duration_sec,omitempty"`
	VKPlaceholderMessageID int64  `json:"vk_placeholder_message_id,omitempty"`
}

type videoModeSpec struct {
	Mode        dialogMode
	ModelID     string
	ModelName   string
	Provider    domain.ProviderName
	ModelCode   string
	DurationSec int
}

func videoModeForCommand(t domain.CommandType) (videoModeSpec, bool) {
	switch t {
	case domain.CommandMenuVideoPrunaAI:
		return videoModeFromCatalog("video:prunaai", modelcatalog.VKVideoPrunaAI)
	case domain.CommandMenuVideoSora2Start:
		return videoModeFromCatalog("video:sora_2", modelcatalog.VKVideoSora2)
	default:
		return videoModeSpec{}, false
	}
}

func videoModeFromCatalog(mode dialogMode, modelID string) (videoModeSpec, bool) {
	model, ok := modelcatalog.ResolveVKVideoModel(modelID)
	if !ok || strings.TrimSpace(model.ModelCode) == "" {
		return videoModeSpec{}, false
	}
	return videoModeSpec{
		Mode:        mode,
		ModelID:     model.ModelID,
		ModelName:   model.ModelName,
		Provider:    model.Provider,
		ModelCode:   model.ModelCode,
		DurationSec: model.DurationSec,
	}, true
}

func videoModeFromDialogMode(mode dialogMode) (videoModeSpec, bool) {
	for _, command := range []domain.CommandType{
		domain.CommandMenuVideoPrunaAI,
		domain.CommandMenuVideoSora2Start,
		domain.CommandMenuVideoKling21Start,
		domain.CommandMenuVideoSeedance1Lite,
		domain.CommandMenuVideoSeedance1Pro,
		domain.CommandMenuVideoHaiuo02Standard,
		domain.CommandMenuVideoHaiuo02Fast,
	} {
		spec, ok := videoModeForCommand(command)
		if ok && spec.Mode == mode {
			return spec, true
		}
	}
	return videoModeSpec{}, false
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
	Message               *vkMessage     `json:"message"`
	FromID                int64          `json:"from_id"`
	PeerID                int64          `json:"peer_id"`
	Text                  string         `json:"text"`
	Payload               string         `json:"payload"`
	Ref                   string         `json:"ref"`
	RefSource             string         `json:"ref_source"`
	ConversationMessageID int64          `json:"conversation_message_id"`
	Attachments           []vkAttachment `json:"attachments"`
}

type vkMessage struct {
	FromID                int64          `json:"from_id"`
	PeerID                int64          `json:"peer_id"`
	Text                  string         `json:"text"`
	Payload               string         `json:"payload"`
	Ref                   string         `json:"ref"`
	RefSource             string         `json:"ref_source"`
	ConversationMessageID int64          `json:"conversation_message_id"`
	Attachments           []vkAttachment `json:"attachments"`
}

type messageEvent struct {
	UserID                int64           `json:"user_id"`
	PeerID                int64           `json:"peer_id"`
	EventID               string          `json:"event_id"`
	Payload               json.RawMessage `json:"payload"`
	ConversationMessageID int64           `json:"conversation_message_id"`
}

func (m messageEvent) payloadString() string {
	raw := strings.TrimSpace(string(m.Payload))
	if raw == "" || raw == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(m.Payload, &s); err == nil {
		return s
	}
	return raw
}

type vkAttachment struct {
	Type    string     `json:"type"`
	Sticker *vkSticker `json:"sticker,omitempty"`
}

type vkSticker struct {
	StickerID int64  `json:"sticker_id"`
	ProductID int64  `json:"product_id"`
	Emoji     string `json:"emoji"`
}

func (m messageNew) resolve() (fromID, peerID, conversationMessageID int64, text, payload, ref string) {
	if m.Message != nil {
		return m.Message.FromID, m.Message.PeerID, m.Message.ConversationMessageID, normalizedMessageText(m.Message.Text, m.Message.Attachments), m.Message.Payload, m.Message.Ref
	}
	return m.FromID, m.PeerID, m.ConversationMessageID, normalizedMessageText(m.Text, m.Attachments), m.Payload, m.Ref
}

func normalizedMessageText(text string, attachments []vkAttachment) string {
	if strings.TrimSpace(text) != "" {
		return text
	}
	for _, attachment := range attachments {
		if attachment.Type != "sticker" || attachment.Sticker == nil {
			continue
		}
		sticker := attachment.Sticker
		prompt := fmt.Sprintf("Пользователь отправил VK-стикер (sticker_id=%d, product_id=%d). Ответь коротко и дружелюбно.", sticker.StickerID, sticker.ProductID)
		if sticker.Emoji != "" {
			prompt = fmt.Sprintf("Пользователь отправил VK-стикер %q (sticker_id=%d, product_id=%d). Ответь коротко и дружелюбно.", sticker.Emoji, sticker.StickerID, sticker.ProductID)
		}
		return prompt
	}
	return text
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
	case "message_event":
		if err := h.handleMessageEvent(r.Context(), cb, body); err != nil {
			h.logger.Error("vk message_event processing failed",
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

func (h *Handler) handleMessageNew(ctx context.Context, cb callback, rawBody []byte) (err error) {
	eventID := cb.EventID

	var obj messageNew
	if len(cb.Object) > 0 {
		if err := json.Unmarshal(cb.Object, &obj); err != nil {
			return fmt.Errorf("decode object: %w", err)
		}
	}
	fromID, peerID, conversationMessageID, text, payload, ref := obj.resolve()
	if fromID == 0 {
		return errors.New("message has no from_id")
	}
	if eventID == "" {
		// Fall back to a stable synthetic id when VK omits event_id.
		if conversationMessageID > 0 {
			eventID = fmt.Sprintf("conversation_message:%d:%d:%d", peerID, fromID, conversationMessageID)
		} else {
			eventID = fmt.Sprintf("%d:%d:%x", peerID, fromID, hashText(text))
		}
	}

	idemKey := fmt.Sprintf("vk_event:%d:%s", cb.GroupID, eventID)
	ctx, span := tracing.Start(ctx, "vk.message_new",
		attribute.Int64("vk.group_id", cb.GroupID),
		attribute.String("vk.event_id", eventID),
		attribute.Int64("vk.peer_id", peerID),
		attribute.Int64("vk.user_id", fromID),
		tracing.CorrelationAttr(idemKey),
	)
	defer func() {
		tracing.RecordError(span, err)
		span.End()
	}()

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

	if err := h.process(ctx, cb, rawBody, eventID, idemKey, fromID, peerID, text, payload, ref, false); err != nil {
		_ = h.deps.Idempotency.MarkFailed(ctx, idemKey)
		return err
	}
	return nil
}

func (h *Handler) handleMessageEvent(ctx context.Context, cb callback, rawBody []byte) (err error) {
	var obj messageEvent
	if len(cb.Object) > 0 {
		if err := json.Unmarshal(cb.Object, &obj); err != nil {
			return fmt.Errorf("decode object: %w", err)
		}
	}
	fromID, peerID, payload := obj.UserID, obj.PeerID, obj.payloadString()
	if fromID == 0 {
		return errors.New("message_event has no user_id")
	}
	if peerID == 0 {
		return errors.New("message_event has no peer_id")
	}
	eventID := cb.EventID
	if eventID == "" {
		eventID = obj.EventID
	}
	if eventID == "" {
		eventID = fmt.Sprintf("message_event:%d:%d:%x", peerID, fromID, hashText(payload))
	}

	idemKey := fmt.Sprintf("vk_event:%d:%s", cb.GroupID, eventID)
	ctx, span := tracing.Start(ctx, "vk.message_event",
		attribute.Int64("vk.group_id", cb.GroupID),
		attribute.String("vk.event_id", eventID),
		attribute.Int64("vk.peer_id", peerID),
		attribute.Int64("vk.user_id", fromID),
		tracing.CorrelationAttr(idemKey),
	)
	defer func() {
		tracing.RecordError(span, err)
		span.End()
	}()

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
		return nil
	}

	answerEventID := obj.EventID
	if answerEventID == "" {
		answerEventID = eventID
	}
	h.answerMessageEvent(ctx, answerEventID, fromID, peerID)

	if err := h.process(ctx, cb, rawBody, eventID, idemKey, fromID, peerID, "", payload, "", true); err != nil {
		_ = h.deps.Idempotency.MarkFailed(ctx, idemKey)
		return err
	}
	return nil
}

func (h *Handler) answerMessageEvent(ctx context.Context, eventID string, userID, peerID int64) {
	if eventID == "" || h.deps.Control == nil {
		return
	}
	if err := h.deps.Control.AnswerMessageEvent(ctx, eventID, userID, peerID); err != nil {
		h.logger.Warn("vk message_event answer failed",
			slog.Int64("peer_id", peerID),
			slog.String("error", err.Error()))
	}
}

// process runs the InboundEvent -> User -> Command -> Job flow.
func (h *Handler) process(ctx context.Context, cb callback, rawBody []byte, eventID, idemKey string, fromID, peerID int64, text, payload, ref string, controlOnly bool) error {
	metrics.ObserveProductEvent("vk_bot", "inbound", "received", "unknown", "unknown", cb.Type)

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
	if err := h.deps.Inbound.Create(ctx, inbound); err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			return fmt.Errorf("save inbound: %w", err)
		}
		existing, getErr := h.deps.Inbound.GetByIdempotencyKey(ctx, idemKey)
		if getErr != nil {
			return fmt.Errorf("load existing inbound: %w", getErr)
		}
		inbound = existing
	}

	// User: get or create, granting the starting balance to brand-new users.
	user, err := h.ensureUser(ctx, fromID)
	if err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}

	// Command: classify the message into a normalized command.
	parsed := h.deps.Router.Parse(text)
	controlFromPayload := false
	topUpProductCode := ""
	topUpAction := ""
	if control, ok := controlPayloadFromPayload(payload); ok {
		parsed = commandrouter.Result{Type: domain.CommandType(control.Command)}
		topUpProductCode = strings.TrimSpace(control.ProductCode)
		topUpAction = strings.TrimSpace(control.Action)
		controlFromPayload = true
	} else if controlOnly {
		parsed = commandrouter.Result{Type: domain.CommandUnknown}
	}
	activateReferral := shouldActivateReferralOnVKCommand(parsed.Type)
	if isMenuCommand(parsed.Type) && !h.menuCommandEnabled(parsed.Type) {
		parsed = commandrouter.Result{Type: domain.CommandShowMenu}
		topUpProductCode = ""
		topUpAction = ""
		controlFromPayload = true
	}

	if code := h.referralCodeFromEvent(ref, parsed); code != "" {
		if err := h.applyReferralCode(ctx, user.ID, code); err != nil {
			return fmt.Errorf("apply referral code: %w", err)
		}
	}
	if activateReferral {
		if err := h.activateReferral(ctx, user.ID); err != nil {
			return fmt.Errorf("activate referral: %w", err)
		}
	}

	if h.shouldRoutePhotoText(ctx, peerID, parsed, controlFromPayload, controlOnly) {
		parsed = commandrouter.Result{
			Type:      domain.CommandImageGenerate,
			Operation: domain.OperationImageGenerate,
			Modality:  domain.ModalityImage,
			Prompt:    strings.TrimSpace(text),
		}
	}
	videoSpec := videoModeSpec{}
	videoTextJob := false
	if spec, ok := h.shouldRouteVideoText(ctx, peerID, parsed, controlFromPayload, controlOnly); ok {
		videoSpec = spec
		videoTextJob = true
		parsed = commandrouter.Result{
			Type:      domain.CommandVideoGenerate,
			Operation: domain.OperationVideoGenerate,
			Modality:  domain.ModalityVideo,
			Prompt:    strings.TrimSpace(text),
		}
	}

	textAskEnabled := false
	if parsed.Type == domain.CommandTextAsk {
		textAskEnabled = h.textAskEnabled(ctx, peerID)
	}
	if shouldForceOnboarding(user, parsed, controlFromPayload, controlOnly, textAskEnabled) {
		parsed = commandrouter.Result{Type: domain.CommandStart}
	}

	unroutedText := false
	if parsed.Type == domain.CommandTextAsk && !textAskEnabled {
		unroutedText = true
		parsed = commandrouter.Result{Type: domain.CommandUnknown}
	}

	if h.deps.AntiSpam != nil {
		decision, err := h.deps.AntiSpam.Check(ctx, antispam.CheckInput{
			User:        user,
			VKUserID:    fromID,
			CommandType: parsed.Type,
			Operation:   parsed.Operation,
			CreatesJob:  parsed.CreatesJob(),
		})
		if err != nil {
			h.logger.Warn("vk anti-spam check failed; allowing event",
				slog.Int64("vk_user_id", fromID),
				slog.String("error", err.Error()))
		} else if !decision.Allowed {
			metrics.ObserveProductEvent("vk_bot", "command", "antispam", productCommandOperation(parsed), productCommandModality(parsed), "denied")
			if err := h.sendAntiSpamResponse(ctx, idemKey, peerID, decision); err != nil {
				return fmt.Errorf("send anti-spam response: %w", err)
			}
			if err := h.deps.Inbound.SetStatus(ctx, inbound.ID, domain.InboundProcessed); err != nil {
				return fmt.Errorf("mark inbound processed: %w", err)
			}
			if err := h.deps.Idempotency.MarkCompleted(ctx, idemKey, inbound.ID); err != nil {
				return fmt.Errorf("mark idempotency completed: %w", err)
			}
			return nil
		}
	}
	metrics.ObserveProductEvent("vk_bot", "command", "parsed", productCommandOperation(parsed), productCommandModality(parsed), productCommandResult(parsed, controlOnly, controlFromPayload))

	photoTextJob := parsed.Type == domain.CommandImageGenerate && h.photoTextDialogActive(ctx, peerID)

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

	switch {
	case controlOnly && parsed.Type == domain.CommandShowMenu && controlFromPayload && !h.hasActiveMenu(peerID):
		// A stale inline "Back/show menu" callback can arrive after a GPT answer
		// has already cleared the active menu. Acknowledge it, but do not create
		// a new welcome/menu message; the persistent lower text button remains
		// the explicit way to open a fresh menu at the bottom of the chat.
		h.clearDialogMode(ctx, peerID)
	case unroutedText:
		if err := h.sendUnroutedTextResponse(ctx, idemKey, peerID); err != nil {
			return fmt.Errorf("send unrouted text response: %w", err)
		}
	case parsed.Type == domain.CommandTopUp && topUpAction == topUpActionNewPayment:
		if err := h.sendTopUpCatalog(ctx, idemKey, peerID, true, controlFromPayload); err != nil {
			return fmt.Errorf("send top-up catalog: %w", err)
		}
		h.clearDialogMode(ctx, peerID)
	case topUpProductCode != "":
		topUpForceNew := topUpAction == topUpActionNewPayment
		if err := h.createAndSendTopUpPayment(ctx, cb.GroupID, eventID, idemKey, peerID, user, topUpProductCode, topUpForceNew); err != nil {
			return fmt.Errorf("create top-up payment: %w", err)
		}
	case shouldSendControlResponse(parsed.Type):
		if shouldRepairPersistentKeyboard(parsed.Type, controlFromPayload, controlOnly) {
			if err := h.sendPersistentMenuButton(ctx, idemKey, peerID); err != nil {
				return fmt.Errorf("send persistent menu repair: %w", err)
			}
		}
		allowEdit := controlFromPayload && (parsed.Type != domain.CommandShowMenu || controlOnly)
		if err := h.sendControlResponse(ctx, parsed.Type, idemKey, cb.GroupID, peerID, user, allowEdit); err != nil {
			return fmt.Errorf("send control response: %w", err)
		}
		if parsed.Type == domain.CommandMenuText {
			h.setDialogMode(ctx, peerID, dialogModeGPT)
		} else if parsed.Type == domain.CommandMenuImage || parsed.Type == domain.CommandMenuImageText {
			h.setDialogMode(ctx, peerID, dialogModePhotoText)
		} else if spec, ok := videoModeForCommand(parsed.Type); ok {
			h.setDialogMode(ctx, peerID, spec.Mode)
		} else {
			h.clearDialogMode(ctx, peerID)
		}
	default:
		h.clearActiveMenu(peerID)
		if parsed.Type != domain.CommandTextAsk && !photoTextJob && !videoTextJob {
			h.clearDialogMode(ctx, peerID)
		}
	}

	// Job: only commands that map to an AI operation become jobs. Control
	// commands (balance/status/cancel/help) are recorded but produce no job.
	if parsed.CreatesJob() {
		placeholderID := int64(0)
		if parsed.Type == domain.CommandTextAsk && h.gptDialogActive(ctx, peerID) {
			placeholderID = h.sendGPTPendingMessage(ctx, idemKey, peerID)
		} else if photoTextJob {
			placeholderID = h.sendPhotoPendingMessage(ctx, idemKey, peerID)
		} else if videoTextJob {
			placeholderID = h.sendVideoPendingMessage(ctx, idemKey, peerID)
		}

		// Carry the user's prompt on the job so workers can render it and the
		// output-moderation stage has the request text to evaluate.
		jp := jobParams{
			Prompt:                 parsed.Prompt,
			VKPlaceholderMessageID: placeholderID,
		}
		if videoTextJob {
			jp.ModelID = videoSpec.ModelID
			jp.ModelName = videoSpec.ModelName
			jp.Provider = string(videoSpec.Provider)
			jp.ModelCode = videoSpec.ModelCode
			jp.DurationSec = videoSpec.DurationSec
		}
		params, _ := json.Marshal(jp)
		metrics.ObserveProductPromptLength("vk_bot", string(parsed.Operation), string(parsed.Modality), parsed.Prompt)
		job, err := h.deps.Orchestrator.CreateJob(ctx, joborchestrator.CreateJobInput{
			UserID:         user.ID,
			Source:         "vk_bot",
			VKPeerID:       peerID,
			CommandID:      cmd.ID,
			Operation:      parsed.Operation,
			Modality:       parsed.Modality,
			IdempotencyKey: "vk_job:" + strconv.FormatInt(cb.GroupID, 10) + ":" + eventID,
			CorrelationID:  idemKey,
			Params:         params,
		})
		switch {
		case err == nil:
			resourceID = job.ID
		case errors.Is(err, domain.ErrInsufficientCredits):
			// Expected business outcome: job is parked in awaiting_payment.
			resourceID = job.ID
			h.editGPTPendingMessage(ctx, peerID, placeholderID, "Недостаточно средств для запроса. Пополните баланс или выберите другой режим.")
		default:
			h.editGPTPendingMessage(ctx, peerID, placeholderID, "Не удалось поставить запрос в очередь. Попробуйте позже.")
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

func productCommandOperation(parsed commandrouter.Result) string {
	if parsed.Operation != "" {
		return string(parsed.Operation)
	}
	if parsed.Type != "" {
		return string(parsed.Type)
	}
	return "unknown"
}

func productCommandModality(parsed commandrouter.Result) string {
	if parsed.Modality != "" {
		return string(parsed.Modality)
	}
	if parsed.CreatesJob() {
		return "unknown"
	}
	return "control"
}

func productCommandResult(parsed commandrouter.Result, controlOnly, controlFromPayload bool) string {
	if parsed.CreatesJob() {
		return "job_candidate"
	}
	if controlOnly {
		return "control_callback"
	}
	if controlFromPayload {
		return "control_payload"
	}
	if parsed.Type == domain.CommandUnknown {
		return "unknown"
	}
	return "control"
}

func (h *Handler) shouldRoutePhotoText(ctx context.Context, peerID int64, parsed commandrouter.Result, controlFromPayload, controlOnly bool) bool {
	if controlFromPayload || controlOnly || parsed.Type != domain.CommandTextAsk {
		return false
	}
	if strings.TrimSpace(parsed.Prompt) == "" {
		return false
	}
	return h.photoTextDialogActive(ctx, peerID)
}

func (h *Handler) shouldRouteVideoText(ctx context.Context, peerID int64, parsed commandrouter.Result, controlFromPayload, controlOnly bool) (videoModeSpec, bool) {
	if controlFromPayload || controlOnly || parsed.Type != domain.CommandTextAsk {
		return videoModeSpec{}, false
	}
	if strings.TrimSpace(parsed.Prompt) == "" {
		return videoModeSpec{}, false
	}
	mode, ok := h.getDialogMode(ctx, peerID)
	if !ok {
		return videoModeSpec{}, false
	}
	return videoModeFromDialogMode(mode)
}

func (h *Handler) createAndSendTopUpPayment(ctx context.Context, groupID int64, eventID, idemKey string, peerID int64, user *domain.User, productCode string, forceNew bool) error {
	email := strings.TrimSpace(h.cfg.TopUpReceiptEmail)
	phone := strings.TrimSpace(h.cfg.TopUpReceiptPhone)
	if email == "" && phone == "" {
		return h.sendTopUpNotice(ctx, idemKey, peerID, "Платежи временно недоступны: не настроены данные для чека.")
	}
	if h.deps.Payment == nil {
		return h.sendTopUpNotice(ctx, idemKey, peerID, "Платежи пока недоступны. Попробуйте позже.")
	}
	if !forceNew {
		if active, ok, err := h.activeTopUpIntent(ctx, user.ID); err != nil {
			return fmt.Errorf("load active top-up intent: %w", err)
		} else if ok {
			activeProductCode := paymentIntentProductCode(active)
			if activeProductCode != "" && activeProductCode != productCode {
				forceNew = true
			}
		}
	}
	result, err := h.deps.Payment.CreateIntent(ctx, paymentservice.CreateIntentInput{
		UserID:         user.ID,
		ProductCode:    productCode,
		ReceiptEmail:   email,
		ReceiptPhone:   phone,
		IdempotencyKey: "vk_payment:" + strconv.FormatInt(groupID, 10) + ":" + eventID,
		ReturnURL:      h.cfg.TopUpReturnURL,
		Source:         "vk_bot",
		ForceNew:       forceNew,
	})
	if err != nil {
		if errors.Is(err, paymentservice.ErrReceiptContactRequired) || errors.Is(err, paymentservice.ErrInvalidInput) {
			return h.sendTopUpNotice(ctx, idemKey, peerID, "Не удалось создать платеж. Попробуйте позже.")
		}
		if errors.Is(err, domain.ErrNotFound) {
			h.clearDialogMode(ctx, peerID)
			return h.sendTopUpNotice(ctx, idemKey, peerID, "Этот пакет пополнения уже недоступен. Откройте меню пополнения заново.")
		}
		return err
	}
	if result.Intent == nil || strings.TrimSpace(result.Intent.ConfirmationURL) == "" {
		return h.sendTopUpNotice(ctx, idemKey, peerID, "Платеж создан, но ссылка на оплату пока недоступна. Попробуйте позже.")
	}
	h.clearDialogMode(ctx, peerID)
	return h.sendTopUpPaymentLink(ctx, idemKey, peerID, result.Intent)
}

func paymentIntentProductCode(intent *domain.PaymentIntent) string {
	if intent == nil || len(intent.Metadata) == 0 {
		return ""
	}
	var metadata struct {
		ProductCode string `json:"product_code"`
	}
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.ProductCode)
}

func shouldForceOnboarding(user *domain.User, parsed commandrouter.Result, controlFromPayload, controlOnly, textAskEnabled bool) bool {
	if user == nil || !user.WelcomeNameSentAt.IsZero() || controlFromPayload || controlOnly {
		return false
	}
	switch parsed.Type {
	case domain.CommandTextAsk:
		return !textAskEnabled
	case domain.CommandUnknown, domain.CommandShowMenu:
		return true
	default:
		return false
	}
}

func shouldRepairPersistentKeyboard(t domain.CommandType, controlFromPayload, controlOnly bool) bool {
	return t == domain.CommandShowMenu && !controlFromPayload && !controlOnly
}

func shouldActivateReferralOnVKCommand(t domain.CommandType) bool {
	switch t {
	case domain.CommandStart, domain.CommandShowMenu:
		return true
	default:
		return false
	}
}

func (h *Handler) textAskEnabled(ctx context.Context, peerID int64) bool {
	if h.cfg.UnroutedTextMode == unroutedTextModeGPT {
		return true
	}
	return h.gptDialogActive(ctx, peerID)
}

func (h *Handler) sendAntiSpamResponse(ctx context.Context, idemKey string, peerID int64, decision antispam.Decision) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk anti-spam response skipped because VK_ACCESS_TOKEN is not configured",
			slog.String("decision", string(decision.Kind)))
		return nil
	}
	if decision.Message == "" {
		return nil
	}
	msg := vkdelivery.Message{Text: decision.Message}
	randomID := vkdelivery.DeterministicRandomID("vk_control_antispam:" + idemKey)
	_, err := h.sendControlMessage(ctx, domain.CommandShowMenu, peerID, randomID, msg)
	return err
}

func (h *Handler) gptDialogActive(ctx context.Context, peerID int64) bool {
	mode, ok := h.getDialogMode(ctx, peerID)
	return ok && mode == dialogModeGPT
}

func (h *Handler) photoTextDialogActive(ctx context.Context, peerID int64) bool {
	mode, ok := h.getDialogMode(ctx, peerID)
	return ok && mode == dialogModePhotoText
}

func (h *Handler) getDialogMode(ctx context.Context, peerID int64) (dialogMode, bool) {
	h.modeMu.Lock()
	mode, ok := h.dialogModes[peerID]
	h.modeMu.Unlock()
	if ok {
		return mode, true
	}
	if h.deps.DialogState == nil {
		return "", false
	}
	persistedMode, ok, err := h.deps.DialogState.Get(ctx, peerID)
	if err != nil {
		h.logger.Warn("vk dialog mode lookup failed",
			slog.Int64("peer_id", peerID),
			slog.String("error", err.Error()))
		return "", false
	}
	if !ok {
		return "", false
	}
	mode = dialogMode(persistedMode)
	h.modeMu.Lock()
	h.dialogModes[peerID] = mode
	h.modeMu.Unlock()
	return mode, true
}

func (h *Handler) setDialogMode(ctx context.Context, peerID int64, mode dialogMode) {
	h.modeMu.Lock()
	h.dialogModes[peerID] = mode
	h.modeMu.Unlock()
	if h.deps.DialogState == nil {
		return
	}
	if err := h.deps.DialogState.Set(ctx, peerID, string(mode)); err != nil {
		h.logger.Warn("vk dialog mode persist failed",
			slog.Int64("peer_id", peerID),
			slog.String("mode", string(mode)),
			slog.String("error", err.Error()))
	}
}

func (h *Handler) clearDialogMode(ctx context.Context, peerID int64) {
	h.modeMu.Lock()
	delete(h.dialogModes, peerID)
	h.modeMu.Unlock()
	if h.deps.DialogState == nil {
		return
	}
	if err := h.deps.DialogState.Clear(ctx, peerID); err != nil {
		h.logger.Warn("vk dialog mode clear failed",
			slog.Int64("peer_id", peerID),
			slog.String("error", err.Error()))
	}
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

func (h *Handler) referralCodeFromEvent(ref string, parsed commandrouter.Result) string {
	if code := referralCodeFromRaw(ref); code != "" {
		return code
	}
	if parsed.Type == domain.CommandStart {
		return referralCodeFromRaw(parsed.Arg)
	}
	return ""
}

func referralCodeFromRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil {
		if ref := u.Query().Get("ref"); ref != "" {
			return referralservice.NormalizeCode(ref)
		}
		if ref := u.Query().Get("start"); ref != "" {
			return referralservice.NormalizeCode(ref)
		}
	}
	if strings.Contains(raw, "=") && !strings.Contains(raw, " ") {
		if values, err := url.ParseQuery(raw); err == nil {
			if ref := values.Get("ref"); ref != "" {
				return referralservice.NormalizeCode(ref)
			}
		}
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	return referralservice.NormalizeCode(fields[0])
}

func (h *Handler) applyReferralCode(ctx context.Context, userID uuid.UUID, code string) error {
	if h.deps.Referrals == nil {
		return nil
	}
	result, err := h.deps.Referrals.Apply(ctx, referralservice.ApplyInput{
		Code:           code,
		ReferredUserID: userID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		return err
	}
	if result.Applied {
		h.logger.Info("vk referral applied")
	}
	return nil
}

func (h *Handler) activateReferral(ctx context.Context, userID uuid.UUID) error {
	if h.deps.Referrals == nil {
		return nil
	}
	result, err := h.deps.Referrals.Activate(ctx, referralservice.ActivateInput{
		ReferredUserID: userID,
		Source:         domain.ReferralSourceVKBot,
	})
	if err != nil {
		return err
	}
	if result.Rewarded {
		h.logger.Info("vk referral activated")
	}
	return nil
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
