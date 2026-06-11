// Package config loads service configuration from environment variables with
// sensible local-development defaults.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the full application configuration shared by the entrypoints.
type Config struct {
	// Env is the deployment environment ("development" or "production"). In
	// production the API fails closed when required secrets are missing.
	Env string

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

	VKConfirmationToken string
	VKSecret            string

	AdminToken string

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
	PaymentProvider                   string
	YooKassaShopID                    string
	YooKassaSecretKey                 string
	YooKassaBaseURL                   string
	YooKassaReturnURL                 string
	YooKassaWebhookIPAllowlistEnabled bool
	PaymentWebhookRequireHTTPS        bool
	PaymentWebhookAddr                string
	PaymentWebhookPollInterval        time.Duration
	PaymentWebhookBatchLimit          int
	PaymentReconciliationInterval     time.Duration
	PaymentReconciliationLimit        int
	PaymentReconciliationStaleAfter   time.Duration

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
	// "openai".
	ModerationProvider    string
	OpenAIModerationModel string
	ArtifactScanner       string

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
	VKMenuVideoEnabled                bool
	VKMenuImageEnabled                bool
	VKMenuGPTEnabled                  bool
	VKMenuStudentsEnabled             bool
	VKMenuAccountEnabled              bool
	VKMenuTopUpEnabled                bool
	VKMenuVideoSora2Enabled           bool
	VKMenuVideoSora2StartEnabled      bool
	VKMenuVideoSora2ExamplesEnabled   bool
	VKMenuVideoKling21Enabled         bool
	VKMenuVideoKling21StartEnabled    bool
	VKMenuVideoKling21ExamplesEnabled bool
	VKMenuVideoSeedance1Enabled       bool
	VKMenuVideoSeedance1LiteEnabled   bool
	VKMenuVideoSeedance1ProEnabled    bool
	VKMenuVideoHaiuo02Enabled         bool
	VKMenuVideoHaiuo02StandardEnabled bool
	VKMenuVideoHaiuo02FastEnabled     bool
	VKMenuImageTextEnabled            bool
	VKMenuImageReferenceEnabled       bool
	VKMenuStudentsSolverEnabled       bool
	VKMenuStudentsPresentationEnabled bool
	VKMenuStudentsReportEnabled       bool
	VKMenuStudentsQAEnabled           bool
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

	// WorkerProviderCallTimeout bounds one provider Submit/Poll call in workers.
	WorkerProviderCallTimeout time.Duration

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
	return strings.EqualFold(c.Env, "production") || strings.EqualFold(c.Env, "prod")
}

// PaymentWebhookHTTPSRequired reports whether the payment webhook receiver must
// reject requests that did not arrive over HTTPS or through a trusted HTTPS
// reverse proxy.
func (c Config) PaymentWebhookHTTPSRequired() bool {
	return c.PaymentWebhookRequireHTTPS || c.IsProduction()
}

// Validate fails closed: in production, secrets that protect inbound webhooks
// and the admin API must be set. Returns a descriptive error otherwise.
func (c Config) Validate() error {
	var missing []string
	if mode := strings.ToLower(strings.TrimSpace(c.VKMenuButtonMode)); mode != "" && mode != "callback" && mode != "text" {
		return fmt.Errorf("config: VK_MENU_BUTTON_MODE must be callback or text")
	}
	if mode := strings.ToLower(strings.TrimSpace(c.VKUnroutedTextMode)); mode != "" && mode != "reply" && mode != "silent" && mode != "gpt" {
		return fmt.Errorf("config: VK_UNROUTED_TEXT_MODE must be reply, silent, or gpt")
	}
	if mode := strings.ToLower(strings.TrimSpace(c.VKVideoDeliveryMode)); mode != "" && mode != "doc" && mode != "video" {
		return fmt.Errorf("config: VK_VIDEO_DELIVERY_MODE must be doc or video")
	}
	if provider := strings.ToLower(strings.TrimSpace(c.Provider)); provider != "" && !knownProvider(provider) {
		return fmt.Errorf("config: PROVIDER must be one of mock, openai, deepinfra")
	}
	for _, provider := range c.ProviderChain {
		if provider = strings.ToLower(strings.TrimSpace(provider)); provider != "" && !knownProvider(provider) {
			return fmt.Errorf("config: PROVIDER_CHAIN contains unknown provider %q; allowed: mock, openai, deepinfra", provider)
		}
	}
	if provider := strings.ToLower(strings.TrimSpace(c.ImageProvider)); provider != "" && !knownProvider(provider) {
		return fmt.Errorf("config: IMAGE_PROVIDER must be one of mock, openai, deepinfra")
	}
	if provider := strings.ToLower(strings.TrimSpace(c.VideoProvider)); provider != "" && !knownProvider(provider) {
		return fmt.Errorf("config: VIDEO_PROVIDER must be one of mock, openai, deepinfra")
	}
	if provider := strings.ToLower(strings.TrimSpace(c.PaymentProvider)); provider != "" && !knownPaymentProvider(provider) {
		return fmt.Errorf("config: PAYMENT_PROVIDER must be one of mock, yookassa")
	}
	if c.IsProduction() {
		if c.usesMockProvider() {
			return fmt.Errorf("config: mock provider is not allowed in production")
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
	}
	if c.usesOpenAI() && c.OpenAIAPIKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if c.usesDeepInfra() && c.DeepInfraAPIKey == "" {
		missing = append(missing, "DEEPINFRA_API_KEY")
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
	return nil
}

// Load reads configuration from .env/_env and the process environment.
func Load() Config {
	loadDotenv()

	host, _ := os.Hostname()
	provider := env("PROVIDER", "mock")
	providerChain := envList("PROVIDER_CHAIN")
	if len(providerChain) == 0 {
		providerChain = []string{provider}
	}
	return Config{
		Env:           env("APP_ENV", "development"),
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		DatabaseURL:   env("DATABASE_URL", "postgres://vk_ai_aggregator:vk_ai_aggregator@localhost:5432/vk_ai_aggregator?sslmode=disable"),
		MigrationsDir: env("MIGRATIONS_DIR", "migrations"),

		RedisAddr:     env("REDIS_ADDR", "localhost:6379"),
		RedisPassword: env("REDIS_PASSWORD", ""),
		RedisDB:       envInt("REDIS_DB", 0),

		S3Endpoint:  env("S3_ENDPOINT", "localhost:9000"),
		S3AccessKey: env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey: env("S3_SECRET_KEY", "minioadmin"),
		S3UseSSL:    envBool("S3_USE_SSL", false),
		S3Bucket:    env("S3_BUCKET", "artifacts"),

		VKConfirmationToken: env("VK_CONFIRMATION_TOKEN", "dev-confirmation"),
		VKSecret:            env("VK_SECRET", ""),

		AdminToken: env("ADMIN_TOKEN", ""),

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

		DBMaxConns:    int32(envInt("DB_MAX_CONNS", 10)),
		DBMinConns:    int32(envInt("DB_MIN_CONNS", 0)),
		RedisPoolSize: envInt("REDIS_POOL_SIZE", 10),
		StreamMaxLen:  int64(envInt("STREAM_MAX_LEN", 100000)),

		PriceOverrides: envPriceMap("PRICES"),
		MaxJobCost:     int64(envInt("MAX_JOB_COST", 0)),

		PaymentProvider:                   env("PAYMENT_PROVIDER", "mock"),
		YooKassaShopID:                    env("YOOKASSA_SHOP_ID", ""),
		YooKassaSecretKey:                 env("YOOKASSA_SECRET_KEY", ""),
		YooKassaBaseURL:                   env("YOOKASSA_BASE_URL", "https://api.yookassa.ru/v3"),
		YooKassaReturnURL:                 env("YOOKASSA_RETURN_URL", ""),
		YooKassaWebhookIPAllowlistEnabled: envBool("YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED", true),
		PaymentWebhookRequireHTTPS:        envBool("PAYMENT_WEBHOOK_REQUIRE_HTTPS", false),
		PaymentWebhookAddr:                env("PAYMENT_WEBHOOK_ADDR", ":8082"),
		PaymentWebhookPollInterval:        envDuration("PAYMENT_WEBHOOK_POLL_INTERVAL", 5*time.Second),
		PaymentWebhookBatchLimit:          envInt("PAYMENT_WEBHOOK_BATCH_LIMIT", 20),
		PaymentReconciliationInterval:     envDuration("PAYMENT_RECONCILIATION_INTERVAL", time.Minute),
		PaymentReconciliationLimit:        envInt("PAYMENT_RECONCILIATION_LIMIT", 100),
		PaymentReconciliationStaleAfter:   envDuration("PAYMENT_RECONCILIATION_STALE_AFTER", 30*time.Second),
		Provider:                          provider,
		ProviderChain:                     providerChain,
		ImageProvider:                     env("IMAGE_PROVIDER", ""),
		VideoProvider:                     env("VIDEO_PROVIDER", ""),
		ImageModel:                        env("IMAGE_MODEL", ""),
		ImageSize:                         env("IMAGE_SIZE", ""),
		VideoModel:                        env("VIDEO_MODEL", ""),
		VideoDurationSec:                  envInt("VIDEO_DURATION_SEC", 5),
		VideoResolution:                   env("VIDEO_RESOLUTION", "720p"),
		VideoAspectRatio:                  env("VIDEO_ASPECT_RATIO", "16:9"),
		VideoDraft:                        envBool("VIDEO_DRAFT", true),
		OpenAIAPIKey:                      env("OPENAI_API_KEY", ""),
		OpenAIBaseURL:                     env("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAITextModel:                   env("OPENAI_TEXT_MODEL", "gpt-4.1-mini"),
		OpenAIImageModel:                  env("OPENAI_IMAGE_MODEL", "gpt-image-1"),
		OpenAIImageSize:                   env("OPENAI_IMAGE_SIZE", "1024x1024"),
		OpenAIVideoModel:                  env("OPENAI_VIDEO_MODEL", "sora-2"),
		OpenAIVideoSeconds:                env("OPENAI_VIDEO_SECONDS", "4"),
		OpenAIVideoSize:                   env("OPENAI_VIDEO_SIZE", "720x1280"),
		OpenAITextPrice:                   int64(envInt("OPENAI_TEXT_PRICE", 1)),
		OpenAIImagePrice:                  int64(envInt("OPENAI_IMAGE_PRICE", 10)),
		OpenAIVideoPrice:                  int64(envInt("OPENAI_VIDEO_PRICE", 50)),
		DeepInfraAPIKey:                   env("DEEPINFRA_API_KEY", ""),
		DeepInfraBaseURL:                  env("DEEPINFRA_BASE_URL", "https://api.deepinfra.com/v1/openai"),
		DeepInfraTextModel:                env("DEEPINFRA_TEXT_MODEL", "deepseek-ai/DeepSeek-V4-Flash"),
		DeepInfraTextPrice:                int64(envInt("DEEPINFRA_TEXT_PRICE", 1)),
		DeepInfraImageModel:               env("DEEPINFRA_IMAGE_MODEL", "ByteDance/Seedream-4.5"),
		DeepInfraImageFallbackModel:       env("DEEPINFRA_IMAGE_FALLBACK_MODEL", ""),
		DeepInfraImagePrice:               int64(envInt("DEEPINFRA_IMAGE_PRICE", 10)),
		DeepInfraImageReferenceEnabled:    envBool("DEEPINFRA_IMAGE_REFERENCE_ENABLED", false),
		DeepInfraVideoModel:               env("DEEPINFRA_VIDEO_MODEL", "PrunaAI/p-video"),
		DeepInfraVideoDurationSec:         envInt("DEEPINFRA_VIDEO_DURATION_SEC", 5),
		DeepInfraVideoResolution:          env("DEEPINFRA_VIDEO_RESOLUTION", "720p"),
		DeepInfraVideoAspectRatio:         env("DEEPINFRA_VIDEO_ASPECT_RATIO", "16:9"),
		DeepInfraVideoDraft:               envBool("DEEPINFRA_VIDEO_DRAFT", true),
		DeepInfraVideoPrice:               int64(envInt("DEEPINFRA_VIDEO_PRICE", 10)),
		DeepInfraVideoHTTPTimeout:         envDuration("DEEPINFRA_VIDEO_HTTP_TIMEOUT", 180*time.Second),
		TextContextEnabled:                envBool("TEXT_CONTEXT_ENABLED", true),
		TextContextMaxInputTokens:         envInt("TEXT_CONTEXT_MAX_INPUT_TOKENS", 1600),
		TextContextMaxOutputTokens:        envInt("TEXT_CONTEXT_MAX_OUTPUT_TOKENS", 800),
		TextContextSummaryMaxTokens:       envInt("TEXT_CONTEXT_SUMMARY_MAX_TOKENS", 400),
		TextContextRecentMessagesLimit:    envInt("TEXT_CONTEXT_RECENT_MESSAGES_LIMIT", 6),
		TextContextSummarizeAfterMessages: envInt("TEXT_CONTEXT_SUMMARIZE_AFTER_MESSAGES", 10),
		TextContextSummarizeAfterTokens:   envInt("TEXT_CONTEXT_SUMMARIZE_AFTER_TOKENS", 1500),
		ModerationProvider:                env("MODERATION_PROVIDER", "keyword"),
		OpenAIModerationModel:             env("OPENAI_MODERATION_MODEL", "omni-moderation-latest"),
		ArtifactScanner:                   env("ARTIFACT_SCANNER", "none"),

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
		VKMenuVideoHaiuo02Enabled:         envBool("VK_MENU_VIDEO_HAIUO02_ENABLED", true),
		VKMenuVideoHaiuo02StandardEnabled: envBool("VK_MENU_VIDEO_HAIUO02_STANDARD_ENABLED", true),
		VKMenuVideoHaiuo02FastEnabled:     envBool("VK_MENU_VIDEO_HAIUO02_FAST_ENABLED", true),
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

		VKAppID:                         env("VK_APP_ID", ""),
		VKAppSecret:                     env("VK_APP_SECRET", ""),
		MiniAppLaunchParamsMaxAge:       envDuration("MINIAPP_LAUNCH_PARAMS_MAX_AGE", time.Hour),
		FrontendTelemetryEnabled:        envBool("FRONTEND_TELEMETRY_ENABLED", false),
		FrontendTelemetryUserHashSecret: env("FRONTEND_TELEMETRY_USER_HASH_SECRET", ""),

		ArtifactURLTTL:        envDuration("ARTIFACT_URL_TTL", time.Hour),
		SignedDelivery:        envBool("SIGNED_DELIVERY", false),
		ArtifactRetentionDays: envInt("ARTIFACT_RETENTION_DAYS", 0),

		WorkerProviderCallTimeout:     envDuration("WORKER_PROVIDER_CALL_TIMEOUT", 180*time.Second),
		WorkerShutdownGrace:           envDuration("WORKER_SHUTDOWN_GRACE", 30*time.Second),
		MaintenanceInterval:           envDuration("MAINTENANCE_INTERVAL", time.Hour),
		OutboxRetention:               envDuration("OUTBOX_RETENTION", 7*24*time.Hour),
		BillingReconciliationInterval: envDuration("BILLING_RECONCILIATION_INTERVAL", 5*time.Minute),
		BillingReconciliationLimit:    envInt("BILLING_RECONCILIATION_LIMIT", 100),

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

func knownProvider(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mock", "openai", "deepinfra":
		return true
	default:
		return false
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

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
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

func defaultStr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
