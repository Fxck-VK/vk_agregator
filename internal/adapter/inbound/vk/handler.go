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
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/logging"
	"vk-ai-aggregator/internal/platform/metrics"
	"vk-ai-aggregator/internal/platform/tracing"
	"vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
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
	// ImageModels are public product model aliases already filtered by server
	// feature flags and provider readiness. VK buttons may carry only these
	// aliases, never provider model ids or prices.
	ImageModels []productcatalog.ImageModel
	// VideoRoutes are public product route aliases already filtered by server
	// feature flags and provider readiness. VK buttons may carry only these
	// aliases and public options, never provider ids, model codes or prices.
	VideoRoutes []productcatalog.VideoRoute
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
	// TopUpPaymentRedirectBaseURL is the public API base used to hide provider
	// confirmation URLs behind a server-owned redirect endpoint.
	TopUpPaymentRedirectBaseURL string
	// TopUpStatusEditEnabled stores the VK payment message id so provider
	// webhooks can edit it after a final payment status.
	TopUpStatusEditEnabled bool
	// ReferenceUploadsDisabled is an operator kill switch for VK photo inputs.
	ReferenceUploadsDisabled bool
	ArtifactBucket           string
	MaxUploadBytes           int64
	MaxUploadImageWidth      int
	MaxUploadImageHeight     int
	MaxUploadImagePixels     int64
	// LocalUIStateTTL bounds best-effort process-local UI caches such as the
	// last editable menu message and dialog-mode fallback cache.
	LocalUIStateTTL time.Duration
	// LocalUIStateMaxEntries caps each process-local UI cache by peer count.
	LocalUIStateMaxEntries int
}

// MenuFeatureFlags allows deployments to hide VK menu buttons without deleting
// the menu screens or changing command parsing.
type MenuFeatureFlags struct {
	DisabledCommands map[domain.CommandType]bool
	EnabledCommands  map[domain.CommandType]bool
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
	Idempotency    domain.IdempotencyRepository
	Inbound        domain.InboundEventRepository
	Users          domain.UserRepository
	Jobs           domain.JobRepository
	Commands       domain.CommandRepository
	Billing        *billingservice.Service
	Payment        *paymentservice.Service
	Referrals      ReferralService
	Orchestrator   *joborchestrator.Orchestrator
	PricingCatalog *pricingcatalog.Catalog
	Router         *commandrouter.Router
	Control        vkdelivery.ControlClient
	Profile        vkdelivery.UserProfileClient
	DialogState    DialogState
	AntiSpam       AntiSpam
	Artifacts      domain.ArtifactRepository
	Objects        ObjectStore
	Downloader     Downloader
	Logger         *slog.Logger
}

// ObjectStore persists input artifact bytes.
type ObjectStore interface {
	Put(ctx context.Context, bucket, key string, data []byte, contentType string) error
}

// Downloader fetches VK CDN media without exposing source URLs in logs.
type Downloader interface {
	Download(ctx context.Context, url string) ([]byte, string, error)
}

// Handler serves the POST /webhooks/vk endpoint.
type Handler struct {
	cfg    Config
	deps   Deps
	logger *slog.Logger

	menuMu      sync.Mutex
	activeMenus map[int64]activeMenuMessage

	modeMu      sync.Mutex
	dialogModes map[int64]cachedDialogMode
}

// NewHandler builds a VK callback handler.
func NewHandler(cfg Config, deps Deps) *Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	cfg.MenuButtonMode = normalizeMenuButtonMode(cfg.MenuButtonMode)
	cfg.UnroutedTextMode = normalizeUnroutedTextMode(cfg.UnroutedTextMode)
	cfg.LocalUIStateTTL = normalizeLocalUIStateTTL(cfg.LocalUIStateTTL)
	cfg.LocalUIStateMaxEntries = normalizeLocalUIStateMaxEntries(cfg.LocalUIStateMaxEntries)
	return &Handler{
		cfg:         cfg,
		deps:        deps,
		logger:      logger,
		activeMenus: map[int64]activeMenuMessage{},
		dialogModes: map[int64]cachedDialogMode{},
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

const (
	defaultLocalUIStateTTL        = time.Hour
	defaultLocalUIStateMaxEntries = 10000
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

func normalizeLocalUIStateTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultLocalUIStateTTL
	}
	return ttl
}

func normalizeLocalUIStateMaxEntries(maxEntries int) int {
	if maxEntries <= 0 {
		return defaultLocalUIStateMaxEntries
	}
	return maxEntries
}

type activeMenuMessage struct {
	MessageID int64
	ExpiresAt time.Time
}

type cachedDialogMode struct {
	Mode      dialogMode
	ExpiresAt time.Time
}

type dialogMode string

const (
	dialogModeGPT               dialogMode = "gpt"
	dialogModePhotoText         dialogMode = "photo_text"
	dialogModePhotoNanoBanana2  dialogMode = "photo_nano_banana_2"
	dialogModePhotoGPTImage2    dialogMode = "photo_gpt_image_2"
	dialogModePhotoSelectPrefix            = "photo_select:"
	dialogModePhotoPromptPrefix            = "photo_prompt:"
	dialogModeVideoRoutePrefix             = "video_route:"
)

const topUpActionNewPayment = "new_payment"

type jobParams struct {
	Prompt                 string      `json:"prompt"`
	ModelID                string      `json:"model_id,omitempty"`
	ModelName              string      `json:"model_name,omitempty"`
	VideoRouteAlias        string      `json:"video_route_alias,omitempty"`
	Provider               string      `json:"provider,omitempty"`
	ModelCode              string      `json:"model_code,omitempty"`
	Size                   string      `json:"size,omitempty"`
	Resolution             string      `json:"resolution,omitempty"`
	ImageQuality           string      `json:"image_quality,omitempty"`
	DurationSec            int         `json:"duration_sec,omitempty"`
	AspectRatio            string      `json:"aspect_ratio,omitempty"`
	ReferenceArtifactIDs   []uuid.UUID `json:"reference_artifact_ids,omitempty"`
	VKPlaceholderMessageID int64       `json:"vk_placeholder_message_id,omitempty"`
}

type videoModeSpec struct {
	Mode                   dialogMode
	ModelName              string
	VideoRouteAlias        domain.VideoRouteAlias
	DurationSec            int
	AllowedDurationsSec    []int
	Resolution             string
	RequiresStartImage     bool
	SupportsReferenceImage bool
	MaxReferenceImages     int
	AllowedAspectRatios    []string
}

func videoModeForCommand(t domain.CommandType) (videoModeSpec, bool) {
	switch t {
	case domain.CommandMenuVideoSora2Start:
		return videoRouteMode("video:runway_gen4_turbo", "Runway Gen-4 Turbo", domain.VideoRouteRunwayGen4Turbo, 5, []int{3, 5, 10}, true, 1, "16:9", "9:16", "4:3", "3:4", "1:1", "21:9"), true
	case domain.CommandMenuVideoKling21Start:
		return videoRouteMode("video:kling_o3_standard", "Kling O3 Standard", domain.VideoRouteKlingO3Standard, 5, []int{5, 10}, false, 1, "16:9", "9:16", "1:1"), true
	case domain.CommandMenuVideoSeedance1Lite:
		return videoRouteMode("video:seedance_2_0_fast", "Seedance 2.0 Fast", domain.VideoRouteSeedance20Fast, 5, []int{5, 10}, false, 4, "16:9", "9:16", "1:1"), true
	case domain.CommandMenuVideoHailuo02Standard:
		return videoRouteMode("video:hailuo_2_3_standard", "Hailuo 2.3 Standard", domain.VideoRouteHailuo23Standard, 6, []int{6, 10}, false, 1), true
	case domain.CommandMenuVideoHailuo02Fast:
		return videoRouteMode("video:hailuo_2_3_fast", "Hailuo 2.3 Fast", domain.VideoRouteHailuo23Fast, 6, []int{6, 10}, true, 1), true
	default:
		return videoModeSpec{}, false
	}
}

func videoRouteMode(mode dialogMode, name string, alias domain.VideoRouteAlias, durationSec int, allowedDurations []int, requiresStartImage bool, maxReferenceImages int, allowedAspectRatios ...string) videoModeSpec {
	return videoModeSpec{
		Mode:                   mode,
		ModelName:              name,
		VideoRouteAlias:        alias,
		DurationSec:            durationSec,
		AllowedDurationsSec:    append([]int(nil), allowedDurations...),
		Resolution:             defaultVKVideoResolution(alias),
		RequiresStartImage:     requiresStartImage,
		SupportsReferenceImage: true,
		MaxReferenceImages:     maxReferenceImages,
		AllowedAspectRatios:    append([]string(nil), allowedAspectRatios...),
	}
}

func defaultVKVideoResolution(alias domain.VideoRouteAlias) string {
	switch alias {
	case domain.VideoRouteKlingO3Standard,
		domain.VideoRouteRunwayGen4Turbo,
		domain.VideoRouteSeedance20Fast,
		domain.VideoRouteRunwayGen45:
		return pricingcatalog.VideoResolution720p
	case domain.VideoRouteHailuo23Fast,
		domain.VideoRouteHailuo23Standard:
		return pricingcatalog.VideoResolution768p
	default:
		return ""
	}
}

func videoModeFromDialogMode(mode dialogMode) (videoModeSpec, bool) {
	baseMode, durationSec := splitVideoDialogMode(mode)
	for _, command := range []domain.CommandType{
		domain.CommandMenuVideoSora2Start,
		domain.CommandMenuVideoKling21Start,
		domain.CommandMenuVideoSeedance1Lite,
		domain.CommandMenuVideoHailuo02Standard,
		domain.CommandMenuVideoHailuo02Fast,
	} {
		spec, ok := videoModeForCommand(command)
		if ok && spec.Mode == baseMode {
			if spec.supportsDuration(durationSec) {
				spec.DurationSec = durationSec
			}
			return spec, true
		}
	}
	return videoModeSpec{}, false
}

func videoModeFromPublicRoute(route productcatalog.VideoRoute) (videoModeSpec, bool) {
	alias := strings.TrimSpace(route.Alias)
	name := strings.TrimSpace(route.Name)
	if alias == "" || name == "" || !route.Enabled {
		return videoModeSpec{}, false
	}
	durationSec := route.DefaultDurationSec
	if durationSec <= 0 && len(route.AllowedDurationsSec) > 0 {
		durationSec = route.AllowedDurationsSec[0]
	}
	spec := videoModeSpec{
		Mode:                   videoRouteDialogMode(alias),
		ModelName:              name,
		VideoRouteAlias:        domain.VideoRouteAlias(alias),
		DurationSec:            durationSec,
		AllowedDurationsSec:    append([]int(nil), route.AllowedDurationsSec...),
		Resolution:             strings.TrimSpace(route.DefaultResolution),
		RequiresStartImage:     route.RequiresStartImage,
		SupportsReferenceImage: route.SupportsReferenceImage,
		MaxReferenceImages:     route.MaxReferenceImages,
		AllowedAspectRatios:    append([]string(nil), route.AllowedAspectRatios...),
	}
	if !spec.supportsDuration(spec.DurationSec) {
		return videoModeSpec{}, false
	}
	return spec, true
}

func videoRouteDialogMode(alias string) dialogMode {
	return dialogMode(dialogModeVideoRoutePrefix + strings.TrimSpace(alias))
}

func parseVideoRouteDialogMode(mode dialogMode) (string, bool) {
	alias, ok := strings.CutPrefix(string(mode), dialogModeVideoRoutePrefix)
	alias = strings.TrimSpace(alias)
	return alias, ok && alias != ""
}

func (h *Handler) videoModeForRouteAlias(alias string) (videoModeSpec, bool) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return videoModeSpec{}, false
	}
	for _, route := range h.cfg.VideoRoutes {
		if route.Enabled && route.Alias == alias {
			return videoModeFromPublicRoute(route)
		}
	}
	return videoModeSpec{}, false
}

func (h *Handler) videoModeFromDialogMode(mode dialogMode) (videoModeSpec, bool) {
	baseMode, durationSec := splitVideoDialogMode(mode)
	if alias, ok := parseVideoRouteDialogMode(baseMode); ok {
		spec, ok := h.videoModeForRouteAlias(alias)
		if !ok {
			return videoModeSpec{}, false
		}
		if spec.supportsDuration(durationSec) {
			spec.DurationSec = durationSec
		}
		return spec, true
	}
	return videoModeFromDialogMode(mode)
}

func splitVideoDialogMode(mode dialogMode) (dialogMode, int) {
	raw := string(mode)
	idx := strings.LastIndex(raw, ":")
	if idx <= 0 || idx == len(raw)-1 {
		return mode, 0
	}
	durationSec, err := strconv.Atoi(raw[idx+1:])
	if err != nil {
		return mode, 0
	}
	return dialogMode(raw[:idx]), durationSec
}

func (s videoModeSpec) modeWithDuration(durationSec int) dialogMode {
	if !s.supportsDuration(durationSec) || durationSec == s.DurationSec {
		return s.Mode
	}
	return dialogMode(fmt.Sprintf("%s:%d", s.Mode, durationSec))
}

func (s videoModeSpec) supportsDuration(durationSec int) bool {
	if durationSec <= 0 {
		return false
	}
	for _, allowed := range s.AllowedDurationsSec {
		if durationSec == allowed {
			return true
		}
	}
	return false
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
	Out                   int            `json:"out"`
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
	Out                   int            `json:"out"`
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
	Photo   *vkPhoto   `json:"photo,omitempty"`
}

type vkSticker struct {
	StickerID int64  `json:"sticker_id"`
	ProductID int64  `json:"product_id"`
	Emoji     string `json:"emoji"`
}

type vkPhoto struct {
	Sizes []vkPhotoSize `json:"sizes"`
}

type vkPhotoSize struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (m messageNew) resolve() (fromID, peerID, conversationMessageID int64, text, payload, ref string, attachments []vkAttachment) {
	if m.Message != nil {
		return m.Message.FromID, m.Message.PeerID, m.Message.ConversationMessageID, normalizedMessageText(m.Message.Text, m.Message.Attachments), m.Message.Payload, m.Message.Ref, m.Message.Attachments
	}
	return m.FromID, m.PeerID, m.ConversationMessageID, normalizedMessageText(m.Text, m.Attachments), m.Payload, m.Ref, m.Attachments
}

func (m messageNew) outgoing() bool {
	if m.Message != nil {
		return m.Message.Out != 0
	}
	return m.Out != 0
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
				slog.Int64("group_id", cb.GroupID), logging.ErrorAttr(err))
			http.Error(w, "processing error", http.StatusInternalServerError)
			return
		}
		writeText(w, http.StatusOK, "ok")
	case "message_event":
		if err := h.handleMessageEvent(r.Context(), cb, body); err != nil {
			h.logger.Error("vk message_event processing failed",
				slog.Int64("group_id", cb.GroupID), logging.ErrorAttr(err))
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
	fromID, peerID, conversationMessageID, text, payload, ref, attachments := obj.resolve()
	if obj.outgoing() || fromID <= 0 {
		return nil
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

	if err := h.process(ctx, cb, rawBody, eventID, idemKey, fromID, peerID, text, payload, ref, attachments, false); err != nil {
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

	if err := h.process(ctx, cb, rawBody, eventID, idemKey, fromID, peerID, "", payload, "", nil, true); err != nil {
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
			logging.ErrorAttr(err))
	}
}

// process runs the InboundEvent -> User -> Command -> Job flow.
func (h *Handler) process(ctx context.Context, cb callback, rawBody []byte, eventID, idemKey string, fromID, peerID int64, text, payload, ref string, attachments []vkAttachment, controlOnly bool) error {
	metrics.ObserveProductEvent("vk_bot", "inbound", "received", "unknown", "unknown", cb.Type)

	// InboundEvent: persist only minimized metadata for audit and reprocessing.
	inbound := &domain.InboundEvent{
		Source:         "vk",
		EventType:      cb.Type,
		GroupID:        cb.GroupID,
		VKEventID:      eventID,
		PeerID:         peerID,
		VKUserID:       fromID,
		Payload:        redactedVKInboundPayload(cb, eventID, peerID, fromID, controlOnly),
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
	videoDurationSec := 0
	videoRouteAlias := ""
	photoModelID := ""
	photoQuality := ""
	control := controlPayload{}
	if parsedControl, ok := controlPayloadFromPayload(payload); ok {
		control = parsedControl
		parsed = commandrouter.Result{Type: domain.CommandType(control.Command)}
		topUpProductCode = strings.TrimSpace(control.ProductCode)
		topUpAction = strings.TrimSpace(control.Action)
		videoDurationSec = control.DurationSec
		videoRouteAlias = strings.TrimSpace(control.VideoRouteAlias)
		photoModelID = strings.TrimSpace(control.ModelID)
		photoQuality = strings.TrimSpace(control.ImageQuality)
		controlFromPayload = true
	} else if controlOnly {
		parsed = commandrouter.Result{Type: domain.CommandUnknown}
	}
	activateReferral := shouldActivateReferralOnVKCommand(parsed.Type)
	if isMenuCommand(parsed.Type) && (!h.menuCommandEnabled(parsed.Type) || (controlFromPayload && !h.controlPayloadEnabled(control))) {
		parsed = commandrouter.Result{Type: domain.CommandShowMenu}
		topUpProductCode = ""
		topUpAction = ""
		videoDurationSec = 0
		videoRouteAlias = ""
		photoModelID = ""
		photoQuality = ""
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
			blocked, err := h.handleAntiSpamDegraded(ctx, err, inbound.ID, idemKey, peerID, parsed)
			if err != nil {
				return err
			}
			if blocked {
				return nil
			}
		} else if !decision.Allowed {
			metrics.ObserveProductEvent("vk_bot", "command", "antispam", productCommandOperation(parsed), productCommandModality(parsed), "denied")
			if err := h.finishAntiSpamBlockedEvent(ctx, inbound.ID, idemKey, peerID, parsed, decision); err != nil {
				return err
			}
			return nil
		}
	}
	metrics.ObserveProductEvent("vk_bot", "command", "parsed", productCommandOperation(parsed), productCommandModality(parsed), productCommandResult(parsed, controlOnly, controlFromPayload))

	photoSelection, photoTextJob := h.photoSelectionForActiveDialog(ctx, peerID)
	photoTextJob = parsed.Type == domain.CommandImageGenerate && photoTextJob
	videoBlockedMessage := ""
	var videoReferenceIDs []uuid.UUID
	videoAspectRatio := ""

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
	if videoTextJob {
		var ok bool
		videoReferenceIDs, videoAspectRatio, videoBlockedMessage, ok = h.prepareVideoReferenceArtifacts(ctx, user.ID, videoSpec, attachments)
		if !ok {
			if err := h.sendVideoReferenceNotice(ctx, idemKey, peerID, videoBlockedMessage); err != nil {
				return fmt.Errorf("send video reference notice: %w", err)
			}
			if err := h.deps.Inbound.SetStatus(ctx, inbound.ID, domain.InboundProcessed); err != nil {
				return fmt.Errorf("mark inbound processed: %w", err)
			}
			if err := h.deps.Idempotency.MarkCompleted(ctx, idemKey, cmd.ID); err != nil {
				return fmt.Errorf("mark idempotency completed: %w", err)
			}
			return nil
		}
	}

	resourceID := cmd.ID

	switch {
	case unroutedText:
		if err := h.sendUnroutedTextResponse(ctx, idemKey, peerID); err != nil {
			return fmt.Errorf("send unrouted text response: %w", err)
		}
	case topUpProductCode != "":
		topUpForceNew := topUpAction == topUpActionNewPayment
		if err := h.createAndSendTopUpPayment(ctx, cb.GroupID, eventID, idemKey, peerID, user, topUpProductCode, topUpForceNew); err != nil {
			return fmt.Errorf("create top-up payment: %w", err)
		}
	case parsed.Type == domain.CommandTopUp && topUpAction == topUpActionNewPayment:
		if err := h.sendTopUpCatalog(ctx, idemKey, peerID, user, true, controlFromPayload); err != nil {
			return fmt.Errorf("send top-up catalog: %w", err)
		}
		h.clearDialogMode(ctx, peerID)
	case parsed.Type == domain.CommandMenuVideoRouteSelect:
		if err := h.sendVideoDurationSelection(ctx, videoRouteAlias, idemKey, parsed.Type, peerID, controlFromPayload); err != nil {
			return fmt.Errorf("send video duration selection: %w", err)
		}
	case parsed.Type == domain.CommandMenuVideoDurationSelect:
		if err := h.sendVideoPromptForDuration(ctx, videoRouteAlias, videoDurationSec, idemKey, parsed.Type, peerID, controlFromPayload); err != nil {
			return fmt.Errorf("send video prompt instruction: %w", err)
		}
	case parsed.Type == domain.CommandMenuImageSelect:
		if err := h.sendPhotoQualitySelection(ctx, photoModelID, idemKey, parsed.Type, peerID, controlFromPayload); err != nil {
			return fmt.Errorf("send photo quality selection: %w", err)
		}
	case isPhotoModelCommand(parsed.Type):
		modelID, _ := photoModelIDFromCommand(parsed.Type)
		allowEdit := controlFromPayload
		if err := h.sendPhotoQualitySelection(ctx, modelID, idemKey, parsed.Type, peerID, allowEdit); err != nil {
			return fmt.Errorf("send photo quality selection: %w", err)
		}
	case parsed.Type == domain.CommandMenuImageQualitySelect:
		modelID := photoModelID
		if modelID == "" {
			var ok bool
			modelID, ok = h.photoModelIDForQualitySelection(ctx, peerID)
			if !ok {
				h.clearDialogMode(ctx, peerID)
				if err := h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, cb.GroupID, peerID, user, controlFromPayload); err != nil {
					return fmt.Errorf("send photo model selection: %w", err)
				}
				break
			}
		}
		if err := h.sendPhotoPromptForQuality(ctx, modelID, photoQuality, idemKey, parsed.Type, peerID, controlFromPayload); err != nil {
			return fmt.Errorf("send photo prompt instruction: %w", err)
		}
	case isPhotoQualityCommand(parsed.Type):
		modelID, ok := h.photoModelIDForQualitySelection(ctx, peerID)
		if !ok {
			h.clearDialogMode(ctx, peerID)
			if err := h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, cb.GroupID, peerID, user, controlFromPayload); err != nil {
				return fmt.Errorf("send photo model selection: %w", err)
			}
			break
		}
		quality, _ := photoQualityFromCommand(parsed.Type)
		if err := h.sendPhotoPromptForQuality(ctx, modelID, quality, idemKey, parsed.Type, peerID, controlFromPayload); err != nil {
			return fmt.Errorf("send photo prompt instruction: %w", err)
		}
	case parsed.Type == domain.CommandMenuImageBackToQuality:
		modelID, ok := h.photoModelIDForQualityBack(ctx, peerID)
		if !ok {
			h.clearDialogMode(ctx, peerID)
			if err := h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, cb.GroupID, peerID, user, controlFromPayload); err != nil {
				return fmt.Errorf("send photo model selection: %w", err)
			}
			break
		}
		if err := h.sendPhotoQualitySelection(ctx, modelID, idemKey, parsed.Type, peerID, controlFromPayload); err != nil {
			return fmt.Errorf("send photo quality selection: %w", err)
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
		} else if parsed.Type == domain.CommandMenuImage {
			h.clearDialogMode(ctx, peerID)
		} else if spec, ok := videoModeForCommand(parsed.Type); ok {
			h.setDialogMode(ctx, peerID, spec.modeWithDuration(videoDurationSec))
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
		if photoTextJob {
			jp.ModelID = photoSelection.Model.ModelID
			jp.ModelName = photoSelection.Model.ModelName
			jp.Provider = string(photoSelection.Model.Provider)
			jp.ModelCode = photoSelection.Model.ModelCode
			jp.Size = imageSizeForSelection(photoSelection)
			jp.Resolution = photoSelection.Quality
			jp.ImageQuality = photoSelection.Quality
		}
		if videoTextJob {
			jp.ModelName = videoSpec.ModelName
			jp.VideoRouteAlias = string(videoSpec.VideoRouteAlias)
			jp.DurationSec = videoSpec.DurationSec
			jp.Resolution = videoSpec.Resolution
			jp.AspectRatio = videoAspectRatio
			jp.ReferenceArtifactIDs = videoReferenceIDs
		}
		params, _ := json.Marshal(jp)
		pricingSnapshot, err := h.jobPricingSnapshot(parsed.Operation, parsed.Modality, photoSelection, videoSpec)
		if err != nil {
			metrics.ObserveProductEvent("vk_bot", "job", "estimate", string(parsed.Operation), string(parsed.Modality), "error")
			return fmt.Errorf("vk pricing catalog estimate: %w", err)
		}
		costEstimateCredits := pricingSnapshot.InternalCredits
		metrics.ObserveProductPromptLength("vk_bot", string(parsed.Operation), string(parsed.Modality), parsed.Prompt)
		job, err := h.deps.Orchestrator.CreateJob(ctx, joborchestrator.CreateJobInput{
			UserID:              user.ID,
			Source:              "vk_bot",
			VKPeerID:            peerID,
			CommandID:           cmd.ID,
			Operation:           parsed.Operation,
			Modality:            parsed.Modality,
			IdempotencyKey:      "vk_job:" + strconv.FormatInt(cb.GroupID, 10) + ":" + eventID,
			CorrelationID:       idemKey,
			Params:              params,
			InputArtifactIDs:    videoReferenceIDs,
			CostEstimateCredits: costEstimateCredits,
			PricingSnapshot:     pricingSnapshot,
		})
		switch {
		case err == nil:
			resourceID = job.ID
		case errors.Is(err, domain.ErrInsufficientCredits):
			// Expected business outcome: job is parked in awaiting_payment.
			resourceID = job.ID
			h.showInsufficientBalanceMessage(ctx, idemKey, peerID, placeholderID)
		case errors.Is(err, domain.ErrActiveJobLimitExceeded):
			h.editGPTPendingMessage(ctx, peerID, placeholderID, "У вас уже есть видео в обработке\nДождитесь результата или попробуйте позже")
		default:
			h.editGPTPendingMessage(ctx, peerID, placeholderID, "Не удалось поставить запрос в очередь\nПопробуйте позже")
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

func redactedVKInboundPayload(cb callback, eventID string, peerID, fromID int64, controlOnly bool) json.RawMessage {
	summary := struct {
		Redacted     bool   `json:"redacted"`
		Source       string `json:"source"`
		EventType    string `json:"event_type"`
		PayloadClass string `json:"payload_class"`
		ControlOnly  bool   `json:"control_only,omitempty"`
		HasEventID   bool   `json:"has_vk_event_id"`
		HasGroupID   bool   `json:"has_group_id"`
		HasPeerID    bool   `json:"has_peer_id"`
		HasVKUserID  bool   `json:"has_vk_user_id"`
	}{
		Redacted:     true,
		Source:       "vk",
		EventType:    cb.Type,
		PayloadClass: "vk_callback_metadata",
		ControlOnly:  controlOnly,
		HasEventID:   strings.TrimSpace(eventID) != "",
		HasGroupID:   cb.GroupID != 0,
		HasPeerID:    peerID != 0,
		HasVKUserID:  fromID != 0,
	}
	b, err := json.Marshal(summary)
	if err != nil {
		return json.RawMessage(`{"redacted":true,"source":"vk","payload_class":"vk_callback_metadata"}`)
	}
	return json.RawMessage(b)
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
	return h.photoDialogActive(ctx, peerID)
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
	return h.videoModeFromDialogMode(mode)
}

func (h *Handler) sendVideoDurationSelection(ctx context.Context, routeAlias, idemKey string, command domain.CommandType, peerID int64, allowEdit bool) error {
	spec, ok := h.videoModeForRouteAlias(routeAlias)
	if !ok {
		h.clearDialogMode(ctx, peerID)
		return h.sendControlResponse(ctx, domain.CommandMenuVideo, idemKey, 0, peerID, &domain.User{}, allowEdit)
	}
	h.setDialogMode(ctx, peerID, spec.Mode)
	if len(spec.AllowedDurationsSec) == 0 {
		return h.sendVideoPromptInstruction(ctx, spec, idemKey, command, peerID, allowEdit)
	}
	msg := vkdelivery.Message{
		Text:     videoDurationSelectionText(spec),
		Keyboard: h.videoRouteDurationKeyboard(spec),
	}
	return h.deliverVideoControl(ctx, command, idemKey, peerID, msg, allowEdit)
}

func (h *Handler) sendVideoPromptForDuration(ctx context.Context, routeAlias string, durationSec int, idemKey string, command domain.CommandType, peerID int64, allowEdit bool) error {
	spec, ok := h.videoModeForRouteAlias(routeAlias)
	if !ok || !spec.supportsDuration(durationSec) {
		h.clearDialogMode(ctx, peerID)
		return h.sendControlResponse(ctx, domain.CommandMenuVideo, idemKey, 0, peerID, &domain.User{}, allowEdit)
	}
	mode := spec.modeWithDuration(durationSec)
	spec.DurationSec = durationSec
	h.setDialogMode(ctx, peerID, mode)
	return h.sendVideoPromptInstruction(ctx, spec, idemKey, command, peerID, allowEdit)
}

func (h *Handler) sendVideoPromptInstruction(ctx context.Context, spec videoModeSpec, idemKey string, command domain.CommandType, peerID int64, allowEdit bool) error {
	msg := vkdelivery.Message{
		Text:     h.videoPromptInstructionText(spec),
		Keyboard: videoPromptBackKeyboard(),
	}
	return h.deliverVideoControl(ctx, command, idemKey, peerID, msg, allowEdit)
}

func (h *Handler) deliverVideoControl(ctx context.Context, command domain.CommandType, idemKey string, peerID int64, msg vkdelivery.Message, allowEdit bool) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk video control response skipped because VK_ACCESS_TOKEN is not configured",
			slog.String("command_type", string(command)))
		return nil
	}
	h.filterMenuKeyboard(msg.Keyboard)
	if msg.Keyboard != nil && len(msg.Keyboard.Buttons) == 0 {
		msg.Keyboard = nil
	}
	h.applyMenuButtonMode(msg.Keyboard)
	randomID := vkdelivery.DeterministicRandomID("vk_control:" + idemKey + ":" + string(command))
	result, err := h.deliverControlResponse(ctx, command, peerID, randomID, msg, allowEdit)
	if err == nil {
		h.setActiveMenu(peerID, result.MessageID)
	}
	return err
}

func videoDurationSelectionText(spec videoModeSpec) string {
	var b strings.Builder
	b.WriteString(spec.ModelName)
	b.WriteString("\n\nВыберите длительность видео")
	if spec.RequiresStartImage {
		b.WriteString("\n\nДля этой модели нужно стартовое фото")
	} else if spec.SupportsReferenceImage {
		b.WriteString("\n\nМожно написать только текст или прикрепить фото-референс к промту")
	}
	if spec.MaxReferenceImages > 0 {
		b.WriteString(fmt.Sprintf("\nЛимит фото: %d", spec.MaxReferenceImages))
	}
	return b.String()
}

func (h *Handler) videoPromptInstructionText(spec videoModeSpec) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s · %d сек", spec.ModelName, spec.DurationSec))
	if price, ok := h.videoDisplayEstimateCredits(spec); ok {
		b.WriteString(fmt.Sprintf("\n\nЦена: %d ⭐️", price))
	}
	if spec.RequiresStartImage {
		b.WriteString("\n\nПрикрепите стартовое фото и напишите описание видео одним сообщением")
	} else {
		b.WriteString("\n\nНапишите описание видео обычным сообщением")
		if spec.SupportsReferenceImage {
			b.WriteString("\nФото-референс можно прикрепить к этому же сообщению")
		}
	}
	return b.String()
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
	if !h.topUpPaymentRedirectConfigured() {
		balance, err := h.currentBalance(ctx, user)
		if err != nil {
			return err
		}
		return h.sendTopUpNotice(ctx, idemKey, peerID, topUpPaymentUnavailableText(balance))
	}
	returnURL := h.topUpReturnURL(groupID)
	if !forceNew {
		if active, ok, err := h.activeTopUpIntent(ctx, user.ID, returnURL); err != nil {
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
		ReturnURL:      returnURL,
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
	if result.Intent == nil || result.Intent.Status != domain.PaymentIntentWaitingForUser {
		return h.sendTopUpNotice(ctx, idemKey, peerID, "Платеж создан, но ссылка на оплату пока недоступна. Попробуйте позже.")
	}
	h.clearDialogMode(ctx, peerID)
	balance, err := h.currentBalance(ctx, user)
	if err != nil {
		return err
	}
	messageID, err := h.sendTopUpPaymentLink(ctx, idemKey, peerID, balance, result.Intent)
	if err != nil {
		return err
	}
	if h.cfg.TopUpStatusEditEnabled && messageID > 0 {
		if _, err := h.deps.Payment.AttachVKBotPaymentMessage(ctx, paymentservice.AttachVKBotPaymentMessageInput{
			UserID:    user.ID,
			IntentID:  result.Intent.ID,
			VKPeerID:  peerID,
			MessageID: messageID,
		}); err != nil {
			h.logger.Warn("vk top-up payment message tracking failed",
				slog.String("payment_intent_id", result.Intent.ID.String()),
				logging.ErrorAttr(err))
		} else {
			h.logger.Info("vk top-up payment message tracked",
				slog.String("payment_intent_id", result.Intent.ID.String()))
		}
	}
	return nil
}

func (h *Handler) sendPhotoQualitySelection(ctx context.Context, modelID, idemKey string, command domain.CommandType, peerID int64, allowEdit bool) error {
	publicModel, publicOK := h.publicImageModel(modelID)
	if !publicOK {
		h.clearDialogMode(ctx, peerID)
		return h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, 0, peerID, &domain.User{}, allowEdit)
	}
	model, ok := modelcatalog.ResolveMiniAppModel(domain.OperationImageGenerate, modelID)
	if !ok {
		h.clearDialogMode(ctx, peerID)
		return h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, 0, peerID, &domain.User{}, allowEdit)
	}
	h.setDialogMode(ctx, peerID, photoSelectMode(model.ModelID))
	options := h.photoQualityOptions(publicModel)
	if len(options) == 0 {
		selection := photoDialogSelection{Model: model, Quality: strings.TrimSpace(publicModel.DefaultQuality)}
		h.setDialogMode(ctx, peerID, photoPromptMode(model.ModelID, selection.Quality))
		return h.sendPhotoPromptInstruction(ctx, selection, idemKey, command, peerID, allowEdit)
	}
	text := fmt.Sprintf("%s\n\nВыберите качество генерации\nЦена указана в ⭐️ и списывается только после готового результата", model.ModelName)
	msg := vkdelivery.Message{
		Text:     text,
		Keyboard: photoQualityKeyboard(options),
	}
	return h.deliverPhotoControl(ctx, command, idemKey, peerID, msg, allowEdit)
}

func (h *Handler) sendPhotoPromptForQuality(ctx context.Context, modelID, rawQuality, idemKey string, command domain.CommandType, peerID int64, allowEdit bool) error {
	publicModel, ok := h.publicImageModel(modelID)
	if !ok {
		h.clearDialogMode(ctx, peerID)
		return h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, 0, peerID, &domain.User{}, allowEdit)
	}
	quality, ok := h.normalizePublicImageQuality(publicModel, rawQuality)
	if !ok {
		h.clearDialogMode(ctx, peerID)
		return h.sendControlResponse(ctx, domain.CommandMenuImage, idemKey, 0, peerID, &domain.User{}, allowEdit)
	}
	model, ok := modelcatalog.ResolveMiniAppModel(domain.OperationImageGenerate, modelID)
	if !ok {
		h.clearDialogMode(ctx, peerID)
		return fmt.Errorf("resolve photo model: %s", modelID)
	}
	selection := photoDialogSelection{
		Model:   modelcatalog.ApplyImageQuality(model, quality),
		Quality: quality,
	}
	h.setDialogMode(ctx, peerID, photoPromptMode(model.ModelID, quality))
	return h.sendPhotoPromptInstruction(ctx, selection, idemKey, command, peerID, allowEdit)
}

func (h *Handler) sendPhotoPromptInstruction(ctx context.Context, selection photoDialogSelection, idemKey string, command domain.CommandType, peerID int64, allowEdit bool) error {
	price, ok := h.imageDisplayEstimateCredits(selection.Model.ModelID, selection.Quality)
	if !ok {
		return pricingcatalog.ErrPriceNotFound
	}
	text := fmt.Sprintf("%s · %s\n\nЦена: %d ⭐️\n\nВведите описание изображения обычным сообщением", selection.Model.ModelName, selection.Quality, price)
	msg := vkdelivery.Message{
		Text:     text,
		Keyboard: photoPromptKeyboardForCatalog(h.publicImageModelHasQualityOptions(selection.Model.ModelID)),
	}
	return h.deliverPhotoControl(ctx, command, idemKey, peerID, msg, allowEdit)
}

func (h *Handler) deliverPhotoControl(ctx context.Context, command domain.CommandType, idemKey string, peerID int64, msg vkdelivery.Message, allowEdit bool) error {
	if h.deps.Control == nil {
		h.logger.Warn("vk photo control response skipped because VK_ACCESS_TOKEN is not configured",
			slog.String("command_type", string(command)))
		return nil
	}
	h.filterMenuKeyboard(msg.Keyboard)
	if msg.Keyboard != nil && len(msg.Keyboard.Buttons) == 0 {
		msg.Keyboard = nil
	}
	h.applyMenuButtonMode(msg.Keyboard)
	randomID := vkdelivery.DeterministicRandomID("vk_control:" + idemKey + ":" + string(command))
	result, err := h.deliverControlResponse(ctx, command, peerID, randomID, msg, allowEdit)
	if err == nil {
		h.setActiveMenu(peerID, result.MessageID)
	}
	return err
}

func imageSizeForSelection(selection photoDialogSelection) string {
	return "1:1"
}

func (h *Handler) publicImageModel(modelID string) (productcatalog.ImageModel, bool) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return productcatalog.ImageModel{}, false
	}
	for _, model := range h.cfg.ImageModels {
		if model.Enabled && model.ID == modelID {
			return model, true
		}
	}
	return productcatalog.ImageModel{}, false
}

func (h *Handler) publicImageModelEnabled(modelID string) bool {
	_, ok := h.publicImageModel(modelID)
	return ok
}

func (h *Handler) publicImageModelHasQualityOptions(modelID string) bool {
	model, ok := h.publicImageModel(modelID)
	return ok && len(model.QualityOptions) > 0
}

func (h *Handler) normalizePublicImageQuality(model productcatalog.ImageModel, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(model.QualityOptions) == 0 {
		return "", raw == ""
	}
	if raw == "" {
		raw = model.DefaultQuality
	}
	quality, ok := modelcatalog.NormalizeImageQuality(raw)
	if !ok {
		return "", false
	}
	return quality, imageQualityAllowed(quality, model.QualityOptions)
}

func (h *Handler) publicImageQualityAllowed(modelID, rawQuality string) bool {
	model, ok := h.publicImageModel(modelID)
	if !ok {
		return false
	}
	_, ok = h.normalizePublicImageQuality(model, rawQuality)
	return ok
}

func (h *Handler) publicVideoRoute(routeAlias string) (productcatalog.VideoRoute, bool) {
	routeAlias = strings.TrimSpace(routeAlias)
	if routeAlias == "" {
		return productcatalog.VideoRoute{}, false
	}
	for _, route := range h.cfg.VideoRoutes {
		if route.Enabled && route.Alias == routeAlias {
			return route, true
		}
	}
	return productcatalog.VideoRoute{}, false
}

func (h *Handler) publicVideoRouteEnabled(routeAlias string) bool {
	_, ok := h.publicVideoRoute(routeAlias)
	return ok
}

func (h *Handler) publicVideoRouteDurationAllowed(routeAlias string, durationSec int) bool {
	route, ok := h.publicVideoRoute(routeAlias)
	if !ok || durationSec <= 0 {
		return false
	}
	for _, allowed := range route.AllowedDurationsSec {
		if durationSec == allowed {
			return true
		}
	}
	return false
}

func imageQualityAllowed(quality string, options []string) bool {
	for _, option := range options {
		normalized, ok := modelcatalog.NormalizeImageQuality(option)
		if ok && normalized == quality {
			return true
		}
	}
	return false
}

func (h *Handler) imageDisplayEstimateCredits(modelID, quality string) (int64, bool) {
	if h.deps.PricingCatalog == nil {
		return 0, false
	}
	key, ok := imagePricingProductKey(modelID, quality)
	if !ok {
		return 0, false
	}
	credits, err := h.deps.PricingCatalog.DisplayEstimateCredits(key)
	return credits, err == nil && credits > 0
}

func (h *Handler) videoDisplayEstimateCredits(spec videoModeSpec) (int64, bool) {
	if h.deps.PricingCatalog == nil {
		return 0, false
	}
	key, ok := videoPricingProductKey(spec)
	if !ok {
		return 0, false
	}
	credits, err := h.deps.PricingCatalog.DisplayEstimateCredits(key)
	return credits, err == nil && credits > 0
}

func (h *Handler) jobPricingSnapshot(operation domain.OperationType, modality domain.Modality, photoSelection photoDialogSelection, videoSpec videoModeSpec) (pricingcatalog.PricingSnapshot, error) {
	switch operation {
	case domain.OperationImageGenerate:
		if h.deps.PricingCatalog == nil {
			return pricingcatalog.PricingSnapshot{}, pricingcatalog.ErrPriceNotFound
		}
		if modality != domain.ModalityImage {
			return pricingcatalog.PricingSnapshot{}, pricingcatalog.ErrInvalidProductKey
		}
		key, ok := h.imageJobPricingProductKey(photoSelection)
		if !ok {
			return pricingcatalog.PricingSnapshot{}, pricingcatalog.ErrInvalidProductKey
		}
		return h.deps.PricingCatalog.Snapshot(key)
	case domain.OperationVideoGenerate:
		if h.deps.PricingCatalog == nil {
			return pricingcatalog.PricingSnapshot{}, pricingcatalog.ErrPriceNotFound
		}
		if modality != domain.ModalityVideo {
			return pricingcatalog.PricingSnapshot{}, pricingcatalog.ErrInvalidProductKey
		}
		key, ok := h.videoJobPricingProductKey(videoSpec)
		if !ok {
			return pricingcatalog.PricingSnapshot{}, pricingcatalog.ErrInvalidProductKey
		}
		return h.deps.PricingCatalog.Snapshot(key)
	default:
		return pricingcatalog.PricingSnapshot{}, nil
	}
}

func (h *Handler) imageJobPricingProductKey(selection photoDialogSelection) (pricingcatalog.ProductKey, bool) {
	if key, ok := imagePricingProductKey(selection.Model.ModelID, selection.Quality); ok {
		return key, true
	}
	for _, model := range h.cfg.ImageModels {
		if !model.Enabled {
			continue
		}
		quality := strings.TrimSpace(model.DefaultQuality)
		if quality == "" && len(model.QualityOptions) > 0 {
			quality = strings.TrimSpace(model.QualityOptions[0])
		}
		key, ok := imagePricingProductKey(model.ID, quality)
		if !ok {
			continue
		}
		if _, err := h.deps.PricingCatalog.CostEstimateCredits(key); err == nil {
			return key, true
		}
	}
	return pricingcatalog.ProductKey{}, false
}

func (h *Handler) videoJobPricingProductKey(spec videoModeSpec) (pricingcatalog.ProductKey, bool) {
	if key, ok := videoPricingProductKey(spec); ok {
		return key, true
	}
	for _, route := range h.cfg.VideoRoutes {
		if !route.Enabled {
			continue
		}
		candidate, ok := videoModeFromPublicRoute(route)
		if !ok {
			continue
		}
		key, ok := videoPricingProductKey(candidate)
		if !ok {
			continue
		}
		if _, err := h.deps.PricingCatalog.CostEstimateCredits(key); err == nil {
			return key, true
		}
	}
	key, ok := videoPricingProductKey(videoModeSpec{
		VideoRouteAlias: domain.VideoRouteKlingO3Standard,
		DurationSec:     5,
		Resolution:      pricingcatalog.VideoResolution720p,
	})
	if !ok {
		return pricingcatalog.ProductKey{}, false
	}
	if _, err := h.deps.PricingCatalog.CostEstimateCredits(key); err != nil {
		return pricingcatalog.ProductKey{}, false
	}
	return key, true
}

func imagePricingProductKey(modelID, quality string) (pricingcatalog.ProductKey, bool) {
	key := pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: strings.TrimSpace(modelID),
		Quality:      strings.TrimSpace(quality),
	}
	key = key.Normalize()
	return key, key.Valid()
}

func videoPricingProductKey(spec videoModeSpec) (pricingcatalog.ProductKey, bool) {
	resolution := strings.TrimSpace(spec.Resolution)
	if resolution == "" {
		resolution = defaultVKVideoResolution(spec.VideoRouteAlias)
	}
	key := pricingcatalog.ProductKey{
		Operation:       domain.OperationVideoGenerate,
		Modality:        domain.ModalityVideo,
		VideoRouteAlias: spec.VideoRouteAlias,
		Resolution:      resolution,
		DurationSec:     spec.DurationSec,
	}
	key = key.Normalize()
	return key, key.Valid()
}

func (h *Handler) photoQualityOptions(publicModel productcatalog.ImageModel) []photoQualityOption {
	out := make([]photoQualityOption, 0, len(publicModel.QualityOptions))
	for _, rawQuality := range publicModel.QualityOptions {
		quality, ok := modelcatalog.NormalizeImageQuality(rawQuality)
		if !ok {
			continue
		}
		price, ok := h.imageDisplayEstimateCredits(publicModel.ID, quality)
		if !ok {
			continue
		}
		out = append(out, photoQualityOption{
			Label:   quality,
			Price:   price,
			Command: domain.CommandMenuImageQualitySelect,
			ModelID: publicModel.ID,
			Quality: quality,
		})
	}
	return out
}

func (h *Handler) currentBalance(ctx context.Context, user *domain.User) (int64, error) {
	if user == nil {
		return 0, nil
	}
	acc, err := h.deps.Billing.EnsureAccount(ctx, user.ID)
	if err != nil {
		return 0, fmt.Errorf("ensure billing account: %w", err)
	}
	return acc.BalanceCached, nil
}

func (h *Handler) topUpReturnURL(groupID int64) string {
	raw := strings.TrimSpace(h.cfg.TopUpReturnURL)
	if shouldUseVKDialogReturnURL(raw) && groupID > 0 {
		return fmt.Sprintf("https://vk.com/write-%d", groupID)
	}
	return raw
}

func shouldUseVKDialogReturnURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Host)
	path := strings.Trim(u.EscapedPath(), "/")
	return (host == "vk.com" || host == "www.vk.com") && path == ""
}

func paymentIntentReturnURLMatches(intent *domain.PaymentIntent, returnURL string) bool {
	if intent == nil {
		return false
	}
	return paymentIntentReturnURL(intent) == strings.TrimSpace(returnURL)
}

func paymentIntentReturnURL(intent *domain.PaymentIntent) string {
	if intent == nil || len(intent.Metadata) == 0 {
		return ""
	}
	var metadata struct {
		ReturnURL string `json:"return_url"`
	}
	if err := json.Unmarshal(intent.Metadata, &metadata); err != nil {
		return ""
	}
	return strings.TrimSpace(metadata.ReturnURL)
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

const antiSpamDegradedGenerationMessage = "Генерации временно приостановлены: сервис защиты от спама недоступен.\n\nМеню, баланс и помощь работают. Попробуйте чуть позже."

func (h *Handler) handleAntiSpamDegraded(ctx context.Context, cause error, inboundID uuid.UUID, idemKey string, peerID int64, parsed commandrouter.Result) (bool, error) {
	actionClass := antiSpamActionClass(parsed)
	blockExpensive := antiSpamBlocksOnDependencyError(parsed)
	result := "degraded_allowed"
	if blockExpensive {
		result = "degraded_blocked"
	}
	h.logger.Warn("vk anti-spam degraded",
		slog.String("surface", "vk_bot"),
		slog.String("action_class", actionClass),
		slog.String("reason", "dependency_error"),
		slog.String("operation", productCommandOperation(parsed)),
		slog.String("modality", productCommandModality(parsed)),
		logging.ErrorAttr(cause))
	metrics.ObserveProductEvent("vk_bot", "command", "antispam", productCommandOperation(parsed), productCommandModality(parsed), result)
	if !blockExpensive {
		return false, nil
	}
	decision := antispam.Decision{
		Allowed: false,
		Kind:    antispam.DecisionDegraded,
		Message: antiSpamDegradedGenerationMessage,
	}
	if err := h.finishAntiSpamBlockedEvent(ctx, inboundID, idemKey, peerID, parsed, decision); err != nil {
		return true, err
	}
	return true, nil
}

func (h *Handler) finishAntiSpamBlockedEvent(ctx context.Context, inboundID uuid.UUID, idemKey string, peerID int64, parsed commandrouter.Result, decision antispam.Decision) error {
	if err := h.sendAntiSpamResponse(ctx, idemKey, peerID, decision); err != nil {
		h.logger.Warn("vk anti-spam response failed; completing inbound",
			slog.String("surface", "vk_bot"),
			slog.String("action_class", antiSpamActionClass(parsed)),
			slog.String("decision", string(decision.Kind)),
			logging.ErrorAttr(err))
	}
	if err := h.deps.Inbound.SetStatus(ctx, inboundID, domain.InboundProcessed); err != nil {
		return fmt.Errorf("mark inbound processed: %w", err)
	}
	if err := h.deps.Idempotency.MarkCompleted(ctx, idemKey, inboundID); err != nil {
		return fmt.Errorf("mark idempotency completed: %w", err)
	}
	return nil
}

func antiSpamBlocksOnDependencyError(parsed commandrouter.Result) bool {
	return parsed.CreatesJob()
}

func antiSpamActionClass(parsed commandrouter.Result) string {
	switch {
	case parsed.CreatesJob():
		return "generation"
	case parsed.Type == domain.CommandTopUp:
		return "payment"
	case parsed.Type == domain.CommandUnknown:
		return "unknown"
	default:
		return "control"
	}
}

func (h *Handler) gptDialogActive(ctx context.Context, peerID int64) bool {
	mode, ok := h.getDialogMode(ctx, peerID)
	return ok && mode == dialogModeGPT
}

func (h *Handler) photoDialogActive(ctx context.Context, peerID int64) bool {
	_, ok := h.photoSelectionForActiveDialog(ctx, peerID)
	return ok
}

type photoDialogSelection struct {
	Model   modelcatalog.Model
	Quality string
}

func (h *Handler) photoSelectionForActiveDialog(ctx context.Context, peerID int64) (photoDialogSelection, bool) {
	mode, ok := h.getDialogMode(ctx, peerID)
	if !ok {
		return photoDialogSelection{}, false
	}
	return photoSelectionFromDialogMode(mode)
}

func photoSelectionFromDialogMode(mode dialogMode) (photoDialogSelection, bool) {
	modelID, quality, ok := parsePhotoPromptMode(mode)
	if !ok {
		switch mode {
		case dialogModePhotoNanoBanana2:
			modelID, quality, ok = modelcatalog.MiniAppImageNanoBanana2, modelcatalog.ImageQuality1K, true
		case dialogModePhotoGPTImage2:
			modelID, quality, ok = modelcatalog.MiniAppImageGPTImage2, modelcatalog.ImageQuality1K, true
		case dialogModePhotoText:
			modelID, quality, ok = modelcatalog.MiniAppImageNanoBananaPro, modelcatalog.ImageQuality1K, true
		default:
			return photoDialogSelection{}, false
		}
	}
	model, ok := modelcatalog.ResolveMiniAppModel(domain.OperationImageGenerate, modelID)
	if !ok {
		return photoDialogSelection{}, false
	}
	model = modelcatalog.ApplyImageQuality(model, quality)
	return photoDialogSelection{Model: model, Quality: quality}, true
}

func photoModelIDFromCommand(t domain.CommandType) (string, bool) {
	switch t {
	case domain.CommandMenuImageNanoBanana2:
		return modelcatalog.MiniAppImageNanoBanana2, true
	case domain.CommandMenuImageDeepInfraSeedream:
		return modelcatalog.MiniAppImageSeedream45, true
	case domain.CommandMenuImageDeepInfraSDXL:
		return modelcatalog.MiniAppImageSDXLTurbo, true
	case domain.CommandMenuImageGPTImage2:
		return modelcatalog.MiniAppImageGPTImage2, true
	case domain.CommandMenuImageText:
		return modelcatalog.MiniAppImageNanoBananaPro, true
	default:
		return "", false
	}
}

func isPhotoModelCommand(t domain.CommandType) bool {
	_, ok := photoModelIDFromCommand(t)
	return ok
}

func (h *Handler) photoModelIDForQualitySelection(ctx context.Context, peerID int64) (string, bool) {
	mode, ok := h.getDialogMode(ctx, peerID)
	if !ok {
		return "", false
	}
	return parsePhotoSelectMode(mode)
}

func (h *Handler) photoModelIDForQualityBack(ctx context.Context, peerID int64) (string, bool) {
	mode, ok := h.getDialogMode(ctx, peerID)
	if !ok {
		return "", false
	}
	if modelID, ok := parsePhotoSelectMode(mode); ok {
		return modelID, true
	}
	modelID, _, ok := parsePhotoPromptMode(mode)
	return modelID, ok
}

func photoQualityFromCommand(t domain.CommandType) (string, bool) {
	switch t {
	case domain.CommandMenuImageQuality1K:
		return modelcatalog.ImageQuality1K, true
	case domain.CommandMenuImageQuality2K:
		return modelcatalog.ImageQuality2K, true
	case domain.CommandMenuImageQuality4K:
		return modelcatalog.ImageQuality4K, true
	default:
		return "", false
	}
}

func isPhotoQualityCommand(t domain.CommandType) bool {
	_, ok := photoQualityFromCommand(t)
	return ok
}

func photoSelectMode(modelID string) dialogMode {
	return dialogMode(dialogModePhotoSelectPrefix + strings.TrimSpace(modelID))
}

func photoPromptMode(modelID, quality string) dialogMode {
	if strings.TrimSpace(quality) == "" {
		return dialogMode(dialogModePhotoPromptPrefix + strings.TrimSpace(modelID))
	}
	return dialogMode(dialogModePhotoPromptPrefix + strings.TrimSpace(modelID) + ":" + strings.TrimSpace(quality))
}

func parsePhotoSelectMode(mode dialogMode) (string, bool) {
	raw, ok := strings.CutPrefix(string(mode), dialogModePhotoSelectPrefix)
	if !ok || strings.TrimSpace(raw) == "" {
		return "", false
	}
	return strings.TrimSpace(raw), true
}

func parsePhotoPromptMode(mode dialogMode) (string, string, bool) {
	raw, ok := strings.CutPrefix(string(mode), dialogModePhotoPromptPrefix)
	if !ok {
		return "", "", false
	}
	modelID, quality, ok := strings.Cut(raw, ":")
	if !ok {
		modelID = raw
		quality = ""
	}
	if strings.TrimSpace(quality) != "" {
		quality, ok = modelcatalog.NormalizeImageQuality(quality)
		if !ok {
			return "", "", false
		}
	}
	if strings.TrimSpace(modelID) == "" {
		return "", "", false
	}
	return strings.TrimSpace(modelID), quality, true
}

func (h *Handler) getDialogMode(ctx context.Context, peerID int64) (dialogMode, bool) {
	now := time.Now()
	h.modeMu.Lock()
	h.pruneDialogModesLocked(now, peerID)
	cached, ok := h.dialogModes[peerID]
	h.modeMu.Unlock()
	if ok {
		return cached.Mode, true
	}
	if h.deps.DialogState == nil {
		return "", false
	}
	persistedMode, ok, err := h.deps.DialogState.Get(ctx, peerID)
	if err != nil {
		h.logger.Warn("vk dialog mode lookup failed",
			slog.Int64("peer_id", peerID),
			logging.ErrorAttr(err))
		return "", false
	}
	if !ok {
		return "", false
	}
	mode := dialogMode(persistedMode)
	h.modeMu.Lock()
	h.dialogModes[peerID] = cachedDialogMode{
		Mode:      mode,
		ExpiresAt: time.Now().Add(h.cfg.LocalUIStateTTL),
	}
	h.pruneDialogModesLocked(time.Now(), peerID)
	h.modeMu.Unlock()
	return mode, true
}

func (h *Handler) setDialogMode(ctx context.Context, peerID int64, mode dialogMode) {
	now := time.Now()
	h.modeMu.Lock()
	h.dialogModes[peerID] = cachedDialogMode{
		Mode:      mode,
		ExpiresAt: now.Add(h.cfg.LocalUIStateTTL),
	}
	h.pruneDialogModesLocked(now, peerID)
	h.modeMu.Unlock()
	if h.deps.DialogState == nil {
		return
	}
	if err := h.deps.DialogState.Set(ctx, peerID, string(mode)); err != nil {
		h.logger.Warn("vk dialog mode persist failed",
			slog.Int64("peer_id", peerID),
			slog.String("mode", string(mode)),
			logging.ErrorAttr(err))
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
			logging.ErrorAttr(err))
	}
}

func (h *Handler) pruneDialogModesLocked(now time.Time, keepPeerID int64) {
	for peerID, cached := range h.dialogModes {
		if cached.Mode == "" || localUIStateExpired(cached.ExpiresAt, now) {
			delete(h.dialogModes, peerID)
		}
	}
	for len(h.dialogModes) > h.cfg.LocalUIStateMaxEntries {
		removed := false
		for peerID := range h.dialogModes {
			if peerID == keepPeerID && len(h.dialogModes) > 1 {
				continue
			}
			delete(h.dialogModes, peerID)
			removed = true
			break
		}
		if !removed {
			return
		}
	}
}

func localUIStateExpired(expiresAt, now time.Time) bool {
	return expiresAt.IsZero() || !expiresAt.After(now)
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
