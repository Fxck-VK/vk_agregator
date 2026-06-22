// Package config loads service configuration from environment variables with
// sensible local-development defaults.
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"vk-ai-aggregator/internal/domain"
)

const (
	DataServiceModeLocal    = "local"
	DataServiceModeExternal = "external"
	DataServiceModeManaged  = "managed"

	WorkerModeAll         = "all"
	WorkerModeJobs        = "jobs"
	WorkerModeMaintenance = "maintenance"

	MediaVideoProbePolicyDisabled        = "disabled"
	MediaVideoProbePolicyTrustedProvider = "trusted_provider"
	MediaVideoProbePolicyProbeRequired   = "probe_required"

	MediaVideoTranscodePolicyNever    = "never"
	MediaVideoTranscodePolicyFallback = "fallback"
	MediaVideoTranscodePolicyAlways   = "always"

	MediaDeliverRawProviderVideoNever         = "never"
	MediaDeliverRawProviderVideoIfProbePassed = "if_probe_passed"
	MediaDeliverRawProviderVideoAlwaysDevOnly = "always_dev_only"
)

// Config is the full application configuration shared by the entrypoints.
type Config struct {
	// Env is the deployment environment ("development", "staging" or
	// "production"). Production fails closed on the full secret/scanner set;
	// staging is for test VPS deployments with production-like routing.
	Env string

	// DataServicesMode is a default for PostgresMode, RedisMode and S3Mode.
	// Modes: local = bundled Docker service, external = self-managed remote
	// service, managed = cloud/provider-managed service.
	DataServicesMode string
	PostgresMode     string
	RedisMode        string
	S3Mode           string

	HTTPAddr      string
	DatabaseURL   string
	MigrationsDir string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
	S3Bucket    string
	S3Region    string
	// S3AddressingStyle controls bucket addressing for S3-compatible storage:
	// path, virtual-hosted or auto.
	S3AddressingStyle string

	VKConfirmationToken string
	VKSecret            string

	AdminToken string

	// WorkerMode selects which loops cmd/worker starts. Production runs job
	// consumers and maintenance as separate processes.
	WorkerMode     string
	WorkerGroup    string
	WorkerConsumer string
	// WorkerMetricsAddr exposes worker-local Prometheus metrics. Empty disables
	// the endpoint.
	WorkerMetricsAddr string

	// MaxAttempts bounds retryable re-enqueues before a task is dead-lettered.
	MaxAttempts int
	// RetryBaseDelay/RetryMaxDelay parameterize exponential backoff between
	// retries.
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration

	// ModerationExtraTerms extends the default keyword blocklist.
	ModerationExtraTerms []string

	// WebhookRateLimitRPS/Burst bound inbound webhook traffic per source.
	WebhookRateLimitRPS   float64
	WebhookRateLimitBurst int
	// MiniAppJobRateLimitRPS/Burst bound Mini App job creation per verified
	// vk_user_id.
	MiniAppJobRateLimitRPS   float64
	MiniAppJobRateLimitBurst int
	// VKAntiSpam* bound user-level VK bot messages and billable GPT jobs.
	VKAntiSpamEnabled             bool
	VKAntiSpamMessageLimit        int
	VKAntiSpamMessageWindow       time.Duration
	VKAntiSpamGPTLimit            int
	VKAntiSpamGPTWindow           time.Duration
	VKAntiSpamImageDailyLimit     int
	VKAntiSpamImageDailyWindow    time.Duration
	VKAntiSpamCooldown            time.Duration
	VKAntiSpamViolationLimit      int
	VKAntiSpamViolationWindow     time.Duration
	VKAntiSpamBlockDuration       time.Duration
	VKAntiSpamNewUserAge          time.Duration
	VKAntiSpamNewUserMessageLimit int
	VKAntiSpamNewUserGPTLimit     int
	VKAntiSpamNewUserGPTWindow    time.Duration
	VKAntiSpamActiveGPTJobLimit   int

	// DBMaxConns/DBMinConns size the Postgres pool (audit SC1).
	DBMaxConns int32
	DBMinConns int32
	// RedisPoolSize sizes the Redis connection pool (audit SC1).
	RedisPoolSize int
	// StreamMaxLen caps Redis stream backlog; 0 disables trimming.
	StreamMaxLen int64

	// PriceOverrides replaces per-operation prices, e.g.
	// "text_generate=2,image_generate=12" (audit C1).
	PriceOverrides map[string]int64
	// MaxJobCost rejects any job whose estimate exceeds this cap (0 = no cap).
	MaxJobCost int64

	// PaymentProvider selects the money provider for balance top-ups.
	PaymentProvider                           string
	YooKassaShopID                            string
	YooKassaSecretKey                         string
	YooKassaBaseURL                           string
	YooKassaReturnURL                         string
	YooKassaReturnURLMiniApp                  string
	YooKassaReturnURLVKBot                    string
	YooKassaWebhookIPAllowlistEnabled         bool
	YooKassaWebhookIPAllowlist                []string
	PaymentWebhookRequireHTTPS                bool
	PaymentWebhookTrustedProxies              []string
	PaymentWebhookAddr                        string
	PaymentWebhookPollInterval                time.Duration
	PaymentWebhookBatchLimit                  int
	PaymentReconciliationInterval             time.Duration
	PaymentReconciliationLimit                int
	PaymentReconciliationStaleAfter           time.Duration
	FeatureVKTopUpStatusEditEnabled           bool
	FeatureMiniAppPaymentCancelEnabled        bool
	FeatureMiniAppTopUpCatalogDropdownEnabled bool
	FeatureMiniAppDarkThemeOnlyEnabled        bool
	FeatureMiniAppTopUpHistoryDropdownEnabled bool

	// Provider selects the primary generation provider. ProviderChain, when set,
	// enables router/fallback selection across multiple providers.
	Provider      string
	ProviderChain []string
	// ImageProvider, when set, makes one provider the preferred route for image
	// jobs while preserving the configured provider chain as fallback.
	ImageProvider string
	// VideoProvider optionally overrides the provider used for video jobs.
	VideoProvider string
	// ImageModel/ImageSize are provider-agnostic defaults attached to image jobs.
	ImageModel string
	ImageSize  string
	// VideoModel/VideoDurationSec/VideoResolution/VideoAspectRatio/VideoDraft are
	// worker-owned defaults for video jobs (not trusted from clients).
	VideoModel       string
	VideoDurationSec int
	VideoResolution  string
	VideoAspectRatio string
	VideoDraft       bool

	// Media pipeline config is worker-owned. VK Bot and Mini App surfaces must
	// not call ffmpeg/ffprobe directly.
	MediaPipelineEnabled         bool
	MediaVideoProbePolicy        string
	MediaVideoTranscodePolicy    string
	MediaDeliverRawProviderVideo string
	FFProbePath                  string
	FFmpegPath                   string
	MediaMaxVideoSizeBytes       int64
	MediaMaxVideoDurationSec     int
	MediaMaxVideoWidth           int
	MediaMaxVideoHeight          int
	MediaMaxVideoBitrate         int64
	MediaAllowedVideoContainers  []string
	MediaAllowedVideoCodecs      []string
	MediaProbeTimeout            time.Duration
	MediaTranscodeTimeout        time.Duration
	// Media scale guards keep expensive media work bounded under production
	// traffic. Queue/backpressure guards are shared via Redis/Postgres wiring;
	// concurrent probe/transcode limits are per worker process.
	MediaMaxConcurrentProbes              int
	MediaMaxConcurrentTranscodes          int
	MediaMaxPendingVariants               int
	MediaMaxActiveVideoJobsPerUser        int
	MediaProviderMaxAttemptsPerJob        int
	MediaProviderFallbackBudget           int
	MediaQueueDegradeThreshold            int64
	MediaMaxConcurrentUploads             int
	MediaReferenceUploadsEnabled          bool
	MediaReferenceWebPEnabled             bool
	MediaMaxImageUploadBytes              int64
	MediaMaxImageWidth                    int
	MediaMaxImageHeight                   int
	MediaMaxImagePixels                   int64
	MediaProviderQualityGuardEnabled      bool
	MediaProviderQualityDegradedFailures  int
	MediaProviderQualityDisabledFailures  int
	MediaProviderQualityRecoverySuccesses int
	// MediaProviderContractsRaw is the original env string used to fail closed
	// on malformed JSON during Validate.
	MediaProviderContractsRaw string
	// MediaProviderContracts is an optional product-level media allowlist. It
	// augments built-in safe defaults in cmd/worker and never belongs in the
	// frontend.
	MediaProviderContracts []domain.ProviderMediaContract

	APIMartAPIKey          string
	APIMartBaseURL         string
	APIMartProviderEnabled bool
	PoYoAPIKey             string
	PoYoBaseURL            string
	PoYoProviderEnabled    bool
	RunwayMLAPISecret      string
	RunwayMLBaseURL        string
	RunwayProviderEnabled  bool

	FeatureImageModelNanoBananaProEnabled       bool
	FeatureImageModelGPTImage2Enabled           bool
	FeatureImageModelNanoBanana2Enabled         bool
	FeatureVideoRouterEnabled                   bool
	FeatureVideoRouteHailuo23FastEnabled        bool
	FeatureVideoRouteHailuo23StandardEnabled    bool
	FeatureVideoRouteKlingO3StandardEnabled     bool
	FeatureVideoRouteRunwayGen4TurboEnabled     bool
	FeatureVideoRouteSeedance20FastEnabled      bool
	FeatureVideoRouteRunwayGen45Enabled         bool
	FeatureVideoRouteResellerExperimentsEnabled bool

	OpenAIAPIKey       string
	OpenAIBaseURL      string
	OpenAITextModel    string
	OpenAIImageModel   string
	OpenAIImageSize    string
	OpenAIVideoModel   string
	OpenAIVideoSeconds string
	OpenAIVideoSize    string
	OpenAITextPrice    int64
	OpenAIImagePrice   int64
	OpenAIVideoPrice   int64

	DeepInfraAPIKey                string
	DeepInfraBaseURL               string
	DeepInfraTextModel             string
	DeepInfraTextPrice             int64
	DeepInfraImageModel            string
	DeepInfraImageFallbackModel    string
	DeepInfraImagePrice            int64
	DeepInfraImageReferenceEnabled bool
	DeepInfraVideoModel            string
	DeepInfraVideoDurationSec      int
	DeepInfraVideoResolution       string
	DeepInfraVideoAspectRatio      string
	DeepInfraVideoDraft            bool
	DeepInfraVideoPrice            int64
	DeepInfraVideoHTTPTimeout      time.Duration

	TextContextEnabled                bool
	TextContextMaxInputTokens         int
	TextContextMaxOutputTokens        int
	TextContextSummaryMaxTokens       int
	TextContextRecentMessagesLimit    int
	TextContextSummarizeAfterMessages int
	TextContextSummarizeAfterTokens   int

	// ModerationProvider selects output moderation: "keyword" (default) or
	// "openai". ArtifactScanner selects artifact byte scanning: "none" or
	// "openai". Production requires a scanner; staging may run with "none".
	ModerationProvider                  string
	OpenAIModerationModel               string
	ArtifactScanner                     string
	AllowUnscannedArtifactsInProduction bool

	// VKDeliveryMode selects the delivery client: "mock" (default) or "real".
	VKDeliveryMode       string
	VKAccessToken        string
	VKVideoAccessToken   string
	VKVideoUploadGroupID int64
	VKVideoDeliveryMode  string
	VKAPIVersion         string
	VKAPIBaseURL         string
	// VKWelcomeAttachment is an optional pre-uploaded VK attachment sent with
	// the /start menu, e.g. photo-239332376_123_accesskey.
	VKWelcomeAttachment string
	// VKMenuButtonMode controls inline product menu buttons: "callback"
	// prevents user echo messages; "text" keeps legacy behavior.
	VKMenuButtonMode string
	// VKUnroutedTextMode controls plain VK text outside an active text mode:
	// "reply" asks the user to choose a mode, "silent" ignores it, and "gpt"
	// preserves the legacy behavior where any text creates a GPT job.
	VKUnroutedTextMode string
	// VKDialogModeTTL controls how long an active VK peer mode (for example
	// GPT/text mode) survives without activity.
	VKDialogModeTTL time.Duration
	// VK menu feature flags hide individual product-menu buttons without
	// removing the underlying screens. Account/top-up are disabled by default;
	// other menu flags default to enabled.
	VKMenuVideoEnabled                 bool
	VKMenuImageEnabled                 bool
	VKMenuGPTEnabled                   bool
	VKMenuStudentsEnabled              bool
	VKMenuAccountEnabled               bool
	VKMenuTopUpEnabled                 bool
	VKMenuVideoSora2Enabled            bool
	VKMenuVideoSora2StartEnabled       bool
	VKMenuVideoSora2ExamplesEnabled    bool
	VKMenuVideoKling21Enabled          bool
	VKMenuVideoKling21StartEnabled     bool
	VKMenuVideoKling21ExamplesEnabled  bool
	VKMenuVideoSeedance1Enabled        bool
	VKMenuVideoSeedance1LiteEnabled    bool
	VKMenuVideoSeedance1ProEnabled     bool
	VKMenuVideoHailuo02Enabled         bool
	VKMenuVideoHailuo02StandardEnabled bool
	VKMenuVideoHailuo02FastEnabled     bool
	VKMenuVideoRoutesPreviewEnabled    bool
	VKMenuImageTextEnabled             bool
	VKMenuImageReferenceEnabled        bool
	VKMenuStudentsSolverEnabled        bool
	VKMenuStudentsPresentationEnabled  bool
	VKMenuStudentsReportEnabled        bool
	VKMenuStudentsQAEnabled            bool
	// VKTopUpReceiptEmail/VKTopUpReceiptPhone are server-side receipt contacts
	// used by the VK Bot quick top-up flow. Mini App may still collect a user
	// receipt contact explicitly.
	VKTopUpReceiptEmail string
	VKTopUpReceiptPhone string

	// VKReferralLinkBase is the public VK entry URL used to build a user's
	// single referral link. If it contains "{code}", the placeholder is replaced;
	// otherwise the code is appended as ref=<code>.
	VKReferralLinkBase string
	// VKReferralShareBase is reserved for future VK share/open-link flows.
	VKReferralShareBase string
	ReferralCodeLength  int
	// Referral signup rewards are posted through billing ledger entries.
	ReferralReferrerSignupRewardCredits int64
	ReferralReferredSignupRewardCredits int64
	// ReferralRewardOnActivation gates reward ledger writes during rollout.
	ReferralRewardOnActivation bool

	// VKAppID is the VK Mini App application identifier.
	VKAppID string
	// VKAppSecret is the VK Mini App protected key used to verify launch-params
	// signatures. When empty the signature check is skipped (dev/mock mode).
	VKAppSecret string
	// MiniAppLaunchParamsMaxAge is the maximum age of VK launch params before
	// they are rejected. Zero disables the age check.
	MiniAppLaunchParamsMaxAge time.Duration
	// FrontendTelemetryEnabled accepts safe Mini App client telemetry events.
	FrontendTelemetryEnabled bool
	// FrontendTelemetryUserHashSecret enables anonymized client user hashing
	// later; raw user identifiers must never be emitted in telemetry labels.
	FrontendTelemetryUserHashSecret string

	// ArtifactURLTTL is how long signed artifact delivery URLs stay valid.
	ArtifactURLTTL time.Duration
	// SignedDelivery delivers media via signed URLs instead of bucket refs (ST1).
	SignedDelivery bool
	// ArtifactRetentionDays configures object lifecycle expiry (0 = keep) (ST1).
	ArtifactRetentionDays int
	// Media*RetentionDays split artifact cleanup by lifecycle class. Zero means
	// keep that class; failed/deleted defaults to ARTIFACT_RETENTION_DAYS.
	ArtifactFreeRetentionDays      int
	ArtifactPaidRetentionDays      int
	ArtifactTemporaryRetentionDays int
	ArtifactOrphanRetentionDays    int
	MediaInputRetentionDays        int
	MediaFailedRetentionDays       int
	MediaOriginalRetentionDays     int
	MediaVariantRetentionDays      int

	// WorkerProviderCallTimeout bounds one provider Submit/Poll call in workers.
	WorkerProviderCallTimeout time.Duration
	// WorkerProviderPollBaseDelay/MaxDelay control async provider status polling.
	WorkerProviderPollBaseDelay time.Duration
	WorkerProviderPollMaxDelay  time.Duration

	// WorkerShutdownGrace is how long workers may drain in-flight work after a
	// shutdown signal before their processing context is cancelled.
	WorkerShutdownGrace time.Duration

	// MaintenanceInterval runs operational cleanup jobs on this cadence.
	MaintenanceInterval time.Duration
	// OutboxRetention keeps already-published/failed outbox rows for this long.
	OutboxRetention time.Duration
	// BillingReconciliationInterval runs balance-vs-ledger checks on this cadence.
	BillingReconciliationInterval time.Duration
	// BillingReconciliationLimit caps accounts checked per reconciliation pass.
	BillingReconciliationLimit int
	// JobEventsRetentionDays bounds short-lived job lifecycle diagnostics.
	JobEventsRetentionDays int
	// ProviderPayloadRetentionDays bounds raw provider request/response storage.
	ProviderPayloadRetentionDays int
	// JobLogRetentionBatchSize caps job diagnostics cleanup per maintenance pass.
	JobLogRetentionBatchSize int
	// JobErrorAggregateLookbackDays bounds the aggregate refresh window.
	JobErrorAggregateLookbackDays int
	// AnalyticsAggregateLookbackDays bounds dashboard aggregate refreshes.
	AnalyticsAggregateLookbackDays int
	// ConversationMessageRetentionDays bounds raw dialog prompt/answer storage.
	ConversationMessageRetentionDays int
	// ConversationSummaryRetentionDays keeps compact memory longer than raw turns.
	ConversationSummaryRetentionDays int
	// ConversationRetentionBatchSize caps retention updates per maintenance pass.
	ConversationRetentionBatchSize int

	// TracingServiceName is reported in OpenTelemetry resource attributes.
	TracingServiceName string
	// TracingExporter selects the trace exporter: "none" (default), "stdout" or "otlp".
	TracingExporter string
	// TracingOTLPEndpoint is the OTLP gRPC collector endpoint.
	TracingOTLPEndpoint string
	// TracingSampleRatio is the default parent-based trace sampling ratio.
	TracingSampleRatio float64
	// TracingCriticalSampleRatio is reserved for critical path sampling policy.
	TracingCriticalSampleRatio float64
}

// IsProduction reports whether the service runs in a production environment.
func (c Config) IsProduction() bool {
	return isProductionEnv(c.Env)
}

// IsLoadTest reports whether the service runs in the synthetic load-test
// contour. Load tests must stay mock-backed and isolated from paid providers.
func (c Config) IsLoadTest() bool {
	return isLoadTestEnv(c.Env)
}

// EffectiveMediaVideoProbePolicy returns the normalized worker probe policy.
func (c Config) EffectiveMediaVideoProbePolicy() string {
	if policy := normalizeConfigToken(c.MediaVideoProbePolicy); policy != "" {
		return policy
	}
	return defaultMediaVideoProbePolicy(c.Env, c.MediaPipelineEnabled)
}

// EffectiveMediaVideoTranscodePolicy returns the normalized worker transcode policy.
func (c Config) EffectiveMediaVideoTranscodePolicy() string {
	if policy := normalizeConfigToken(c.MediaVideoTranscodePolicy); policy != "" {
		return policy
	}
	return MediaVideoTranscodePolicyNever
}

// EffectiveMediaDeliverRawProviderVideo returns the normalized raw-video delivery policy.
func (c Config) EffectiveMediaDeliverRawProviderVideo() string {
	if policy := normalizeConfigToken(c.MediaDeliverRawProviderVideo); policy != "" {
		return policy
	}
	return defaultMediaDeliverRawProviderVideo(c.Env, c.MediaPipelineEnabled)
}

// MediaVideoProbeRequired reports whether generated video must be probed before delivery.
func (c Config) MediaVideoProbeRequired() bool {
	return c.EffectiveMediaVideoProbePolicy() == MediaVideoProbePolicyProbeRequired
}

// MediaVideoTranscodeEnabled reports whether ffmpeg may be used by the worker.
func (c Config) MediaVideoTranscodeEnabled() bool {
	policy := c.EffectiveMediaVideoTranscodePolicy()
	return policy == MediaVideoTranscodePolicyFallback || policy == MediaVideoTranscodePolicyAlways
}

// PaymentWebhookHTTPSRequired reports whether the payment webhook receiver must
// reject requests that did not arrive over HTTPS or through a trusted HTTPS
// reverse proxy.
func (c Config) PaymentWebhookHTTPSRequired() bool {
	return c.PaymentWebhookRequireHTTPS || c.IsServerEnv()
}

// Validate fails closed: in production, secrets that protect inbound webhooks
// and the admin API must be set. Returns a descriptive error otherwise.
func (c Config) Validate() error {
	var missing []string
	if err := c.ValidateDataServiceModes(); err != nil {
		return err
	}
	if style := strings.ToLower(strings.TrimSpace(c.S3AddressingStyle)); style != "" && !knownS3AddressingStyle(style) {
		return fmt.Errorf("config: S3_ADDRESSING_STYLE must be auto, path, or virtual-hosted")
	}
	if mode := strings.ToLower(strings.TrimSpace(c.VKMenuButtonMode)); mode != "" && mode != "callback" && mode != "text" {
		return fmt.Errorf("config: VK_MENU_BUTTON_MODE must be callback or text")
	}
	if mode := strings.ToLower(strings.TrimSpace(c.VKUnroutedTextMode)); mode != "" && mode != "reply" && mode != "silent" && mode != "gpt" {
		return fmt.Errorf("config: VK_UNROUTED_TEXT_MODE must be reply, silent, or gpt")
	}
	if mode := strings.ToLower(strings.TrimSpace(c.WorkerMode)); mode != "" && mode != WorkerModeAll && mode != WorkerModeJobs && mode != WorkerModeMaintenance {
		return fmt.Errorf("config: WORKER_MODE must be all, jobs, or maintenance")
	}
	if mode := strings.ToLower(strings.TrimSpace(c.VKVideoDeliveryMode)); mode != "" && mode != "doc" && mode != "video" {
		return fmt.Errorf("config: VK_VIDEO_DELIVERY_MODE must be doc or video")
	}
	if provider := strings.ToLower(strings.TrimSpace(c.Provider)); provider != "" && !knownProvider(provider) {
		return fmt.Errorf("config: PROVIDER must be one of %s", knownProviderList())
	}
	for _, provider := range c.ProviderChain {
		if provider = strings.ToLower(strings.TrimSpace(provider)); provider != "" && !knownProvider(provider) {
			return fmt.Errorf("config: PROVIDER_CHAIN contains unknown provider %q; allowed: %s", provider, knownProviderList())
		}
	}
	if provider := strings.ToLower(strings.TrimSpace(c.ImageProvider)); provider != "" && !knownProvider(provider) {
		return fmt.Errorf("config: IMAGE_PROVIDER must be one of %s", knownProviderList())
	}
	if provider := strings.ToLower(strings.TrimSpace(c.VideoProvider)); provider != "" && !knownProvider(provider) {
		return fmt.Errorf("config: VIDEO_PROVIDER must be one of %s", knownProviderList())
	}
	if provider := strings.ToLower(strings.TrimSpace(c.ModerationProvider)); provider != "" && provider != "keyword" && provider != "openai" {
		return fmt.Errorf("config: MODERATION_PROVIDER must be keyword or openai")
	}
	if scanner := strings.ToLower(strings.TrimSpace(c.ArtifactScanner)); scanner != "" && !knownArtifactScanner(scanner) {
		return fmt.Errorf("config: ARTIFACT_SCANNER must be none or openai")
	}
	if err := validatePriceOverrides(c.PriceOverrides); err != nil {
		return err
	}
	if provider := strings.ToLower(strings.TrimSpace(c.PaymentProvider)); provider != "" && !knownPaymentProvider(provider) {
		return fmt.Errorf("config: PAYMENT_PROVIDER must be one of mock, yookassa")
	}
	if c.YooKassaWebhookIPAllowlistEnabled && len(c.YooKassaWebhookIPAllowlist) == 0 {
		return fmt.Errorf("config: YOOKASSA_WEBHOOK_IP_ALLOWLIST must be set when YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED=true")
	}
	if err := c.validateVideoRouteProviderConfig(); err != nil {
		return err
	}
	if err := validateIPOrCIDRList("YOOKASSA_WEBHOOK_IP_ALLOWLIST", c.YooKassaWebhookIPAllowlist); err != nil {
		return err
	}
	if err := validateIPOrCIDRList("PAYMENT_WEBHOOK_TRUSTED_PROXIES", c.PaymentWebhookTrustedProxies); err != nil {
		return err
	}
	probePolicy := c.EffectiveMediaVideoProbePolicy()
	if err := validateMediaVideoProbePolicy(probePolicy); err != nil {
		return err
	}
	transcodePolicy := c.EffectiveMediaVideoTranscodePolicy()
	if err := validateMediaVideoTranscodePolicy(transcodePolicy); err != nil {
		return err
	}
	rawProviderVideoPolicy := c.EffectiveMediaDeliverRawProviderVideo()
	if err := validateMediaDeliverRawProviderVideo(rawProviderVideoPolicy); err != nil {
		return err
	}
	if err := c.validateLoadTestSafeModes(); err != nil {
		return err
	}
	if c.IsProduction() && probePolicy != MediaVideoProbePolicyProbeRequired {
		return fmt.Errorf("config: MEDIA_VIDEO_PROBE_POLICY must be %s in production", MediaVideoProbePolicyProbeRequired)
	}
	if probePolicy == MediaVideoProbePolicyTrustedProvider && !c.usesOnlyMockProviders() {
		return fmt.Errorf("config: MEDIA_VIDEO_PROBE_POLICY=trusted_provider is only allowed for mock-only provider config")
	}
	if c.IsProduction() && transcodePolicy == MediaVideoTranscodePolicyAlways {
		return fmt.Errorf("config: MEDIA_VIDEO_TRANSCODE_POLICY=always is not allowed in production")
	}
	if transcodePolicy != MediaVideoTranscodePolicyNever && !c.MediaPipelineEnabled {
		return fmt.Errorf("config: MEDIA_VIDEO_TRANSCODE_POLICY=%s requires MEDIA_PIPELINE_ENABLED=true", transcodePolicy)
	}
	if transcodePolicy != MediaVideoTranscodePolicyNever && probePolicy != MediaVideoProbePolicyProbeRequired {
		return fmt.Errorf("config: MEDIA_VIDEO_TRANSCODE_POLICY=%s requires MEDIA_VIDEO_PROBE_POLICY=probe_required", transcodePolicy)
	}
	if c.IsProduction() && rawProviderVideoPolicy == MediaDeliverRawProviderVideoAlwaysDevOnly {
		return fmt.Errorf("config: MEDIA_DELIVER_RAW_PROVIDER_VIDEO=always_dev_only is not allowed in production")
	}
	if c.MediaPipelineEnabled && probePolicy == MediaVideoProbePolicyProbeRequired {
		if strings.TrimSpace(c.FFProbePath) == "" {
			return fmt.Errorf("config: FFPROBE_PATH must be set when MEDIA_VIDEO_PROBE_POLICY=probe_required and MEDIA_PIPELINE_ENABLED=true")
		}
		if c.MediaMaxVideoSizeBytes <= 0 {
			return fmt.Errorf("config: MEDIA_MAX_VIDEO_SIZE_BYTES must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if c.MediaMaxVideoDurationSec <= 0 {
			return fmt.Errorf("config: MEDIA_MAX_VIDEO_DURATION_SEC must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if c.MediaMaxVideoWidth <= 0 {
			return fmt.Errorf("config: MEDIA_MAX_VIDEO_WIDTH must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if c.MediaMaxVideoHeight <= 0 {
			return fmt.Errorf("config: MEDIA_MAX_VIDEO_HEIGHT must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if c.MediaMaxVideoBitrate <= 0 {
			return fmt.Errorf("config: MEDIA_MAX_VIDEO_BITRATE must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if len(c.MediaAllowedVideoContainers) == 0 {
			return fmt.Errorf("config: MEDIA_ALLOWED_VIDEO_CONTAINERS must not be empty when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if err := validateNormalizedList("MEDIA_ALLOWED_VIDEO_CONTAINERS", c.MediaAllowedVideoContainers); err != nil {
			return err
		}
		if len(c.MediaAllowedVideoCodecs) == 0 {
			return fmt.Errorf("config: MEDIA_ALLOWED_VIDEO_CODECS must not be empty when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
		if err := validateNormalizedList("MEDIA_ALLOWED_VIDEO_CODECS", c.MediaAllowedVideoCodecs); err != nil {
			return err
		}
		if c.MediaProbeTimeout <= 0 {
			return fmt.Errorf("config: MEDIA_PROBE_TIMEOUT must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
		}
	}
	if c.MediaPipelineEnabled && c.MediaVideoTranscodeEnabled() {
		if strings.TrimSpace(c.FFmpegPath) == "" {
			return fmt.Errorf("config: FFMPEG_PATH must be set when MEDIA_VIDEO_TRANSCODE_POLICY=%s", transcodePolicy)
		}
		if c.MediaTranscodeTimeout <= 0 {
			return fmt.Errorf("config: MEDIA_TRANSCODE_TIMEOUT must be positive when MEDIA_VIDEO_TRANSCODE_POLICY=%s", transcodePolicy)
		}
	}
	if c.MediaMaxConcurrentProbes < 0 {
		return fmt.Errorf("config: MEDIA_MAX_CONCURRENT_PROBES must be non-negative")
	}
	if c.MediaMaxConcurrentTranscodes < 0 {
		return fmt.Errorf("config: MEDIA_MAX_CONCURRENT_TRANSCODES must be non-negative")
	}
	if c.MediaMaxPendingVariants < 0 {
		return fmt.Errorf("config: MEDIA_MAX_PENDING_VARIANTS must be non-negative")
	}
	if c.MediaMaxActiveVideoJobsPerUser < 0 {
		return fmt.Errorf("config: MEDIA_MAX_ACTIVE_VIDEO_JOBS_PER_USER must be non-negative")
	}
	if c.MediaProviderMaxAttemptsPerJob < 0 {
		return fmt.Errorf("config: MEDIA_PROVIDER_MAX_ATTEMPTS_PER_JOB must be non-negative")
	}
	if c.MediaProviderFallbackBudget < 0 {
		return fmt.Errorf("config: MEDIA_PROVIDER_FALLBACK_BUDGET_PER_JOB must be non-negative")
	}
	if c.MediaQueueDegradeThreshold < 0 {
		return fmt.Errorf("config: MEDIA_QUEUE_DEGRADE_THRESHOLD must be non-negative")
	}
	if c.MediaMaxConcurrentUploads < 0 {
		return fmt.Errorf("config: MEDIA_MAX_CONCURRENT_UPLOADS must be non-negative")
	}
	if c.MediaMaxImageUploadBytes < 0 {
		return fmt.Errorf("config: MEDIA_MAX_IMAGE_UPLOAD_BYTES must be non-negative")
	}
	if c.MediaMaxImageWidth < 0 {
		return fmt.Errorf("config: MEDIA_MAX_IMAGE_WIDTH must be non-negative")
	}
	if c.MediaMaxImageHeight < 0 {
		return fmt.Errorf("config: MEDIA_MAX_IMAGE_HEIGHT must be non-negative")
	}
	if c.MediaMaxImagePixels < 0 {
		return fmt.Errorf("config: MEDIA_MAX_IMAGE_PIXELS must be non-negative")
	}
	if c.MediaProviderQualityDegradedFailures < 0 {
		return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_DEGRADED_FAILURES must be non-negative")
	}
	if c.MediaProviderQualityDisabledFailures < 0 {
		return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_DISABLED_FAILURES must be non-negative")
	}
	if c.MediaProviderQualityRecoverySuccesses < 0 {
		return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_RECOVERY_SUCCESSES must be non-negative")
	}
	if c.MediaProviderQualityGuardEnabled {
		if c.MediaProviderQualityDegradedFailures <= 0 {
			return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_DEGRADED_FAILURES must be positive when MEDIA_PROVIDER_QUALITY_GUARD_ENABLED=true")
		}
		if c.MediaProviderQualityDisabledFailures <= 0 {
			return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_DISABLED_FAILURES must be positive when MEDIA_PROVIDER_QUALITY_GUARD_ENABLED=true")
		}
		if c.MediaProviderQualityDisabledFailures < c.MediaProviderQualityDegradedFailures {
			return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_DISABLED_FAILURES must be >= MEDIA_PROVIDER_QUALITY_DEGRADED_FAILURES")
		}
		if c.MediaProviderQualityRecoverySuccesses <= 0 {
			return fmt.Errorf("config: MEDIA_PROVIDER_QUALITY_RECOVERY_SUCCESSES must be positive when MEDIA_PROVIDER_QUALITY_GUARD_ENABLED=true")
		}
	}
	if c.ArtifactFreeRetentionDays < 0 {
		return fmt.Errorf("config: ARTIFACT_FREE_RETENTION_DAYS must be non-negative")
	}
	if c.ArtifactPaidRetentionDays < 0 {
		return fmt.Errorf("config: ARTIFACT_PAID_RETENTION_DAYS must be non-negative")
	}
	if c.ArtifactTemporaryRetentionDays < 0 {
		return fmt.Errorf("config: ARTIFACT_TEMP_RETENTION_DAYS must be non-negative")
	}
	if c.ArtifactOrphanRetentionDays < 0 {
		return fmt.Errorf("config: ARTIFACT_ORPHAN_RETENTION_DAYS must be non-negative")
	}
	if c.MediaInputRetentionDays < 0 {
		return fmt.Errorf("config: MEDIA_INPUT_RETENTION_DAYS must be non-negative")
	}
	if c.MediaFailedRetentionDays < 0 {
		return fmt.Errorf("config: MEDIA_FAILED_RETENTION_DAYS must be non-negative")
	}
	if c.MediaOriginalRetentionDays < 0 {
		return fmt.Errorf("config: MEDIA_ORIGINAL_RETENTION_DAYS must be non-negative")
	}
	if c.MediaVariantRetentionDays < 0 {
		return fmt.Errorf("config: MEDIA_VARIANT_RETENTION_DAYS must be non-negative")
	}
	if c.JobEventsRetentionDays < 0 {
		return fmt.Errorf("config: RETENTION_JOB_EVENTS_DAYS must be non-negative")
	}
	if c.ProviderPayloadRetentionDays < 0 {
		return fmt.Errorf("config: RETENTION_PROVIDER_PAYLOAD_DAYS must be non-negative")
	}
	if c.JobLogRetentionBatchSize < 0 {
		return fmt.Errorf("config: JOB_LOG_RETENTION_BATCH_SIZE must be non-negative")
	}
	if c.JobErrorAggregateLookbackDays < 0 {
		return fmt.Errorf("config: JOB_ERROR_AGGREGATE_LOOKBACK_DAYS must be non-negative")
	}
	if c.AnalyticsAggregateLookbackDays < 0 {
		return fmt.Errorf("config: ANALYTICS_AGGREGATE_LOOKBACK_DAYS must be non-negative")
	}
	if c.ConversationMessageRetentionDays < 0 {
		return fmt.Errorf("config: RETENTION_CONVERSATION_MESSAGES_DAYS must be non-negative")
	}
	if c.ConversationSummaryRetentionDays < 0 {
		return fmt.Errorf("config: RETENTION_CONVERSATION_SUMMARIES_DAYS must be non-negative")
	}
	if c.ConversationRetentionBatchSize < 0 {
		return fmt.Errorf("config: CONVERSATION_RETENTION_BATCH_SIZE must be non-negative")
	}
	if c.MediaPipelineEnabled && probePolicy == MediaVideoProbePolicyProbeRequired && c.MediaMaxConcurrentProbes == 0 {
		return fmt.Errorf("config: MEDIA_MAX_CONCURRENT_PROBES must be positive when MEDIA_VIDEO_PROBE_POLICY=probe_required")
	}
	if c.MediaPipelineEnabled && c.MediaVideoTranscodeEnabled() && c.MediaMaxConcurrentTranscodes == 0 {
		return fmt.Errorf("config: MEDIA_MAX_CONCURRENT_TRANSCODES must be positive when MEDIA_VIDEO_TRANSCODE_POLICY=%s", transcodePolicy)
	}
	if strings.TrimSpace(c.MediaProviderContractsRaw) != "" {
		contracts, err := parseMediaProviderContracts(c.MediaProviderContractsRaw)
		if err != nil {
			return fmt.Errorf("config: MEDIA_PROVIDER_CONTRACTS_JSON is invalid: %w", err)
		}
		for _, contract := range contracts {
			if err := contract.Validate(); err != nil {
				return fmt.Errorf("config: MEDIA_PROVIDER_CONTRACTS_JSON: %w", err)
			}
		}
	} else {
		for _, contract := range c.MediaProviderContracts {
			if err := contract.Validate(); err != nil {
				return fmt.Errorf("config: MEDIA_PROVIDER_CONTRACTS_JSON: %w", err)
			}
		}
	}
	if c.IsProduction() {
		if c.usesMockProvider() {
			return fmt.Errorf("config: mock provider is not allowed in production")
		}
		if c.usesRealGenerationProvider() && artifactScannerDisabled(c.ArtifactScanner) && !c.AllowUnscannedArtifactsInProduction {
			return fmt.Errorf("config: ARTIFACT_SCANNER=openai is required in production unless ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION=true")
		}
		if strings.EqualFold(strings.TrimSpace(c.PaymentProvider), "mock") {
			return fmt.Errorf("config: PAYMENT_PROVIDER=mock is not allowed in production")
		}
		if c.VKSecret == "" {
			missing = append(missing, "VK_SECRET")
		}
		if c.AdminToken == "" {
			missing = append(missing, "ADMIN_TOKEN")
		}
		if c.VKConfirmationToken == "" || c.VKConfirmationToken == "dev-confirmation" {
			missing = append(missing, "VK_CONFIRMATION_TOKEN")
		}
		// The Mini App BFF must verify launch-param signatures for real in
		// production; an empty secret would silently disable the check.
		if c.VKAppSecret == "" {
			missing = append(missing, "VK_APP_SECRET")
		}
		if strings.EqualFold(strings.TrimSpace(c.PaymentProvider), "yookassa") && len(c.PaymentWebhookTrustedProxies) == 0 {
			missing = append(missing, "PAYMENT_WEBHOOK_TRUSTED_PROXIES")
		}
	}
	if c.usesOpenAI() && c.OpenAIAPIKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if c.usesDeepInfra() && c.DeepInfraAPIKey == "" {
		missing = append(missing, "DEEPINFRA_API_KEY")
	}
	if c.usesProviderName("apimart") {
		if strings.TrimSpace(c.APIMartAPIKey) == "" {
			missing = append(missing, "APIMART_API_KEY")
		}
		if strings.TrimSpace(c.APIMartBaseURL) == "" {
			missing = append(missing, "APIMART_BASE_URL")
		}
	}
	if c.usesProviderName("poyo") {
		if strings.TrimSpace(c.PoYoAPIKey) == "" {
			missing = append(missing, "POYO_API_KEY")
		}
		if strings.TrimSpace(c.PoYoBaseURL) == "" {
			missing = append(missing, "POYO_BASE_URL")
		}
	}
	if c.usesProviderName("runway") && c.RunwayProviderEnabled && c.IsProduction() {
		if strings.TrimSpace(c.RunwayMLAPISecret) == "" {
			missing = append(missing, "RUNWAYML_API_SECRET")
		}
		if strings.TrimSpace(c.RunwayMLBaseURL) == "" {
			missing = append(missing, "RUNWAYML_BASE_URL")
		}
	}
	if c.VKDeliveryMode == "real" && c.VKAccessToken == "" {
		missing = append(missing, "VK_ACCESS_TOKEN")
	}
	if strings.EqualFold(strings.TrimSpace(c.PaymentProvider), "yookassa") {
		if c.YooKassaShopID == "" {
			missing = append(missing, "YOOKASSA_SHOP_ID")
		}
		if c.YooKassaSecretKey == "" {
			missing = append(missing, "YOOKASSA_SECRET_KEY")
		}
		if c.YooKassaReturnURL == "" {
			missing = append(missing, "YOOKASSA_RETURN_URL")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("config: missing required production secrets: %s", strings.Join(missing, ", "))
	}
	if err := c.validateSelectedProviderSwitches(); err != nil {
		return err
	}
	return nil
}

func (c Config) validateLoadTestSafeModes() error {
	if !c.IsLoadTest() {
		return nil
	}

	var problems []string
	if !configTokenEquals(c.Provider, "mock") {
		problems = append(problems, "PROVIDER=mock")
	}
	if !providerChainMockOnly(c.ProviderChain) {
		problems = append(problems, "PROVIDER_CHAIN=mock or empty")
	}
	if !optionalConfigTokenEquals(c.ImageProvider, "mock") {
		problems = append(problems, "IMAGE_PROVIDER=mock or empty")
	}
	if !optionalConfigTokenEquals(c.VideoProvider, "mock") {
		problems = append(problems, "VIDEO_PROVIDER=mock or empty")
	}
	if !configTokenEquals(c.PaymentProvider, "mock") {
		problems = append(problems, "PAYMENT_PROVIDER=mock")
	}
	if !configTokenEquals(c.VKDeliveryMode, "mock") {
		problems = append(problems, "VK_DELIVERY_MODE=mock")
	}
	if !optionalConfigTokenEquals(c.ModerationProvider, "keyword") {
		problems = append(problems, "MODERATION_PROVIDER=keyword or empty")
	}
	if !configTokenEquals(c.ArtifactScanner, "none") {
		problems = append(problems, "ARTIFACT_SCANNER=none")
	}

	if len(problems) > 0 {
		return fmt.Errorf("config: APP_ENV=loadtest requires safe mock modes: %s", strings.Join(problems, ", "))
	}
	return nil
}

func (c Config) ValidateDataServiceModes() error {
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: "DATA_SERVICES_MODE", value: c.DataServicesMode},
		{name: "POSTGRES_MODE", value: c.PostgresMode},
		{name: "REDIS_MODE", value: c.RedisMode},
		{name: "S3_MODE", value: c.S3Mode},
	} {
		if !knownDataServiceMode(item.value) {
			return fmt.Errorf("config: %s must be one of local, external, managed", item.name)
		}
	}
	return nil
}

// Load reads configuration from .env/_env and the process environment.
func Load() Config {
	loadDotenv()

	host, _ := os.Hostname()
	appEnv := env("APP_ENV", "development")
	dataServicesMode := envMode("DATA_SERVICES_MODE", DataServiceModeLocal)
	provider := env("PROVIDER", "mock")
	providerChain := envList("PROVIDER_CHAIN")
	if len(providerChain) == 0 {
		providerChain = []string{provider}
	}
	mediaPipelineEnabled := envBool("MEDIA_PIPELINE_ENABLED", false)
	mediaProviderContractsRaw := env("MEDIA_PROVIDER_CONTRACTS_JSON", "")
	mediaProviderContracts, _ := parseMediaProviderContracts(mediaProviderContractsRaw)
	artifactRetentionDays := envInt("ARTIFACT_RETENTION_DAYS", 0)
	return Config{
		Env:              appEnv,
		DataServicesMode: dataServicesMode,
		PostgresMode:     envMode("POSTGRES_MODE", dataServicesMode),
		RedisMode:        envMode("REDIS_MODE", dataServicesMode),
		S3Mode:           envMode("S3_MODE", dataServicesMode),
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable"),
		MigrationsDir:    env("MIGRATIONS_DIR", "migrations"),

		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),

		S3Endpoint:  env("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey: env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey: env("S3_SECRET_KEY", "minioadmin"),
		S3UseSSL:    envBool("S3_USE_SSL", false),
		S3Bucket:    env("S3_BUCKET", "artifacts"),
		S3Region:    env("S3_REGION", "us-east-1"),
		// path is the safest default for local MinIO and most S3-compatible
		// providers. Set virtual-hosted or auto only when the provider DNS/TLS
		// setup supports bucket hostnames.
		S3AddressingStyle: envConfigToken("S3_ADDRESSING_STYLE", "path"),

		VKConfirmationToken: env("VK_CONFIRMATION_TOKEN", "dev-confirmation"),
		VKSecret:            env("VK_SECRET", ""),

		AdminToken: env("ADMIN_TOKEN", ""),

		WorkerMode:        envConfigToken("WORKER_MODE", WorkerModeAll),
		WorkerGroup:       env("WORKER_GROUP", "workers"),
		WorkerConsumer:    env("WORKER_CONSUMER", defaultStr(host, "worker-1")),
		WorkerMetricsAddr: env("WORKER_METRICS_ADDR", ":9090"),

		MaxAttempts:    envInt("MAX_ATTEMPTS", 3),
		RetryBaseDelay: envDuration("RETRY_BASE_DELAY", 500*time.Millisecond),
		RetryMaxDelay:  envDuration("RETRY_MAX_DELAY", 30*time.Second),

		ModerationExtraTerms: envList("MODERATION_EXTRA_TERMS"),

		WebhookRateLimitRPS:           envFloat("WEBHOOK_RATE_LIMIT_RPS", 20),
		WebhookRateLimitBurst:         envInt("WEBHOOK_RATE_LIMIT_BURST", 40),
		MiniAppJobRateLimitRPS:        envFloat("MINIAPP_JOB_RATE_LIMIT_RPS", 1),
		MiniAppJobRateLimitBurst:      envInt("MINIAPP_JOB_RATE_LIMIT_BURST", 5),
		VKAntiSpamEnabled:             envBool("VK_ANTISPAM_ENABLED", true),
		VKAntiSpamMessageLimit:        envInt("VK_ANTISPAM_MESSAGE_LIMIT", 40),
		VKAntiSpamMessageWindow:       envDuration("VK_ANTISPAM_MESSAGE_WINDOW", time.Minute),
		VKAntiSpamGPTLimit:            envInt("VK_ANTISPAM_GPT_LIMIT", 3),
		VKAntiSpamGPTWindow:           envDuration("VK_ANTISPAM_GPT_WINDOW", 30*time.Second),
		VKAntiSpamImageDailyLimit:     envInt("VK_ANTISPAM_IMAGE_DAILY_LIMIT", 100),
		VKAntiSpamImageDailyWindow:    envDuration("VK_ANTISPAM_IMAGE_DAILY_WINDOW", 24*time.Hour),
		VKAntiSpamCooldown:            envDuration("VK_ANTISPAM_COOLDOWN", 30*time.Second),
		VKAntiSpamViolationLimit:      envInt("VK_ANTISPAM_VIOLATION_LIMIT", 5),
		VKAntiSpamViolationWindow:     envDuration("VK_ANTISPAM_VIOLATION_WINDOW", 10*time.Minute),
		VKAntiSpamBlockDuration:       envDuration("VK_ANTISPAM_BLOCK_DURATION", 15*time.Minute),
		VKAntiSpamNewUserAge:          envDuration("VK_ANTISPAM_NEW_USER_AGE", 4*time.Hour),
		VKAntiSpamNewUserMessageLimit: envInt("VK_ANTISPAM_NEW_USER_MESSAGE_LIMIT", 30),
		VKAntiSpamNewUserGPTLimit:     envInt("VK_ANTISPAM_NEW_USER_GPT_LIMIT", 1),
		VKAntiSpamNewUserGPTWindow:    envDuration("VK_ANTISPAM_NEW_USER_GPT_WINDOW", 15*time.Second),
		VKAntiSpamActiveGPTJobLimit:   envInt("VK_ANTISPAM_ACTIVE_GPT_JOB_LIMIT", 2),

		DBMaxConns:    envInt32("DB_MAX_CONNS", 10),
		DBMinConns:    envInt32("DB_MIN_CONNS", 0),
		RedisPoolSize: envInt("REDIS_POOL_SIZE", 10),
		StreamMaxLen:  int64(envInt("STREAM_MAX_LEN", 100000)),

		PriceOverrides: envPriceMap("PRICES"),
		MaxJobCost:     int64(envInt("MAX_JOB_COST", 0)),

		PaymentProvider:                           env("PAYMENT_PROVIDER", "mock"),
		YooKassaShopID:                            env("YOOKASSA_SHOP_ID", ""),
		YooKassaSecretKey:                         env("YOOKASSA_SECRET_KEY", ""),
		YooKassaBaseURL:                           env("YOOKASSA_BASE_URL", "https://api.yookassa.ru/v3"),
		YooKassaReturnURL:                         env("YOOKASSA_RETURN_URL", ""),
		YooKassaReturnURLMiniApp:                  env("YOOKASSA_RETURN_URL_MINIAPP", ""),
		YooKassaReturnURLVKBot:                    env("YOOKASSA_RETURN_URL_VK_BOT", ""),
		YooKassaWebhookIPAllowlistEnabled:         envBool("YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED", false),
		YooKassaWebhookIPAllowlist:                envList("YOOKASSA_WEBHOOK_IP_ALLOWLIST"),
		PaymentWebhookRequireHTTPS:                envBool("PAYMENT_WEBHOOK_REQUIRE_HTTPS", false),
		PaymentWebhookTrustedProxies:              envList("PAYMENT_WEBHOOK_TRUSTED_PROXIES"),
		PaymentWebhookAddr:                        env("PAYMENT_WEBHOOK_ADDR", ":8082"),
		PaymentWebhookPollInterval:                envDuration("PAYMENT_WEBHOOK_POLL_INTERVAL", 5*time.Second),
		PaymentWebhookBatchLimit:                  envInt("PAYMENT_WEBHOOK_BATCH_LIMIT", 20),
		PaymentReconciliationInterval:             envDuration("PAYMENT_RECONCILIATION_INTERVAL", time.Minute),
		PaymentReconciliationLimit:                envInt("PAYMENT_RECONCILIATION_LIMIT", 100),
		PaymentReconciliationStaleAfter:           envDuration("PAYMENT_RECONCILIATION_STALE_AFTER", 30*time.Second),
		FeatureVKTopUpStatusEditEnabled:           envBool("FEATURE_VK_TOPUP_STATUS_EDIT_ENABLED", false),
		FeatureMiniAppPaymentCancelEnabled:        envBool("FEATURE_MINIAPP_PAYMENT_CANCEL_ENABLED", false),
		FeatureMiniAppTopUpCatalogDropdownEnabled: envBool("FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED", false),
		FeatureMiniAppDarkThemeOnlyEnabled:        envBool("FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED", false),
		FeatureMiniAppTopUpHistoryDropdownEnabled: envBool("FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED", false),
		Provider:                                 provider,
		ProviderChain:                            providerChain,
		ImageProvider:                            env("IMAGE_PROVIDER", ""),
		VideoProvider:                            env("VIDEO_PROVIDER", ""),
		ImageModel:                               env("IMAGE_MODEL", ""),
		ImageSize:                                env("IMAGE_SIZE", ""),
		VideoModel:                               env("VIDEO_MODEL", ""),
		VideoDurationSec:                         envInt("VIDEO_DURATION_SEC", 5),
		VideoResolution:                          env("VIDEO_RESOLUTION", "720p"),
		VideoAspectRatio:                         env("VIDEO_ASPECT_RATIO", "16:9"),
		VideoDraft:                               envBool("VIDEO_DRAFT", true),
		MediaPipelineEnabled:                     mediaPipelineEnabled,
		MediaVideoProbePolicy:                    envConfigToken("MEDIA_VIDEO_PROBE_POLICY", defaultMediaVideoProbePolicy(appEnv, mediaPipelineEnabled)),
		MediaVideoTranscodePolicy:                envConfigToken("MEDIA_VIDEO_TRANSCODE_POLICY", MediaVideoTranscodePolicyNever),
		MediaDeliverRawProviderVideo:             envConfigToken("MEDIA_DELIVER_RAW_PROVIDER_VIDEO", defaultMediaDeliverRawProviderVideo(appEnv, mediaPipelineEnabled)),
		FFProbePath:                              env("FFPROBE_PATH", "ffprobe"),
		FFmpegPath:                               env("FFMPEG_PATH", "ffmpeg"),
		MediaMaxVideoSizeBytes:                   envInt64("MEDIA_MAX_VIDEO_SIZE_BYTES", 256<<20),
		MediaMaxVideoDurationSec:                 envInt("MEDIA_MAX_VIDEO_DURATION_SEC", 60),
		MediaMaxVideoWidth:                       envInt("MEDIA_MAX_VIDEO_WIDTH", 1920),
		MediaMaxVideoHeight:                      envInt("MEDIA_MAX_VIDEO_HEIGHT", 1080),
		MediaMaxVideoBitrate:                     envInt64("MEDIA_MAX_VIDEO_BITRATE", 12000000),
		MediaAllowedVideoContainers:              envNormalizedList("MEDIA_ALLOWED_VIDEO_CONTAINERS", []string{"mp4", "mov", "webm"}),
		MediaAllowedVideoCodecs:                  envNormalizedList("MEDIA_ALLOWED_VIDEO_CODECS", []string{"h264", "h265", "hevc", "vp8", "vp9", "av1"}),
		MediaProbeTimeout:                        envDuration("MEDIA_PROBE_TIMEOUT", 10*time.Second),
		MediaTranscodeTimeout:                    envDuration("MEDIA_TRANSCODE_TIMEOUT", 10*time.Minute),
		MediaMaxConcurrentProbes:                 envInt("MEDIA_MAX_CONCURRENT_PROBES", 2),
		MediaMaxConcurrentTranscodes:             envInt("MEDIA_MAX_CONCURRENT_TRANSCODES", 1),
		MediaMaxPendingVariants:                  envInt("MEDIA_MAX_PENDING_VARIANTS", 16),
		MediaMaxActiveVideoJobsPerUser:           envInt("MEDIA_MAX_ACTIVE_VIDEO_JOBS_PER_USER", 1),
		MediaProviderMaxAttemptsPerJob:           envInt("MEDIA_PROVIDER_MAX_ATTEMPTS_PER_JOB", 1),
		MediaProviderFallbackBudget:              envInt("MEDIA_PROVIDER_FALLBACK_BUDGET_PER_JOB", 0),
		MediaQueueDegradeThreshold:               envInt64("MEDIA_QUEUE_DEGRADE_THRESHOLD", 1000),
		MediaMaxConcurrentUploads:                envInt("MEDIA_MAX_CONCURRENT_UPLOADS", 8),
		MediaReferenceUploadsEnabled:             envBool("MEDIA_REFERENCE_UPLOADS_ENABLED", defaultMediaReferenceUploadsEnabled(appEnv)),
		MediaReferenceWebPEnabled:                envBool("MEDIA_REFERENCE_WEBP_ENABLED", false),
		MediaMaxImageUploadBytes:                 envInt64("MEDIA_MAX_IMAGE_UPLOAD_BYTES", 20<<20),
		MediaMaxImageWidth:                       envInt("MEDIA_MAX_IMAGE_WIDTH", 4096),
		MediaMaxImageHeight:                      envInt("MEDIA_MAX_IMAGE_HEIGHT", 4096),
		MediaMaxImagePixels:                      envInt64("MEDIA_MAX_IMAGE_PIXELS", 4096*4096),
		MediaProviderQualityGuardEnabled:         envBool("MEDIA_PROVIDER_QUALITY_GUARD_ENABLED", false),
		MediaProviderQualityDegradedFailures:     envInt("MEDIA_PROVIDER_QUALITY_DEGRADED_FAILURES", 3),
		MediaProviderQualityDisabledFailures:     envInt("MEDIA_PROVIDER_QUALITY_DISABLED_FAILURES", 5),
		MediaProviderQualityRecoverySuccesses:    envInt("MEDIA_PROVIDER_QUALITY_RECOVERY_SUCCESSES", 2),
		MediaProviderContractsRaw:                mediaProviderContractsRaw,
		MediaProviderContracts:                   mediaProviderContracts,
		APIMartAPIKey:                            env("APIMART_API_KEY", ""),
		APIMartBaseURL:                           env("APIMART_BASE_URL", "https://api.apimart.ai/v1"),
		APIMartProviderEnabled:                   envBool("APIMART_PROVIDER_ENABLED", false),
		PoYoAPIKey:                               env("POYO_API_KEY", ""),
		PoYoBaseURL:                              env("POYO_BASE_URL", ""),
		PoYoProviderEnabled:                      envBool("POYO_PROVIDER_ENABLED", false),
		RunwayMLAPISecret:                        env("RUNWAYML_API_SECRET", ""),
		RunwayMLBaseURL:                          env("RUNWAYML_BASE_URL", "https://api.dev.runwayml.com/v1"),
		RunwayProviderEnabled:                    envBool("RUNWAY_PROVIDER_ENABLED", false),
		FeatureImageModelNanoBananaProEnabled:    envBool("FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED", false),
		FeatureImageModelGPTImage2Enabled:        envBool("FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED", false),
		FeatureImageModelNanoBanana2Enabled:      envBool("FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED", false),
		FeatureVideoRouterEnabled:                envBool("FEATURE_VIDEO_ROUTER_ENABLED", false),
		FeatureVideoRouteHailuo23FastEnabled:     envBool("FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED", false),
		FeatureVideoRouteHailuo23StandardEnabled: envBool("FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED", false),
		FeatureVideoRouteKlingO3StandardEnabled: envBool(
			"FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED",
			envBool("FEATURE_VIDEO_ROUTE_KLING_O3_ENABLED", false),
		),
		FeatureVideoRouteRunwayGen4TurboEnabled:     envBool("FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED", false),
		FeatureVideoRouteSeedance20FastEnabled:      envBool("FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED", false),
		FeatureVideoRouteRunwayGen45Enabled:         envBool("FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED", false),
		FeatureVideoRouteResellerExperimentsEnabled: envBool("FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED", false),
		OpenAIAPIKey:                        env("OPENAI_API_KEY", ""),
		OpenAIBaseURL:                       env("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAITextModel:                     env("OPENAI_TEXT_MODEL", "gpt-4.1-mini"),
		OpenAIImageModel:                    env("OPENAI_IMAGE_MODEL", "gpt-image-1"),
		OpenAIImageSize:                     env("OPENAI_IMAGE_SIZE", "1024x1024"),
		OpenAIVideoModel:                    env("OPENAI_VIDEO_MODEL", "sora-2"),
		OpenAIVideoSeconds:                  env("OPENAI_VIDEO_SECONDS", "4"),
		OpenAIVideoSize:                     env("OPENAI_VIDEO_SIZE", "720x1280"),
		OpenAITextPrice:                     int64(envInt("OPENAI_TEXT_PRICE", 1)),
		OpenAIImagePrice:                    int64(envInt("OPENAI_IMAGE_PRICE", 10)),
		OpenAIVideoPrice:                    int64(envInt("OPENAI_VIDEO_PRICE", 50)),
		DeepInfraAPIKey:                     env("DEEPINFRA_API_KEY", ""),
		DeepInfraBaseURL:                    env("DEEPINFRA_BASE_URL", "https://api.deepinfra.com/v1/openai"),
		DeepInfraTextModel:                  env("DEEPINFRA_TEXT_MODEL", "deepseek-ai/DeepSeek-V4-Flash"),
		DeepInfraTextPrice:                  int64(envInt("DEEPINFRA_TEXT_PRICE", 1)),
		DeepInfraImageModel:                 env("DEEPINFRA_IMAGE_MODEL", "ByteDance/Seedream-4.5"),
		DeepInfraImageFallbackModel:         env("DEEPINFRA_IMAGE_FALLBACK_MODEL", ""),
		DeepInfraImagePrice:                 int64(envInt("DEEPINFRA_IMAGE_PRICE", 10)),
		DeepInfraImageReferenceEnabled:      envBool("DEEPINFRA_IMAGE_REFERENCE_ENABLED", false),
		DeepInfraVideoModel:                 env("DEEPINFRA_VIDEO_MODEL", "PrunaAI/p-video"),
		DeepInfraVideoDurationSec:           envInt("DEEPINFRA_VIDEO_DURATION_SEC", 5),
		DeepInfraVideoResolution:            env("DEEPINFRA_VIDEO_RESOLUTION", "720p"),
		DeepInfraVideoAspectRatio:           env("DEEPINFRA_VIDEO_ASPECT_RATIO", "16:9"),
		DeepInfraVideoDraft:                 envBool("DEEPINFRA_VIDEO_DRAFT", true),
		DeepInfraVideoPrice:                 int64(envInt("DEEPINFRA_VIDEO_PRICE", 10)),
		DeepInfraVideoHTTPTimeout:           envDuration("DEEPINFRA_VIDEO_HTTP_TIMEOUT", 180*time.Second),
		TextContextEnabled:                  envBool("TEXT_CONTEXT_ENABLED", true),
		TextContextMaxInputTokens:           envInt("TEXT_CONTEXT_MAX_INPUT_TOKENS", 1600),
		TextContextMaxOutputTokens:          envInt("TEXT_CONTEXT_MAX_OUTPUT_TOKENS", 800),
		TextContextSummaryMaxTokens:         envInt("TEXT_CONTEXT_SUMMARY_MAX_TOKENS", 400),
		TextContextRecentMessagesLimit:      envInt("TEXT_CONTEXT_RECENT_MESSAGES_LIMIT", 6),
		TextContextSummarizeAfterMessages:   envInt("TEXT_CONTEXT_SUMMARIZE_AFTER_MESSAGES", 10),
		TextContextSummarizeAfterTokens:     envInt("TEXT_CONTEXT_SUMMARIZE_AFTER_TOKENS", 1500),
		ModerationProvider:                  env("MODERATION_PROVIDER", "keyword"),
		OpenAIModerationModel:               env("OPENAI_MODERATION_MODEL", "omni-moderation-latest"),
		ArtifactScanner:                     env("ARTIFACT_SCANNER", "none"),
		AllowUnscannedArtifactsInProduction: envBool("ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION", false),

		VKDeliveryMode:                    env("VK_DELIVERY_MODE", "mock"),
		VKAccessToken:                     env("VK_ACCESS_TOKEN", ""),
		VKVideoAccessToken:                env("VK_VIDEO_ACCESS_TOKEN", ""),
		VKVideoUploadGroupID:              int64(envInt("VK_VIDEO_UPLOAD_GROUP_ID", 0)),
		VKVideoDeliveryMode:               env("VK_VIDEO_DELIVERY_MODE", "doc"),
		VKAPIVersion:                      env("VK_API_VERSION", "5.199"),
		VKAPIBaseURL:                      env("VK_API_BASE_URL", "https://api.vk.com/method"),
		VKWelcomeAttachment:               env("VK_WELCOME_ATTACHMENT", ""),
		VKMenuButtonMode:                  env("VK_MENU_BUTTON_MODE", "callback"),
		VKUnroutedTextMode:                env("VK_UNROUTED_TEXT_MODE", "reply"),
		VKDialogModeTTL:                   envDuration("VK_DIALOG_MODE_TTL", time.Hour),
		VKMenuVideoEnabled:                envBool("VK_MENU_VIDEO_ENABLED", true),
		VKMenuImageEnabled:                envBool("VK_MENU_IMAGE_ENABLED", true),
		VKMenuGPTEnabled:                  envBool("VK_MENU_GPT_ENABLED", true),
		VKMenuStudentsEnabled:             envBool("VK_MENU_STUDENTS_ENABLED", true),
		VKMenuAccountEnabled:              envBool("VK_MENU_ACCOUNT_ENABLED", false),
		VKMenuTopUpEnabled:                envBool("VK_MENU_TOP_UP_ENABLED", false),
		VKMenuVideoSora2Enabled:           envBool("VK_MENU_VIDEO_SORA2_ENABLED", true),
		VKMenuVideoSora2StartEnabled:      envBool("VK_MENU_VIDEO_SORA2_START_ENABLED", true),
		VKMenuVideoSora2ExamplesEnabled:   envBool("VK_MENU_VIDEO_SORA2_EXAMPLES_ENABLED", true),
		VKMenuVideoKling21Enabled:         envBool("VK_MENU_VIDEO_KLING21_ENABLED", true),
		VKMenuVideoKling21StartEnabled:    envBool("VK_MENU_VIDEO_KLING21_START_ENABLED", true),
		VKMenuVideoKling21ExamplesEnabled: envBool("VK_MENU_VIDEO_KLING21_EXAMPLES_ENABLED", true),
		VKMenuVideoSeedance1Enabled:       envBool("VK_MENU_VIDEO_SEEDANCE1_ENABLED", true),
		VKMenuVideoSeedance1LiteEnabled:   envBool("VK_MENU_VIDEO_SEEDANCE1_LITE_ENABLED", true),
		VKMenuVideoSeedance1ProEnabled:    envBool("VK_MENU_VIDEO_SEEDANCE1_PRO_ENABLED", true),
		VKMenuVideoHailuo02Enabled: envBool(
			"VK_MENU_VIDEO_HAILUO02_ENABLED",
			envBool("VK_MENU_VIDEO_HAIUO02_ENABLED", true),
		),
		VKMenuVideoHailuo02StandardEnabled: envBool(
			"VK_MENU_VIDEO_HAILUO02_STANDARD_ENABLED",
			envBool("VK_MENU_VIDEO_HAIUO02_STANDARD_ENABLED", true),
		),
		VKMenuVideoHailuo02FastEnabled: envBool(
			"VK_MENU_VIDEO_HAILUO02_FAST_ENABLED",
			envBool("VK_MENU_VIDEO_HAIUO02_FAST_ENABLED", true),
		),
		VKMenuVideoRoutesPreviewEnabled:   envBool("VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED", false),
		VKMenuImageTextEnabled:            envBool("VK_MENU_IMAGE_TEXT_ENABLED", false),
		VKMenuImageReferenceEnabled:       envBool("VK_MENU_IMAGE_REFERENCE_ENABLED", false),
		VKMenuStudentsSolverEnabled:       envBool("VK_MENU_STUDENTS_SOLVER_ENABLED", true),
		VKMenuStudentsPresentationEnabled: envBool("VK_MENU_STUDENTS_PRESENTATION_ENABLED", true),
		VKMenuStudentsReportEnabled:       envBool("VK_MENU_STUDENTS_REPORT_ENABLED", true),
		VKMenuStudentsQAEnabled:           envBool("VK_MENU_STUDENTS_QA_ENABLED", true),
		VKTopUpReceiptEmail:               env("VK_TOP_UP_RECEIPT_EMAIL", ""),
		VKTopUpReceiptPhone:               env("VK_TOP_UP_RECEIPT_PHONE", ""),
		VKReferralLinkBase:                env("VK_REFERRAL_LINK_BASE", ""),
		VKReferralShareBase:               env("VK_REFERRAL_SHARE_BASE", "https://vk.com/share.php"),
		ReferralCodeLength:                envInt("REFERRAL_CODE_LENGTH", 10),
		ReferralReferrerSignupRewardCredits: int64(envInt(
			"REFERRAL_REFERRER_SIGNUP_REWARD_CREDITS",
			10,
		)),
		ReferralReferredSignupRewardCredits: int64(envInt(
			"REFERRAL_REFERRED_SIGNUP_REWARD_CREDITS",
			0,
		)),
		ReferralRewardOnActivation: envBool("REFERRAL_REWARD_ON_ACTIVATION", true),

		VKAppID:                         env("VK_APP_ID", ""),
		VKAppSecret:                     env("VK_APP_SECRET", ""),
		MiniAppLaunchParamsMaxAge:       envDuration("MINIAPP_LAUNCH_PARAMS_MAX_AGE", time.Hour),
		FrontendTelemetryEnabled:        envBool("FRONTEND_TELEMETRY_ENABLED", false),
		FrontendTelemetryUserHashSecret: env("FRONTEND_TELEMETRY_USER_HASH_SECRET", ""),

		ArtifactURLTTL:                 envDuration("ARTIFACT_URL_TTL", time.Hour),
		SignedDelivery:                 envBool("SIGNED_DELIVERY", false),
		ArtifactRetentionDays:          artifactRetentionDays,
		ArtifactFreeRetentionDays:      envInt("ARTIFACT_FREE_RETENTION_DAYS", 7),
		ArtifactPaidRetentionDays:      envInt("ARTIFACT_PAID_RETENTION_DAYS", 365),
		ArtifactTemporaryRetentionDays: envInt("ARTIFACT_TEMP_RETENTION_DAYS", 1),
		ArtifactOrphanRetentionDays:    envInt("ARTIFACT_ORPHAN_RETENTION_DAYS", 7),
		MediaInputRetentionDays:        envInt("MEDIA_INPUT_RETENTION_DAYS", 0),
		MediaFailedRetentionDays:       envInt("MEDIA_FAILED_RETENTION_DAYS", artifactRetentionDays),
		MediaOriginalRetentionDays:     envInt("MEDIA_ORIGINAL_RETENTION_DAYS", 0),
		MediaVariantRetentionDays:      envInt("MEDIA_VARIANT_RETENTION_DAYS", artifactRetentionDays),

		WorkerProviderCallTimeout:     envDuration("WORKER_PROVIDER_CALL_TIMEOUT", 180*time.Second),
		WorkerProviderPollBaseDelay:   envDuration("WORKER_PROVIDER_POLL_BASE_DELAY", time.Second),
		WorkerProviderPollMaxDelay:    envDuration("WORKER_PROVIDER_POLL_MAX_DELAY", 5*time.Second),
		WorkerShutdownGrace:           envDuration("WORKER_SHUTDOWN_GRACE", 30*time.Second),
		MaintenanceInterval:           envDuration("MAINTENANCE_INTERVAL", time.Hour),
		OutboxRetention:               envDuration("OUTBOX_RETENTION", 7*24*time.Hour),
		BillingReconciliationInterval: envDuration("BILLING_RECONCILIATION_INTERVAL", 5*time.Minute),
		BillingReconciliationLimit:    envInt("BILLING_RECONCILIATION_LIMIT", 100),
		JobEventsRetentionDays:        envInt("RETENTION_JOB_EVENTS_DAYS", 30),
		ProviderPayloadRetentionDays:  envInt("RETENTION_PROVIDER_PAYLOAD_DAYS", 7),
		JobLogRetentionBatchSize:      envInt("JOB_LOG_RETENTION_BATCH_SIZE", 500),
		JobErrorAggregateLookbackDays: envInt("JOB_ERROR_AGGREGATE_LOOKBACK_DAYS", 30),
		AnalyticsAggregateLookbackDays: envInt(
			"ANALYTICS_AGGREGATE_LOOKBACK_DAYS",
			7,
		),
		ConversationMessageRetentionDays: envInt(
			"RETENTION_CONVERSATION_MESSAGES_DAYS",
			90,
		),
		ConversationSummaryRetentionDays: envInt(
			"RETENTION_CONVERSATION_SUMMARIES_DAYS",
			180,
		),
		ConversationRetentionBatchSize: envInt("CONVERSATION_RETENTION_BATCH_SIZE", 500),

		TracingServiceName:         env("OTEL_SERVICE_NAME", "vk-ai-aggregator"),
		TracingExporter:            env("OTEL_TRACES_EXPORTER", "none"),
		TracingOTLPEndpoint:        env("OTEL_EXPORTER_OTLP_ENDPOINT", "127.0.0.1:4317"),
		TracingSampleRatio:         envFloat("OTEL_TRACES_SAMPLE_RATIO", 0.1),
		TracingCriticalSampleRatio: envFloat("OTEL_TRACES_CRITICAL_SAMPLE_RATIO", 1),
	}
}

func (c Config) usesOpenAI() bool {
	if strings.EqualFold(c.Provider, "openai") ||
		strings.EqualFold(c.ImageProvider, "openai") ||
		strings.EqualFold(c.VideoProvider, "openai") ||
		strings.EqualFold(c.ModerationProvider, "openai") ||
		strings.EqualFold(c.ArtifactScanner, "openai") {
		return true
	}
	for _, provider := range c.ProviderChain {
		if strings.EqualFold(provider, "openai") {
			return true
		}
	}
	return false
}

func (c Config) validateVideoRouteProviderConfig() error {
	providers := []struct {
		enabled       bool
		switchEnv     string
		requiredValue string
		requiredEnv   string
		baseURL       string
		baseURLEnv    string
	}{
		{
			enabled:       c.APIMartProviderEnabled,
			switchEnv:     "APIMART_PROVIDER_ENABLED",
			requiredValue: c.APIMartAPIKey,
			requiredEnv:   "APIMART_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:       c.APIMartBaseURL,
			baseURLEnv:    "APIMART_BASE_URL",
		},
		{
			enabled:       c.PoYoProviderEnabled,
			switchEnv:     "POYO_PROVIDER_ENABLED",
			requiredValue: c.PoYoAPIKey,
			requiredEnv:   "POYO_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:       c.PoYoBaseURL,
			baseURLEnv:    "POYO_BASE_URL",
		},
	}
	for _, provider := range providers {
		if !provider.enabled {
			continue
		}
		if strings.TrimSpace(provider.requiredValue) == "" {
			return fmt.Errorf("config: %s=true requires %s", provider.switchEnv, provider.requiredEnv)
		}
		if strings.TrimSpace(provider.baseURL) == "" {
			return fmt.Errorf("config: %s=true requires %s", provider.switchEnv, provider.baseURLEnv)
		}
	}
	if c.FeatureImageModelNanoBanana2Enabled {
		if !c.PoYoProviderEnabled {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=true requires POYO_PROVIDER_ENABLED=true")
		}
		if strings.TrimSpace(c.PoYoAPIKey) == "" {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=true requires POYO_API_KEY")
		}
		if strings.TrimSpace(c.PoYoBaseURL) == "" {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED=true requires POYO_BASE_URL")
		}
	}
	if c.FeatureImageModelNanoBananaProEnabled {
		if !c.APIMartProviderEnabled {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=true requires APIMART_PROVIDER_ENABLED=true")
		}
		if strings.TrimSpace(c.APIMartAPIKey) == "" {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=true requires APIMART_API_KEY")
		}
		if strings.TrimSpace(c.APIMartBaseURL) == "" {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED=true requires APIMART_BASE_URL")
		}
	}
	if c.FeatureImageModelGPTImage2Enabled {
		if !c.APIMartProviderEnabled {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=true requires APIMART_PROVIDER_ENABLED=true")
		}
		if strings.TrimSpace(c.APIMartAPIKey) == "" {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=true requires APIMART_API_KEY")
		}
		if strings.TrimSpace(c.APIMartBaseURL) == "" {
			return fmt.Errorf("config: FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED=true requires APIMART_BASE_URL")
		}
	}

	routes := []struct {
		enabled           bool
		routeEnv          string
		providerEnabled   bool
		providerSwitchEnv string
		requiredValue     string
		requiredEnv       string
		baseURL           string
		baseURLEnv        string
	}{
		{
			enabled:           c.FeatureVideoRouteHailuo23FastEnabled,
			routeEnv:          "FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED",
			providerEnabled:   c.APIMartProviderEnabled,
			providerSwitchEnv: "APIMART_PROVIDER_ENABLED",
			requiredValue:     c.APIMartAPIKey,
			requiredEnv:       "APIMART_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:           c.APIMartBaseURL,
			baseURLEnv:        "APIMART_BASE_URL",
		},
		{
			enabled:           c.FeatureVideoRouteHailuo23StandardEnabled,
			routeEnv:          "FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED",
			providerEnabled:   c.APIMartProviderEnabled,
			providerSwitchEnv: "APIMART_PROVIDER_ENABLED",
			requiredValue:     c.APIMartAPIKey,
			requiredEnv:       "APIMART_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:           c.APIMartBaseURL,
			baseURLEnv:        "APIMART_BASE_URL",
		},
		{
			enabled:           c.FeatureVideoRouteKlingO3StandardEnabled,
			routeEnv:          "FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED",
			providerEnabled:   c.PoYoProviderEnabled,
			providerSwitchEnv: "POYO_PROVIDER_ENABLED",
			requiredValue:     c.PoYoAPIKey,
			requiredEnv:       "POYO_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:           c.PoYoBaseURL,
			baseURLEnv:        "POYO_BASE_URL",
		},
		{
			enabled:           c.FeatureVideoRouteRunwayGen4TurboEnabled,
			routeEnv:          "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED",
			providerEnabled:   c.RunwayProviderEnabled,
			providerSwitchEnv: "RUNWAY_PROVIDER_ENABLED",
			requiredValue:     c.RunwayMLAPISecret,
			requiredEnv:       "RUNWAYML_API_SECRET", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:           c.RunwayMLBaseURL,
			baseURLEnv:        "RUNWAYML_BASE_URL",
		},
		{
			enabled:           c.FeatureVideoRouteSeedance20FastEnabled,
			routeEnv:          "FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED",
			providerEnabled:   c.PoYoProviderEnabled,
			providerSwitchEnv: "POYO_PROVIDER_ENABLED",
			requiredValue:     c.PoYoAPIKey,
			requiredEnv:       "POYO_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:           c.PoYoBaseURL,
			baseURLEnv:        "POYO_BASE_URL",
		},
		{
			enabled:           c.FeatureVideoRouteRunwayGen45Enabled,
			routeEnv:          "FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED",
			providerEnabled:   c.PoYoProviderEnabled,
			providerSwitchEnv: "POYO_PROVIDER_ENABLED",
			requiredValue:     c.PoYoAPIKey,
			requiredEnv:       "POYO_API_KEY", // #nosec G101 -- env var name only; value is read from runtime config.
			baseURL:           c.PoYoBaseURL,
			baseURLEnv:        "POYO_BASE_URL",
		},
	}
	for _, route := range routes {
		if !route.enabled {
			continue
		}
		if !c.FeatureVideoRouterEnabled {
			return fmt.Errorf("config: %s=true requires FEATURE_VIDEO_ROUTER_ENABLED=true", route.routeEnv)
		}
		if !route.providerEnabled {
			return fmt.Errorf("config: %s=true requires %s=true", route.routeEnv, route.providerSwitchEnv)
		}
		if strings.TrimSpace(route.requiredValue) == "" {
			return fmt.Errorf("config: %s=true requires %s", route.routeEnv, route.requiredEnv)
		}
		if strings.TrimSpace(route.baseURL) == "" {
			return fmt.Errorf("config: %s=true requires %s", route.routeEnv, route.baseURLEnv)
		}
	}
	if c.FeatureVideoRouteResellerExperimentsEnabled && !c.FeatureVideoRouterEnabled {
		return fmt.Errorf("config: FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED=true requires FEATURE_VIDEO_ROUTER_ENABLED=true")
	}
	return nil
}

func (c Config) validateSelectedProviderSwitches() error {
	if c.usesProviderName("apimart") && !c.APIMartProviderEnabled {
		return fmt.Errorf("config: selected APIMart provider requires APIMART_PROVIDER_ENABLED=true")
	}
	if c.usesProviderName("poyo") && !c.PoYoProviderEnabled {
		return fmt.Errorf("config: selected PoYo provider requires POYO_PROVIDER_ENABLED=true")
	}
	if c.IsProduction() && c.usesProviderName("runway") && !c.RunwayProviderEnabled {
		return fmt.Errorf("config: selected Runway provider requires RUNWAY_PROVIDER_ENABLED=true")
	}
	return nil
}

func (c Config) usesDeepInfra() bool {
	if strings.EqualFold(c.Provider, "deepinfra") ||
		strings.EqualFold(c.ImageProvider, "deepinfra") ||
		strings.EqualFold(c.VideoProvider, "deepinfra") {
		return true
	}
	for _, provider := range c.ProviderChain {
		if strings.EqualFold(provider, "deepinfra") {
			return true
		}
	}
	return false
}

func (c Config) usesProviderName(name string) bool {
	if strings.EqualFold(c.Provider, name) ||
		strings.EqualFold(c.ImageProvider, name) ||
		strings.EqualFold(c.VideoProvider, name) {
		return true
	}
	for _, provider := range c.ProviderChain {
		if strings.EqualFold(provider, name) {
			return true
		}
	}
	return false
}

func (c Config) usesMockProvider() bool {
	if strings.EqualFold(c.Provider, "mock") ||
		strings.EqualFold(c.ImageProvider, "mock") ||
		strings.EqualFold(c.VideoProvider, "mock") {
		return true
	}
	for _, provider := range c.ProviderChain {
		if strings.EqualFold(provider, "mock") {
			return true
		}
	}
	return false
}

func (c Config) usesOnlyMockProviders() bool {
	providers := make([]string, 0, len(c.ProviderChain)+3)
	providers = append(providers, c.Provider)
	providers = append(providers, c.ProviderChain...)
	providers = append(providers, c.ImageProvider, c.VideoProvider)
	seenProvider := false
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		seenProvider = true
		if provider != "mock" {
			return false
		}
	}
	return seenProvider
}

func (c Config) usesRealGenerationProvider() bool {
	providers := make([]string, 0, len(c.ProviderChain)+3)
	providers = append(providers, c.Provider)
	providers = append(providers, c.ProviderChain...)
	providers = append(providers, c.ImageProvider, c.VideoProvider)
	for _, provider := range providers {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider != "" && provider != "mock" {
			return true
		}
	}
	return false
}

func validatePriceOverrides(overrides map[string]int64) error {
	for op, amount := range overrides {
		op = strings.TrimSpace(op)
		if op == "" {
			return fmt.Errorf("config: PRICES contains an empty operation")
		}
		if amount <= 0 {
			return fmt.Errorf("config: PRICES amount for %s must be positive", op)
		}
	}
	return nil
}

func knownProvider(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mock", "openai", "deepinfra", "apimart", "poyo", "runway":
		return true
	default:
		return false
	}
}

func knownProviderList() string {
	return "mock, openai, deepinfra, apimart, poyo, runway"
}

func knownArtifactScanner(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "none", "openai":
		return true
	default:
		return false
	}
}

func artifactScannerDisabled(name string) bool {
	scanner := strings.ToLower(strings.TrimSpace(name))
	return scanner == "" || scanner == "none"
}

func isProductionEnv(env string) bool {
	return strings.EqualFold(env, "production") || strings.EqualFold(env, "prod")
}

func isStagingEnv(env string) bool {
	return strings.EqualFold(env, "staging") || strings.EqualFold(env, "stage")
}

func isLoadTestEnv(env string) bool {
	normalized := strings.ToLower(strings.TrimSpace(env))
	return normalized == "loadtest" || normalized == "load-test" || normalized == "load_testing"
}

func (c Config) IsStaging() bool {
	return isStagingEnv(c.Env)
}

func (c Config) IsServerEnv() bool {
	return c.IsProduction() || c.IsStaging()
}

func defaultMediaVideoProbePolicy(env string, mediaPipelineEnabled bool) string {
	if isProductionEnv(env) || isStagingEnv(env) || mediaPipelineEnabled {
		return MediaVideoProbePolicyProbeRequired
	}
	return MediaVideoProbePolicyDisabled
}

func defaultMediaReferenceUploadsEnabled(env string) bool {
	return !isProductionEnv(env) && !isStagingEnv(env)
}

func defaultMediaDeliverRawProviderVideo(env string, mediaPipelineEnabled bool) string {
	if isProductionEnv(env) || isStagingEnv(env) || mediaPipelineEnabled {
		return MediaDeliverRawProviderVideoIfProbePassed
	}
	return MediaDeliverRawProviderVideoAlwaysDevOnly
}

func validateMediaVideoProbePolicy(policy string) error {
	switch policy {
	case MediaVideoProbePolicyDisabled, MediaVideoProbePolicyTrustedProvider, MediaVideoProbePolicyProbeRequired:
		return nil
	default:
		return fmt.Errorf("config: MEDIA_VIDEO_PROBE_POLICY must be disabled, trusted_provider, or probe_required")
	}
}

func validateMediaVideoTranscodePolicy(policy string) error {
	switch policy {
	case MediaVideoTranscodePolicyNever, MediaVideoTranscodePolicyFallback, MediaVideoTranscodePolicyAlways:
		return nil
	default:
		return fmt.Errorf("config: MEDIA_VIDEO_TRANSCODE_POLICY must be never, fallback, or always")
	}
}

func validateMediaDeliverRawProviderVideo(policy string) error {
	switch policy {
	case MediaDeliverRawProviderVideoNever, MediaDeliverRawProviderVideoIfProbePassed, MediaDeliverRawProviderVideoAlwaysDevOnly:
		return nil
	default:
		return fmt.Errorf("config: MEDIA_DELIVER_RAW_PROVIDER_VIDEO must be never, if_probe_passed, or always_dev_only")
	}
}

func knownPaymentProvider(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mock", "yookassa":
		return true
	default:
		return false
	}
}

func configTokenEquals(value, want string) bool {
	return strings.EqualFold(strings.TrimSpace(value), want)
}

func optionalConfigTokenEquals(value, want string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.EqualFold(value, want)
}

func providerChainMockOnly(providers []string) bool {
	for _, provider := range providers {
		provider = strings.TrimSpace(provider)
		if provider == "" {
			continue
		}
		if !strings.EqualFold(provider, "mock") {
			return false
		}
	}
	return true
}

func loadDotenv() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("_env")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envMode(key, def string) string {
	return normalizeMode(env(key, def))
}

func normalizeMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return DataServiceModeLocal
	}
	return value
}

func knownDataServiceMode(value string) bool {
	switch normalizeMode(value) {
	case DataServiceModeLocal, DataServiceModeExternal, DataServiceModeManaged:
		return true
	default:
		return false
	}
}

func knownS3AddressingStyle(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto", "path", "virtual-hosted", "virtual", "dns":
		return true
	default:
		return false
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt32(key string, def int32) int32 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envConfigToken(key, def string) string {
	return normalizeConfigToken(env(key, def))
}

// envPriceMap parses a comma-separated "op=amount" list into a price map.
func envPriceMap(key string) map[string]int64 {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	out := map[string]int64{}
	for _, pair := range strings.Split(v, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) != 2 {
			continue
		}
		op := strings.TrimSpace(kv[0])
		amount, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil || op == "" {
			continue
		}
		out[op] = amount
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func envList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func envNormalizedList(key string, def []string) []string {
	raw := envList(key)
	if len(raw) == 0 {
		raw = def
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		item = normalizeConfigToken(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func parseMediaProviderContracts(raw string) ([]domain.ProviderMediaContract, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var contracts []domain.ProviderMediaContract
	if err := dec.Decode(&contracts); err != nil {
		return nil, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("unexpected trailing JSON value")
		}
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, nil
	}
	for i := range contracts {
		contracts[i].AllowedAspectRatios = normalizeLooseList(contracts[i].AllowedAspectRatios)
		contracts[i].AllowedResolutions = normalizeLooseList(contracts[i].AllowedResolutions)
		contracts[i].ExpectedContainer = normalizeConfigToken(contracts[i].ExpectedContainer)
		contracts[i].ExpectedCodec = normalizeConfigToken(contracts[i].ExpectedCodec)
		contracts[i].ModelClass = normalizeConfigToken(contracts[i].ModelClass)
		if contracts[i].ProviderIdempotencyGuarantee == "" {
			contracts[i].ProviderIdempotencyGuarantee = domain.ProviderIdempotencyNone
		}
	}
	return contracts, nil
}

func normalizeLooseList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeConfigToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.' || r == '+':
			b.WriteRune(r)
		}
		if b.Len() >= 64 {
			break
		}
	}
	return strings.Trim(b.String(), "_.+-")
}

func validateNormalizedList(name string, values []string) error {
	for _, value := range values {
		if value == "" || normalizeConfigToken(value) != value {
			return fmt.Errorf("config: %s contains unsafe value %q", name, value)
		}
	}
	return nil
}

func validateIPOrCIDRList(name string, values []string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("config: %s contains empty value", name)
		}
		if ip := net.ParseIP(value); ip != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(value); err == nil {
			continue
		}
		return fmt.Errorf("config: %s contains invalid IP/CIDR %q", name, value)
	}
	return nil
}

func defaultStr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
