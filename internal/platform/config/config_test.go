package config_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
)

func TestValidateProductionSecrets(t *testing.T) {
	cfg := config.Config{Env: "production", VKConfirmationToken: "dev-confirmation"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, want := range []string{"VK_SECRET", "ADMIN_TOKEN", "VK_CONFIRMATION_TOKEN", "VK_APP_SECRET"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %s in validation error, got %q", want, msg)
		}
	}
}

func TestValidateRealModesRequireCredentialsOutsideProduction(t *testing.T) {
	cfg := config.Config{
		Env:                 "development",
		Provider:            "openai",
		VKDeliveryMode:      "real",
		VKConfirmationToken: "dev-confirmation",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, want := range []string{"OPENAI_API_KEY", "VK_ACCESS_TOKEN"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %s in validation error, got %q", want, msg)
		}
	}
}

func TestLoadProviderChain(t *testing.T) {
	t.Setenv("PROVIDER", "mock")
	t.Setenv("PROVIDER_CHAIN", "openai,mock")

	cfg := config.Load()
	if !reflect.DeepEqual(cfg.ProviderChain, []string{"openai", "mock"}) {
		t.Fatalf("provider chain = %#v", cfg.ProviderChain)
	}
}

func TestLoadDataServiceModesDefaultLocal(t *testing.T) {
	for _, key := range []string{"DATA_SERVICES_MODE", "POSTGRES_MODE", "REDIS_MODE", "S3_MODE"} {
		restore := clearEnv(t, key)
		defer restore()
	}

	cfg := config.Load()
	if cfg.DataServicesMode != config.DataServiceModeLocal ||
		cfg.PostgresMode != config.DataServiceModeLocal ||
		cfg.RedisMode != config.DataServiceModeLocal ||
		cfg.S3Mode != config.DataServiceModeLocal {
		t.Fatalf("unexpected data service defaults: data=%q postgres=%q redis=%q s3=%q",
			cfg.DataServicesMode, cfg.PostgresMode, cfg.RedisMode, cfg.S3Mode)
	}
}

func TestLoadDataServiceModesCanOverridePerService(t *testing.T) {
	t.Setenv("DATA_SERVICES_MODE", "managed")
	t.Setenv("POSTGRES_MODE", "external")
	t.Setenv("REDIS_MODE", "local")

	cfg := config.Load()
	if cfg.DataServicesMode != config.DataServiceModeManaged {
		t.Fatalf("DataServicesMode = %q, want managed", cfg.DataServicesMode)
	}
	if cfg.PostgresMode != config.DataServiceModeExternal {
		t.Fatalf("PostgresMode = %q, want external", cfg.PostgresMode)
	}
	if cfg.RedisMode != config.DataServiceModeLocal {
		t.Fatalf("RedisMode = %q, want local", cfg.RedisMode)
	}
	if cfg.S3Mode != config.DataServiceModeManaged {
		t.Fatalf("S3Mode = %q, want managed inherited from DATA_SERVICES_MODE", cfg.S3Mode)
	}
}

func TestLoadWorkerModeDefaultAll(t *testing.T) {
	restore := clearEnv(t, "WORKER_MODE")
	defer restore()

	cfg := config.Load()
	if cfg.WorkerMode != config.WorkerModeAll {
		t.Fatalf("WorkerMode = %q, want %q", cfg.WorkerMode, config.WorkerModeAll)
	}
}

func TestValidateWorkerModeRejectsUnknownValue(t *testing.T) {
	cfg := config.Config{
		DataServicesMode:    config.DataServiceModeLocal,
		PostgresMode:        config.DataServiceModeLocal,
		RedisMode:           config.DataServiceModeLocal,
		S3Mode:              config.DataServiceModeLocal,
		WorkerMode:          "vkbot",
		VKConfirmationToken: "dev-confirmation",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "WORKER_MODE") {
		t.Fatalf("expected WORKER_MODE validation error, got %v", err)
	}
}

func TestValidateDataServiceModesRejectsUnknownValue(t *testing.T) {
	cfg := config.Config{
		DataServicesMode: config.DataServiceModeLocal,
		PostgresMode:     "sidecar",
		RedisMode:        config.DataServiceModeLocal,
		S3Mode:           config.DataServiceModeLocal,
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "POSTGRES_MODE") {
		t.Fatalf("expected POSTGRES_MODE validation error, got %v", err)
	}
}

func TestLoadS3CompatibilityConfig(t *testing.T) {
	t.Setenv("S3_REGION", "ru-central1")
	t.Setenv("S3_ADDRESSING_STYLE", "virtual-hosted")

	cfg := config.Load()
	if cfg.S3Region != "ru-central1" {
		t.Fatalf("S3Region = %q, want ru-central1", cfg.S3Region)
	}
	if cfg.S3AddressingStyle != "virtual-hosted" {
		t.Fatalf("S3AddressingStyle = %q, want virtual-hosted", cfg.S3AddressingStyle)
	}
}

func TestValidateS3AddressingStyle(t *testing.T) {
	cfg := config.Config{
		DataServicesMode:    config.DataServiceModeLocal,
		PostgresMode:        config.DataServiceModeLocal,
		RedisMode:           config.DataServiceModeLocal,
		S3Mode:              config.DataServiceModeLocal,
		S3AddressingStyle:   "bucket-on-the-moon",
		VKConfirmationToken: "dev-confirmation",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "S3_ADDRESSING_STYLE") {
		t.Fatalf("expected S3_ADDRESSING_STYLE validation error, got %v", err)
	}
}

func TestLoadTracingOTLPConfig(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("OTEL_TRACES_SAMPLE_RATIO", "0.25")
	t.Setenv("OTEL_TRACES_CRITICAL_SAMPLE_RATIO", "1")

	cfg := config.Load()
	if cfg.TracingExporter != "otlp" {
		t.Fatalf("TracingExporter = %q, want otlp", cfg.TracingExporter)
	}
	if cfg.TracingOTLPEndpoint != "otel-collector:4317" {
		t.Fatalf("TracingOTLPEndpoint = %q", cfg.TracingOTLPEndpoint)
	}
	if cfg.TracingSampleRatio != 0.25 {
		t.Fatalf("TracingSampleRatio = %v, want 0.25", cfg.TracingSampleRatio)
	}
	if cfg.TracingCriticalSampleRatio != 1 {
		t.Fatalf("TracingCriticalSampleRatio = %v, want 1", cfg.TracingCriticalSampleRatio)
	}
}

func TestLoadFrontendTelemetryConfig(t *testing.T) {
	t.Setenv("FRONTEND_TELEMETRY_ENABLED", "true")
	t.Setenv("FRONTEND_TELEMETRY_USER_HASH_SECRET", "test-hash-secret")

	cfg := config.Load()
	if !cfg.FrontendTelemetryEnabled {
		t.Fatal("FrontendTelemetryEnabled = false, want true")
	}
	if cfg.FrontendTelemetryUserHashSecret != "test-hash-secret" {
		t.Fatalf("FrontendTelemetryUserHashSecret = %q", cfg.FrontendTelemetryUserHashSecret)
	}
}

func TestLoadTopUpFeatureFlags(t *testing.T) {
	t.Setenv("FEATURE_VK_TOPUP_STATUS_EDIT_ENABLED", "true")
	t.Setenv("FEATURE_MINIAPP_PAYMENT_CANCEL_ENABLED", "true")
	t.Setenv("FEATURE_MINIAPP_TOPUP_CATALOG_DROPDOWN_ENABLED", "true")
	t.Setenv("FEATURE_MINIAPP_DARK_THEME_ONLY_ENABLED", "true")
	t.Setenv("FEATURE_MINIAPP_TOPUP_HISTORY_DROPDOWN_ENABLED", "true")

	cfg := config.Load()
	if !cfg.FeatureVKTopUpStatusEditEnabled {
		t.Fatal("FeatureVKTopUpStatusEditEnabled = false, want true")
	}
	if !cfg.FeatureMiniAppPaymentCancelEnabled {
		t.Fatal("FeatureMiniAppPaymentCancelEnabled = false, want true")
	}
	if !cfg.FeatureMiniAppTopUpCatalogDropdownEnabled {
		t.Fatal("FeatureMiniAppTopUpCatalogDropdownEnabled = false, want true")
	}
	if !cfg.FeatureMiniAppDarkThemeOnlyEnabled {
		t.Fatal("FeatureMiniAppDarkThemeOnlyEnabled = false, want true")
	}
	if !cfg.FeatureMiniAppTopUpHistoryDropdownEnabled {
		t.Fatal("FeatureMiniAppTopUpHistoryDropdownEnabled = false, want true")
	}
}

func TestLoadVideoRouterFlagsDefaultDisabled(t *testing.T) {
	for _, key := range []string{
		"FEATURE_VIDEO_ROUTER_ENABLED",
		"FEATURE_VIDEO_ROUTE_HAILUO_2_3_FAST_ENABLED",
		"FEATURE_VIDEO_ROUTE_HAILUO_2_3_STANDARD_ENABLED",
		"FEATURE_VIDEO_ROUTE_KLING_O3_STANDARD_ENABLED",
		"FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_TURBO_ENABLED",
		"FEATURE_VIDEO_ROUTE_SEEDANCE_2_0_FAST_ENABLED",
		"FEATURE_VIDEO_ROUTE_RUNWAY_GEN4_5_ENABLED",
		"FEATURE_VIDEO_ROUTE_RESELLER_EXPERIMENTS_ENABLED",
		"FEATURE_IMAGE_MODEL_NANO_BANANA_PRO_ENABLED",
		"FEATURE_IMAGE_MODEL_GPT_IMAGE_2_ENABLED",
		"FEATURE_IMAGE_MODEL_NANO_BANANA_2_ENABLED",
		"APIMART_PROVIDER_ENABLED",
		"POYO_PROVIDER_ENABLED",
		"RUNWAY_PROVIDER_ENABLED",
	} {
		t.Setenv(key, "false")
	}

	cfg := config.Load()
	if cfg.FeatureVideoRouterEnabled ||
		cfg.FeatureVideoRouteHailuo23FastEnabled ||
		cfg.FeatureVideoRouteHailuo23StandardEnabled ||
		cfg.FeatureVideoRouteKlingO3StandardEnabled ||
		cfg.FeatureVideoRouteRunwayGen4TurboEnabled ||
		cfg.FeatureVideoRouteSeedance20FastEnabled ||
		cfg.FeatureVideoRouteRunwayGen45Enabled ||
		cfg.FeatureVideoRouteResellerExperimentsEnabled ||
		cfg.FeatureImageModelNanoBananaProEnabled ||
		cfg.FeatureImageModelGPTImage2Enabled ||
		cfg.FeatureImageModelNanoBanana2Enabled ||
		cfg.APIMartProviderEnabled ||
		cfg.PoYoProviderEnabled ||
		cfg.RunwayProviderEnabled {
		t.Fatal("video router/provider flags should default to disabled")
	}
}

func TestValidateImageModelNanoBananaProRequiresAPIMartConfig(t *testing.T) {
	cfg := config.Config{
		Env:                                   "development",
		Provider:                              "mock",
		ProviderChain:                         []string{"mock"},
		FeatureImageModelNanoBananaProEnabled: true,
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_PROVIDER_ENABLED") {
		t.Fatalf("expected APIMART_PROVIDER_ENABLED validation error, got %v", err)
	}

	cfg.APIMartProviderEnabled = true
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_API_KEY") {
		t.Fatalf("expected APIMART_API_KEY validation error, got %v", err)
	}

	cfg.APIMartAPIKey = "test-key"
	cfg.APIMartBaseURL = ""
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_BASE_URL") {
		t.Fatalf("expected APIMART_BASE_URL validation error, got %v", err)
	}

	cfg.APIMartBaseURL = "https://api.apimart.ai/v1"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateImageModelGPTImage2RequiresAPIMartConfig(t *testing.T) {
	cfg := config.Config{
		Env:                               "development",
		Provider:                          "mock",
		ProviderChain:                     []string{"mock"},
		FeatureImageModelGPTImage2Enabled: true,
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_PROVIDER_ENABLED") {
		t.Fatalf("expected APIMART_PROVIDER_ENABLED validation error, got %v", err)
	}

	cfg.APIMartProviderEnabled = true
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_API_KEY") {
		t.Fatalf("expected APIMART_API_KEY validation error, got %v", err)
	}

	cfg.APIMartAPIKey = "test-key"
	cfg.APIMartBaseURL = ""
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_BASE_URL") {
		t.Fatalf("expected APIMART_BASE_URL validation error, got %v", err)
	}

	cfg.APIMartBaseURL = "https://api.apimart.ai/v1"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateImageModelNanoBanana2RequiresPoYoConfig(t *testing.T) {
	cfg := config.Config{
		Env:                                 "development",
		Provider:                            "mock",
		ProviderChain:                       []string{"mock"},
		FeatureImageModelNanoBanana2Enabled: true,
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "POYO_PROVIDER_ENABLED") {
		t.Fatalf("expected POYO_PROVIDER_ENABLED validation error, got %v", err)
	}

	cfg.PoYoProviderEnabled = true
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "POYO_API_KEY") {
		t.Fatalf("expected POYO_API_KEY validation error, got %v", err)
	}

	cfg.PoYoAPIKey = "test-key"
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "POYO_BASE_URL") {
		t.Fatalf("expected POYO_BASE_URL validation error, got %v", err)
	}

	cfg.PoYoBaseURL = "https://api.poyo.ai"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateVideoRouteRequiresRouterFlag(t *testing.T) {
	cfg := config.Config{
		Env:                                  "development",
		Provider:                             "mock",
		ProviderChain:                        []string{"mock"},
		FeatureVideoRouteHailuo23FastEnabled: true,
		APIMartProviderEnabled:               true,
		APIMartAPIKey:                        "test-key",
		APIMartBaseURL:                       "https://example.test/v1",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "FEATURE_VIDEO_ROUTER_ENABLED") {
		t.Fatalf("expected FEATURE_VIDEO_ROUTER_ENABLED validation error, got %v", err)
	}
}

func TestValidateVideoRouteRequiresProviderKey(t *testing.T) {
	cfg := config.Config{
		Env:                                      "development",
		Provider:                                 "mock",
		ProviderChain:                            []string{"mock"},
		FeatureVideoRouterEnabled:                true,
		FeatureVideoRouteHailuo23StandardEnabled: true,
		APIMartProviderEnabled:                   true,
		APIMartBaseURL:                           "https://example.test/v1",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_API_KEY") {
		t.Fatalf("expected APIMART_API_KEY validation error, got %v", err)
	}
}

func TestValidateVideoRouteRequiresPoYoBaseURL(t *testing.T) {
	cfg := config.Config{
		Env:                                     "development",
		Provider:                                "mock",
		ProviderChain:                           []string{"mock"},
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteKlingO3StandardEnabled: true,
		PoYoProviderEnabled:                     true,
		PoYoAPIKey:                              "test-key",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "POYO_BASE_URL") {
		t.Fatalf("expected POYO_BASE_URL validation error, got %v", err)
	}
}

func TestValidateRunwayRouteRequiresSecret(t *testing.T) {
	cfg := config.Config{
		Env:                                     "development",
		Provider:                                "mock",
		ProviderChain:                           []string{"mock"},
		FeatureVideoRouterEnabled:               true,
		FeatureVideoRouteRunwayGen4TurboEnabled: true,
		RunwayProviderEnabled:                   true,
		RunwayMLBaseURL:                         "https://api.dev.runwayml.com/v1",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RUNWAYML_API_SECRET") {
		t.Fatalf("expected RUNWAYML_API_SECRET validation error, got %v", err)
	}
}

func TestValidateSelectedRunwayWithoutSecretDoesNotBlockDevelopment(t *testing.T) {
	cfg := config.Config{
		Env:                   "development",
		Provider:              "mock",
		ProviderChain:         []string{"runway", "mock"},
		RunwayProviderEnabled: true,
		RunwayMLBaseURL:       "https://api.dev.runwayml.com/v1",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("selected runway without key should be skipped by worker in development, got %v", err)
	}
}

func TestValidateSelectedRunwayRequiresSwitchInProduction(t *testing.T) {
	cfg := validProductionConfig()
	cfg.Provider = "runway"
	cfg.ProviderChain = []string{"runway"}
	cfg.RunwayProviderEnabled = false

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RUNWAY_PROVIDER_ENABLED") {
		t.Fatalf("expected RUNWAY_PROVIDER_ENABLED validation error, got %v", err)
	}
}

func TestValidateNewProviderSelectionRequiresCredentials(t *testing.T) {
	cfg := config.Config{
		Env:           "development",
		Provider:      "apimart",
		ProviderChain: []string{"apimart"},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_API_KEY") {
		t.Fatalf("expected APIMART_API_KEY validation error, got %v", err)
	}
}

func TestValidateNewProviderSelectionRequiresProviderSwitch(t *testing.T) {
	cfg := config.Config{
		Env:            "development",
		Provider:       "apimart",
		ProviderChain:  []string{"apimart"},
		APIMartAPIKey:  "test-key",
		APIMartBaseURL: "https://example.test/v1",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APIMART_PROVIDER_ENABLED") {
		t.Fatalf("expected APIMART_PROVIDER_ENABLED validation error, got %v", err)
	}
}

func TestLoadDBPoolConfigBoundsInt32Values(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "2147483648")
	t.Setenv("DB_MIN_CONNS", "7")

	cfg := config.Load()
	if cfg.DBMaxConns != 10 {
		t.Fatalf("DBMaxConns = %d, want default 10 for int32 overflow", cfg.DBMaxConns)
	}
	if cfg.DBMinConns != 7 {
		t.Fatalf("DBMinConns = %d, want 7", cfg.DBMinConns)
	}
}

func TestLoadImageProviderConfig(t *testing.T) {
	t.Setenv("IMAGE_PROVIDER", "openai")
	t.Setenv("IMAGE_MODEL", "gpt-image-2")
	t.Setenv("IMAGE_SIZE", "1024x1024")

	cfg := config.Load()
	if cfg.ImageProvider != "openai" {
		t.Fatalf("ImageProvider = %q, want openai", cfg.ImageProvider)
	}
	if cfg.ImageModel != "gpt-image-2" || cfg.ImageSize != "1024x1024" {
		t.Fatalf("unexpected image config: model=%q size=%q", cfg.ImageModel, cfg.ImageSize)
	}
}

func TestLoadVKVideoUploadConfig(t *testing.T) {
	t.Setenv("VK_VIDEO_ACCESS_TOKEN", "video-token")
	t.Setenv("VK_VIDEO_UPLOAD_GROUP_ID", "239332376")
	t.Setenv("VK_VIDEO_DELIVERY_MODE", "video")

	cfg := config.Load()
	if cfg.VKVideoAccessToken != "video-token" {
		t.Fatalf("VKVideoAccessToken = %q", cfg.VKVideoAccessToken)
	}
	if cfg.VKVideoUploadGroupID != 239332376 {
		t.Fatalf("VKVideoUploadGroupID = %d", cfg.VKVideoUploadGroupID)
	}
	if cfg.VKVideoDeliveryMode != "video" {
		t.Fatalf("VKVideoDeliveryMode = %q", cfg.VKVideoDeliveryMode)
	}
}

func TestLoadMediaPipelineConfig(t *testing.T) {
	t.Setenv("MEDIA_PIPELINE_ENABLED", "true")
	t.Setenv("MEDIA_VIDEO_PROBE_POLICY", "probe_required")
	t.Setenv("MEDIA_VIDEO_TRANSCODE_POLICY", "fallback")
	t.Setenv("MEDIA_DELIVER_RAW_PROVIDER_VIDEO", "if_probe_passed")
	t.Setenv("FFPROBE_PATH", "/opt/bin/ffprobe")
	t.Setenv("FFMPEG_PATH", "/opt/bin/ffmpeg")
	t.Setenv("MEDIA_MAX_VIDEO_SIZE_BYTES", "1048576")
	t.Setenv("MEDIA_MAX_VIDEO_DURATION_SEC", "45")
	t.Setenv("MEDIA_MAX_VIDEO_WIDTH", "1280")
	t.Setenv("MEDIA_MAX_VIDEO_HEIGHT", "720")
	t.Setenv("MEDIA_MAX_VIDEO_BITRATE", "6000000")
	t.Setenv("MEDIA_ALLOWED_VIDEO_CONTAINERS", "MP4, webm, mp4, !!!")
	t.Setenv("MEDIA_ALLOWED_VIDEO_CODECS", "H.264, VP9, vp9")
	t.Setenv("MEDIA_PROBE_TIMEOUT", "3s")
	t.Setenv("MEDIA_TRANSCODE_TIMEOUT", "4m")
	t.Setenv("MEDIA_MAX_CONCURRENT_PROBES", "3")
	t.Setenv("MEDIA_MAX_CONCURRENT_TRANSCODES", "2")
	t.Setenv("MEDIA_MAX_PENDING_VARIANTS", "24")
	t.Setenv("MEDIA_MAX_ACTIVE_VIDEO_JOBS_PER_USER", "2")
	t.Setenv("MEDIA_PROVIDER_MAX_ATTEMPTS_PER_JOB", "1")
	t.Setenv("MEDIA_PROVIDER_FALLBACK_BUDGET_PER_JOB", "0")
	t.Setenv("MEDIA_QUEUE_DEGRADE_THRESHOLD", "2500")
	t.Setenv("MEDIA_MAX_CONCURRENT_UPLOADS", "12")
	t.Setenv("MEDIA_REFERENCE_UPLOADS_ENABLED", "false")
	t.Setenv("MEDIA_REFERENCE_WEBP_ENABLED", "true")
	t.Setenv("MEDIA_MAX_IMAGE_UPLOAD_BYTES", "10485760")
	t.Setenv("MEDIA_MAX_IMAGE_WIDTH", "2048")
	t.Setenv("MEDIA_MAX_IMAGE_HEIGHT", "2048")
	t.Setenv("MEDIA_MAX_IMAGE_PIXELS", "4194304")
	t.Setenv("MEDIA_PROVIDER_QUALITY_GUARD_ENABLED", "true")
	t.Setenv("MEDIA_PROVIDER_QUALITY_DEGRADED_FAILURES", "4")
	t.Setenv("MEDIA_PROVIDER_QUALITY_DISABLED_FAILURES", "7")
	t.Setenv("MEDIA_PROVIDER_QUALITY_RECOVERY_SUCCESSES", "3")
	t.Setenv("ARTIFACT_RETENTION_DAYS", "7")
	t.Setenv("ARTIFACT_FREE_RETENTION_DAYS", "11")
	t.Setenv("ARTIFACT_PAID_RETENTION_DAYS", "370")
	t.Setenv("ARTIFACT_TEMP_RETENTION_DAYS", "2")
	t.Setenv("ARTIFACT_ORPHAN_RETENTION_DAYS", "5")
	t.Setenv("MEDIA_INPUT_RETENTION_DAYS", "14")
	t.Setenv("MEDIA_FAILED_RETENTION_DAYS", "3")
	t.Setenv("MEDIA_ORIGINAL_RETENTION_DAYS", "30")
	t.Setenv("MEDIA_VARIANT_RETENTION_DAYS", "21")
	t.Setenv("RETENTION_JOB_EVENTS_DAYS", "21")
	t.Setenv("RETENTION_PROVIDER_PAYLOAD_DAYS", "5")
	t.Setenv("RETENTION_VK_INBOUND_PAYLOAD_DAYS", "6")
	t.Setenv("VK_INBOUND_RETENTION_BATCH_SIZE", "175")
	t.Setenv("RETENTION_COMMAND_RAW_TEXT_DAYS", "8")
	t.Setenv("COMMAND_RETENTION_BATCH_SIZE", "95")
	t.Setenv("JOB_LOG_RETENTION_BATCH_SIZE", "125")
	t.Setenv("JOB_ERROR_AGGREGATE_LOOKBACK_DAYS", "14")
	t.Setenv("ANALYTICS_AGGREGATE_LOOKBACK_DAYS", "9")
	t.Setenv("RETENTION_CONVERSATION_MESSAGES_DAYS", "45")
	t.Setenv("RETENTION_CONVERSATION_SUMMARIES_DAYS", "120")
	t.Setenv("CONVERSATION_RETENTION_BATCH_SIZE", "250")

	cfg := config.Load()
	if !cfg.MediaPipelineEnabled {
		t.Fatal("MediaPipelineEnabled = false, want true")
	}
	if cfg.MediaVideoProbePolicy != config.MediaVideoProbePolicyProbeRequired {
		t.Fatalf("MediaVideoProbePolicy = %q", cfg.MediaVideoProbePolicy)
	}
	if cfg.MediaVideoTranscodePolicy != config.MediaVideoTranscodePolicyFallback {
		t.Fatalf("MediaVideoTranscodePolicy = %q", cfg.MediaVideoTranscodePolicy)
	}
	if cfg.MediaDeliverRawProviderVideo != config.MediaDeliverRawProviderVideoIfProbePassed {
		t.Fatalf("MediaDeliverRawProviderVideo = %q", cfg.MediaDeliverRawProviderVideo)
	}
	if cfg.FFProbePath != "/opt/bin/ffprobe" || cfg.FFmpegPath != "/opt/bin/ffmpeg" {
		t.Fatalf("unexpected tool paths: probe=%q ffmpeg=%q", cfg.FFProbePath, cfg.FFmpegPath)
	}
	if cfg.MediaMaxVideoSizeBytes != 1048576 || cfg.MediaMaxVideoDurationSec != 45 {
		t.Fatalf("unexpected media size/duration limits: size=%d duration=%d", cfg.MediaMaxVideoSizeBytes, cfg.MediaMaxVideoDurationSec)
	}
	if cfg.MediaMaxVideoWidth != 1280 || cfg.MediaMaxVideoHeight != 720 || cfg.MediaMaxVideoBitrate != 6000000 {
		t.Fatalf("unexpected media dimension/bitrate limits: %dx%d bitrate=%d", cfg.MediaMaxVideoWidth, cfg.MediaMaxVideoHeight, cfg.MediaMaxVideoBitrate)
	}
	if !reflect.DeepEqual(cfg.MediaAllowedVideoContainers, []string{"mp4", "webm"}) {
		t.Fatalf("containers = %#v", cfg.MediaAllowedVideoContainers)
	}
	if !reflect.DeepEqual(cfg.MediaAllowedVideoCodecs, []string{"h.264", "vp9"}) {
		t.Fatalf("codecs = %#v", cfg.MediaAllowedVideoCodecs)
	}
	if cfg.MediaProbeTimeout != 3*time.Second || cfg.MediaTranscodeTimeout != 4*time.Minute {
		t.Fatalf("unexpected media timeouts: probe=%s transcode=%s", cfg.MediaProbeTimeout, cfg.MediaTranscodeTimeout)
	}
	if cfg.MediaMaxConcurrentProbes != 3 || cfg.MediaMaxConcurrentTranscodes != 2 || cfg.MediaMaxPendingVariants != 24 {
		t.Fatalf("unexpected media concurrency limits: probes=%d transcodes=%d variants=%d", cfg.MediaMaxConcurrentProbes, cfg.MediaMaxConcurrentTranscodes, cfg.MediaMaxPendingVariants)
	}
	if cfg.MediaMaxActiveVideoJobsPerUser != 2 || cfg.MediaProviderMaxAttemptsPerJob != 1 || cfg.MediaProviderFallbackBudget != 0 {
		t.Fatalf("unexpected media job/provider limits: active=%d attempts=%d fallback=%d", cfg.MediaMaxActiveVideoJobsPerUser, cfg.MediaProviderMaxAttemptsPerJob, cfg.MediaProviderFallbackBudget)
	}
	if cfg.MediaQueueDegradeThreshold != 2500 || cfg.MediaMaxConcurrentUploads != 12 {
		t.Fatalf("unexpected media queue/upload limits: queue=%d uploads=%d", cfg.MediaQueueDegradeThreshold, cfg.MediaMaxConcurrentUploads)
	}
	if cfg.MediaReferenceUploadsEnabled || !cfg.MediaReferenceWebPEnabled ||
		cfg.MediaMaxImageUploadBytes != 10485760 ||
		cfg.MediaMaxImageWidth != 2048 ||
		cfg.MediaMaxImageHeight != 2048 ||
		cfg.MediaMaxImagePixels != 4194304 {
		t.Fatalf("unexpected media image upload config: enabled=%v webp=%v bytes=%d size=%dx%d pixels=%d",
			cfg.MediaReferenceUploadsEnabled,
			cfg.MediaReferenceWebPEnabled,
			cfg.MediaMaxImageUploadBytes,
			cfg.MediaMaxImageWidth,
			cfg.MediaMaxImageHeight,
			cfg.MediaMaxImagePixels)
	}
	if !cfg.MediaProviderQualityGuardEnabled ||
		cfg.MediaProviderQualityDegradedFailures != 4 ||
		cfg.MediaProviderQualityDisabledFailures != 7 ||
		cfg.MediaProviderQualityRecoverySuccesses != 3 {
		t.Fatalf("unexpected provider quality config: enabled=%v degraded=%d disabled=%d recovery=%d",
			cfg.MediaProviderQualityGuardEnabled,
			cfg.MediaProviderQualityDegradedFailures,
			cfg.MediaProviderQualityDisabledFailures,
			cfg.MediaProviderQualityRecoverySuccesses)
	}
	if cfg.ArtifactRetentionDays != 7 ||
		cfg.ArtifactFreeRetentionDays != 11 ||
		cfg.ArtifactPaidRetentionDays != 370 ||
		cfg.ArtifactTemporaryRetentionDays != 2 ||
		cfg.ArtifactOrphanRetentionDays != 5 ||
		cfg.MediaInputRetentionDays != 14 ||
		cfg.MediaFailedRetentionDays != 3 ||
		cfg.MediaOriginalRetentionDays != 30 ||
		cfg.MediaVariantRetentionDays != 21 {
		t.Fatalf("unexpected media retention config: artifact=%d free=%d paid=%d temp=%d orphan=%d input=%d failed=%d original=%d variant=%d",
			cfg.ArtifactRetentionDays,
			cfg.ArtifactFreeRetentionDays,
			cfg.ArtifactPaidRetentionDays,
			cfg.ArtifactTemporaryRetentionDays,
			cfg.ArtifactOrphanRetentionDays,
			cfg.MediaInputRetentionDays,
			cfg.MediaFailedRetentionDays,
			cfg.MediaOriginalRetentionDays,
			cfg.MediaVariantRetentionDays)
	}
	if cfg.JobEventsRetentionDays != 21 ||
		cfg.ProviderPayloadRetentionDays != 5 ||
		cfg.VKInboundPayloadRetentionDays != 6 ||
		cfg.VKInboundRetentionBatchSize != 175 ||
		cfg.CommandRawTextRetentionDays != 8 ||
		cfg.CommandRetentionBatchSize != 95 ||
		cfg.JobLogRetentionBatchSize != 125 ||
		cfg.JobErrorAggregateLookbackDays != 14 ||
		cfg.AnalyticsAggregateLookbackDays != 9 {
		t.Fatalf("unexpected job log retention config: events=%d payloads=%d vk_inbound_days=%d vk_inbound_batch=%d command_days=%d command_batch=%d batch=%d lookback=%d analytics=%d",
			cfg.JobEventsRetentionDays,
			cfg.ProviderPayloadRetentionDays,
			cfg.VKInboundPayloadRetentionDays,
			cfg.VKInboundRetentionBatchSize,
			cfg.CommandRawTextRetentionDays,
			cfg.CommandRetentionBatchSize,
			cfg.JobLogRetentionBatchSize,
			cfg.JobErrorAggregateLookbackDays,
			cfg.AnalyticsAggregateLookbackDays)
	}
	if cfg.ConversationMessageRetentionDays != 45 ||
		cfg.ConversationSummaryRetentionDays != 120 ||
		cfg.ConversationRetentionBatchSize != 250 {
		t.Fatalf("unexpected conversation retention config: messages=%d summaries=%d batch=%d",
			cfg.ConversationMessageRetentionDays,
			cfg.ConversationSummaryRetentionDays,
			cfg.ConversationRetentionBatchSize)
	}
}

func TestLoadMediaProviderContractsJSON(t *testing.T) {
	t.Setenv("MEDIA_PROVIDER_CONTRACTS_JSON", `[{
		"provider":"deepinfra",
		"model":"PrunaAI/p-video",
		"model_class":"deepinfra_video",
		"modality":"video",
		"allowed_durations_sec":[5],
		"allowed_aspect_ratios":["16:9","16:9"],
		"allowed_resolutions":["720P"],
		"expected_container":"MP4",
		"expected_codec":"H264",
		"expected_max_bytes":134217728,
		"delivery_ready_output":true,
		"requires_probe":true,
		"requires_transcode":false,
		"transcode_allowed":false,
		"supports_provider_idempotency":false,
		"provider_idempotency_guarantee":"none",
		"max_provider_attempts":1,
		"max_fallback_attempts":0,
		"max_provider_cost_credits":10
	}]`)

	cfg := config.Load()
	if len(cfg.MediaProviderContracts) != 1 {
		t.Fatalf("contracts = %d, want 1", len(cfg.MediaProviderContracts))
	}
	contract := cfg.MediaProviderContracts[0]
	if contract.Provider != domain.ProviderDeepInfra || contract.Model != "PrunaAI/p-video" || contract.Modality != domain.ModalityVideo {
		t.Fatalf("unexpected contract identity: %+v", contract)
	}
	if contract.ModelClass != "deepinfra_video" || contract.ExpectedContainer != "mp4" || contract.ExpectedCodec != "h264" {
		t.Fatalf("contract was not normalized safely: %+v", contract)
	}
	if !reflect.DeepEqual(contract.AllowedAspectRatios, []string{"16:9"}) || !reflect.DeepEqual(contract.AllowedResolutions, []string{"720p"}) {
		t.Fatalf("contract lists were not normalized: %+v", contract)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid media provider contract rejected: %v", err)
	}
}

func TestValidateMediaProviderContractsFailClosed(t *testing.T) {
	cfg := config.Config{MediaProviderContractsRaw: `[{"provider":"deepinfra","unknown":true}]`}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_PROVIDER_CONTRACTS_JSON") {
		t.Fatalf("expected JSON validation error, got %v", err)
	}

	cfg = config.Config{MediaProviderContracts: []domain.ProviderMediaContract{{
		Provider:            domain.ProviderDeepInfra,
		Model:               "PrunaAI/p-video",
		ModelClass:          "deepinfra_video",
		Modality:            domain.ModalityVideo,
		ExpectedContainer:   "mp4",
		ExpectedCodec:       "h264",
		ExpectedMaxBytes:    1,
		DeliveryReadyOutput: true,
		MaxProviderAttempts: 2,
	}}}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "provider idempotency") {
		t.Fatalf("expected retry-risk validation error, got %v", err)
	}
}

func TestLoadMediaPolicyDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("MEDIA_PIPELINE_ENABLED", "false")

	cfg := config.Load()
	if cfg.MediaVideoProbePolicy != config.MediaVideoProbePolicyDisabled {
		t.Fatalf("dev disabled probe policy = %q", cfg.MediaVideoProbePolicy)
	}
	if cfg.MediaVideoTranscodePolicy != config.MediaVideoTranscodePolicyNever {
		t.Fatalf("default transcode policy = %q", cfg.MediaVideoTranscodePolicy)
	}
	if cfg.MediaDeliverRawProviderVideo != config.MediaDeliverRawProviderVideoAlwaysDevOnly {
		t.Fatalf("dev raw provider video policy = %q", cfg.MediaDeliverRawProviderVideo)
	}
	if cfg.MediaMaxConcurrentUploads != 4 {
		t.Fatalf("default media upload concurrency = %d, want 4", cfg.MediaMaxConcurrentUploads)
	}

	t.Setenv("APP_ENV", "production")
	cfg = config.Load()
	if cfg.MediaVideoProbePolicy != config.MediaVideoProbePolicyProbeRequired {
		t.Fatalf("production probe policy = %q", cfg.MediaVideoProbePolicy)
	}
	if cfg.MediaDeliverRawProviderVideo != config.MediaDeliverRawProviderVideoIfProbePassed {
		t.Fatalf("production raw provider video policy = %q", cfg.MediaDeliverRawProviderVideo)
	}

	t.Setenv("APP_ENV", "staging")
	cfg = config.Load()
	if cfg.MediaVideoProbePolicy != config.MediaVideoProbePolicyProbeRequired {
		t.Fatalf("staging probe policy = %q", cfg.MediaVideoProbePolicy)
	}
	if cfg.MediaDeliverRawProviderVideo != config.MediaDeliverRawProviderVideoIfProbePassed {
		t.Fatalf("staging raw provider video policy = %q", cfg.MediaDeliverRawProviderVideo)
	}
	if cfg.MediaReferenceUploadsEnabled {
		t.Fatal("staging reference uploads should default to false")
	}
}

func TestValidateMediaPipelineDisabledAllowsMissingTools(t *testing.T) {
	cfg := config.Config{MediaPipelineEnabled: false}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("disabled media pipeline should not require tools: %v", err)
	}
}

func TestValidateMediaTranscodeNeverDoesNotRequireFFmpeg(t *testing.T) {
	cfg := validMediaPipelineConfig()
	cfg.FFmpegPath = ""
	cfg.MediaTranscodeTimeout = 0

	if err := cfg.Validate(); err != nil {
		t.Fatalf("transcode=never should not require ffmpeg: %v", err)
	}
}

func TestValidateMediaTranscodeFallbackRequiresFFmpegAndProbe(t *testing.T) {
	cfg := validMediaPipelineConfig()
	cfg.MediaVideoTranscodePolicy = config.MediaVideoTranscodePolicyFallback
	cfg.FFmpegPath = ""

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "FFMPEG_PATH") {
		t.Fatalf("expected FFMPEG_PATH validation error, got %v", err)
	}

	cfg.FFmpegPath = "ffmpeg"
	cfg.MediaTranscodeTimeout = 0
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_TRANSCODE_TIMEOUT") {
		t.Fatalf("expected MEDIA_TRANSCODE_TIMEOUT validation error, got %v", err)
	}

	cfg.MediaTranscodeTimeout = time.Second
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid fallback transcode config rejected: %v", err)
	}

	cfg.MediaVideoProbePolicy = config.MediaVideoProbePolicyDisabled
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_VIDEO_TRANSCODE_POLICY") {
		t.Fatalf("expected transcode/probe validation error, got %v", err)
	}
}

func TestValidateProductionMediaPoliciesFailClosed(t *testing.T) {
	cfg := validProductionConfig()
	cfg.MediaVideoProbePolicy = config.MediaVideoProbePolicyDisabled
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_VIDEO_PROBE_POLICY") {
		t.Fatalf("expected production probe policy validation error, got %v", err)
	}

	cfg = validProductionConfig()
	cfg.MediaVideoTranscodePolicy = config.MediaVideoTranscodePolicyAlways
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_VIDEO_TRANSCODE_POLICY=always") {
		t.Fatalf("expected production transcode policy validation error, got %v", err)
	}

	cfg = validProductionConfig()
	cfg.MediaDeliverRawProviderVideo = config.MediaDeliverRawProviderVideoAlwaysDevOnly
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_DELIVER_RAW_PROVIDER_VIDEO=always_dev_only") {
		t.Fatalf("expected production raw provider video validation error, got %v", err)
	}
}

func TestValidateTrustedProviderProbePolicyRequiresMockOnly(t *testing.T) {
	cfg := config.Config{
		Provider:              "deepinfra",
		ProviderChain:         []string{"deepinfra"},
		MediaVideoProbePolicy: config.MediaVideoProbePolicyTrustedProvider,
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "trusted_provider") {
		t.Fatalf("expected trusted_provider validation error, got %v", err)
	}

	cfg = config.Config{
		Provider:              "mock",
		ProviderChain:         []string{"mock"},
		MediaVideoProbePolicy: config.MediaVideoProbePolicyTrustedProvider,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mock-only trusted_provider config rejected: %v", err)
	}
}

func TestValidateMediaPipelineEnabledRequiresSafeBounds(t *testing.T) {
	cfg := config.Config{
		MediaPipelineEnabled:         true,
		FFProbePath:                  "ffprobe",
		FFmpegPath:                   "ffmpeg",
		MediaMaxVideoSizeBytes:       1,
		MediaMaxVideoDurationSec:     1,
		MediaMaxVideoWidth:           1,
		MediaMaxVideoHeight:          1,
		MediaMaxVideoBitrate:         1,
		MediaAllowedVideoContainers:  []string{"mp4"},
		MediaAllowedVideoCodecs:      []string{"h264"},
		MediaProbeTimeout:            time.Second,
		MediaTranscodeTimeout:        time.Second,
		MediaMaxConcurrentProbes:     1,
		MediaMaxConcurrentTranscodes: 1,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid media pipeline config rejected: %v", err)
	}

	cfg.MediaMaxVideoBitrate = 0
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_MAX_VIDEO_BITRATE") {
		t.Fatalf("expected bitrate validation error, got %v", err)
	}

	cfg.MediaMaxVideoBitrate = 1
	cfg.MediaAllowedVideoCodecs = []string{"H264"}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_ALLOWED_VIDEO_CODECS") {
		t.Fatalf("expected codec allowlist validation error, got %v", err)
	}
}

func TestValidateMediaScaleGuards(t *testing.T) {
	cfg := validMediaPipelineConfig()
	cfg.MediaMaxConcurrentProbes = 0
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_MAX_CONCURRENT_PROBES") {
		t.Fatalf("expected probe concurrency validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.MediaVideoTranscodePolicy = config.MediaVideoTranscodePolicyFallback
	cfg.MediaMaxConcurrentTranscodes = 0
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_MAX_CONCURRENT_TRANSCODES") {
		t.Fatalf("expected transcode concurrency validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.MediaQueueDegradeThreshold = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_QUEUE_DEGRADE_THRESHOLD") {
		t.Fatalf("expected queue threshold validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.MediaFailedRetentionDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_FAILED_RETENTION_DAYS") {
		t.Fatalf("expected retention validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.ConversationMessageRetentionDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RETENTION_CONVERSATION_MESSAGES_DAYS") {
		t.Fatalf("expected conversation retention validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.JobEventsRetentionDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RETENTION_JOB_EVENTS_DAYS") {
		t.Fatalf("expected job events retention validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.ProviderPayloadRetentionDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RETENTION_PROVIDER_PAYLOAD_DAYS") {
		t.Fatalf("expected provider payload retention validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.VKInboundPayloadRetentionDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RETENTION_VK_INBOUND_PAYLOAD_DAYS") {
		t.Fatalf("expected VK inbound retention validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.VKInboundRetentionBatchSize = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "VK_INBOUND_RETENTION_BATCH_SIZE") {
		t.Fatalf("expected VK inbound batch validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.CommandRawTextRetentionDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "RETENTION_COMMAND_RAW_TEXT_DAYS") {
		t.Fatalf("expected command raw text retention validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.CommandRetentionBatchSize = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "COMMAND_RETENTION_BATCH_SIZE") {
		t.Fatalf("expected command retention batch validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.JobLogRetentionBatchSize = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "JOB_LOG_RETENTION_BATCH_SIZE") {
		t.Fatalf("expected job log batch validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.JobErrorAggregateLookbackDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "JOB_ERROR_AGGREGATE_LOOKBACK_DAYS") {
		t.Fatalf("expected job error aggregate validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.AnalyticsAggregateLookbackDays = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ANALYTICS_AGGREGATE_LOOKBACK_DAYS") {
		t.Fatalf("expected analytics aggregate validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.MediaProviderQualityGuardEnabled = true
	cfg.MediaProviderQualityDegradedFailures = 5
	cfg.MediaProviderQualityDisabledFailures = 4
	cfg.MediaProviderQualityRecoverySuccesses = 1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_PROVIDER_QUALITY_DISABLED_FAILURES") {
		t.Fatalf("expected provider quality threshold validation error, got %v", err)
	}

	cfg = validMediaPipelineConfig()
	cfg.MediaMaxImagePixels = -1
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MEDIA_MAX_IMAGE_PIXELS") {
		t.Fatalf("expected image pixel validation error, got %v", err)
	}
}

func validMediaPipelineConfig() config.Config {
	return config.Config{
		MediaPipelineEnabled:         true,
		MediaVideoProbePolicy:        config.MediaVideoProbePolicyProbeRequired,
		MediaVideoTranscodePolicy:    config.MediaVideoTranscodePolicyNever,
		MediaDeliverRawProviderVideo: config.MediaDeliverRawProviderVideoIfProbePassed,
		FFProbePath:                  "ffprobe",
		FFmpegPath:                   "ffmpeg",
		MediaMaxVideoSizeBytes:       1,
		MediaMaxVideoDurationSec:     1,
		MediaMaxVideoWidth:           1,
		MediaMaxVideoHeight:          1,
		MediaMaxVideoBitrate:         1,
		MediaAllowedVideoContainers:  []string{"mp4"},
		MediaAllowedVideoCodecs:      []string{"h264"},
		MediaProbeTimeout:            time.Second,
		MediaTranscodeTimeout:        time.Second,
		MediaMaxConcurrentProbes:     1,
		MediaMaxConcurrentTranscodes: 1,
	}
}

func TestValidateVKVideoDeliveryMode(t *testing.T) {
	cfg := config.Config{VKVideoDeliveryMode: "bad"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "VK_VIDEO_DELIVERY_MODE") {
		t.Fatalf("expected VK_VIDEO_DELIVERY_MODE validation error, got %v", err)
	}
}

func TestValidateImageProvider(t *testing.T) {
	cfg := config.Config{ImageProvider: "unknown"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "IMAGE_PROVIDER") {
		t.Fatalf("expected IMAGE_PROVIDER validation error, got %v", err)
	}
}

func TestValidateProviderChainRejectsUnknownProvider(t *testing.T) {
	cfg := config.Config{ProviderChain: []string{"deepinfra", "unknown"}}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PROVIDER_CHAIN") {
		t.Fatalf("expected PROVIDER_CHAIN validation error, got %v", err)
	}
}

func TestValidateProductionRejectsMockProvider(t *testing.T) {
	cfg := validProductionConfig()
	cfg.ProviderChain = []string{"deepinfra", "mock"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "mock provider") {
		t.Fatalf("expected mock provider validation error, got %v", err)
	}
}

func TestValidateProductionRejectsMockPaymentProvider(t *testing.T) {
	cfg := validProductionConfig()
	cfg.PaymentProvider = "mock"

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_PROVIDER=mock") {
		t.Fatalf("expected mock payment provider validation error, got %v", err)
	}
}

func TestLoadDeepInfraConfig(t *testing.T) {
	t.Setenv("PROVIDER", "deepinfra")
	t.Setenv("DEEPINFRA_API_KEY", "test-key")
	t.Setenv("DEEPINFRA_BASE_URL", "https://example.com/v1/openai")
	t.Setenv("DEEPINFRA_TEXT_MODEL", "deepseek-ai/DeepSeek-V4-Flash")
	t.Setenv("DEEPINFRA_TEXT_PRICE", "2")
	t.Setenv("DEEPINFRA_IMAGE_MODEL", "ByteDance/Seedream-4.5")
	t.Setenv("DEEPINFRA_IMAGE_FALLBACK_MODEL", "stabilityai/sdxl-turbo")
	t.Setenv("DEEPINFRA_IMAGE_PRICE", "11")
	t.Setenv("DEEPINFRA_IMAGE_REFERENCE_ENABLED", "true")

	cfg := config.Load()
	if cfg.DeepInfraAPIKey != "test-key" {
		t.Fatal("DeepInfraAPIKey was not loaded")
	}
	if cfg.DeepInfraBaseURL != "https://example.com/v1/openai" {
		t.Fatalf("DeepInfraBaseURL = %q", cfg.DeepInfraBaseURL)
	}
	if cfg.DeepInfraTextModel != "deepseek-ai/DeepSeek-V4-Flash" {
		t.Fatalf("DeepInfraTextModel = %q", cfg.DeepInfraTextModel)
	}
	if cfg.DeepInfraTextPrice != 2 {
		t.Fatalf("DeepInfraTextPrice = %d, want 2", cfg.DeepInfraTextPrice)
	}
	if cfg.DeepInfraImageModel != "ByteDance/Seedream-4.5" {
		t.Fatalf("DeepInfraImageModel = %q", cfg.DeepInfraImageModel)
	}
	if cfg.DeepInfraImageFallbackModel != "stabilityai/sdxl-turbo" {
		t.Fatalf("DeepInfraImageFallbackModel = %q", cfg.DeepInfraImageFallbackModel)
	}
	if cfg.DeepInfraImagePrice != 11 {
		t.Fatalf("DeepInfraImagePrice = %d, want 11", cfg.DeepInfraImagePrice)
	}
	if !cfg.DeepInfraImageReferenceEnabled {
		t.Fatal("DeepInfraImageReferenceEnabled was not loaded")
	}
}

func validProductionConfig() config.Config {
	return config.Config{
		Env:                          "production",
		Provider:                     "deepinfra",
		ProviderChain:                []string{"deepinfra"},
		DeepInfraAPIKey:              "test-deepinfra-key",
		OpenAIAPIKey:                 "test-openai-key",
		ArtifactScanner:              "openai",
		VKSecret:                     "test-vk-secret",
		AdminToken:                   "test-admin-token",
		VKConfirmationToken:          "test-confirmation-token",
		VKAppSecret:                  "test-vk-app-secret",
		PaymentProvider:              "yookassa",
		YooKassaShopID:               "test-shop",
		YooKassaSecretKey:            "test-yookassa-secret",
		YooKassaReturnURL:            "https://example.com",
		PaymentWebhookTrustedProxies: []string{"127.0.0.1"},
	}
}

func TestLoadVKMenuButtonMode(t *testing.T) {
	t.Setenv("VK_MENU_BUTTON_MODE", "text")

	cfg := config.Load()
	if cfg.VKMenuButtonMode != "text" {
		t.Fatalf("VKMenuButtonMode = %q, want text", cfg.VKMenuButtonMode)
	}
}

func TestLoadVKUnroutedTextMode(t *testing.T) {
	t.Setenv("VK_UNROUTED_TEXT_MODE", "silent")

	cfg := config.Load()
	if cfg.VKUnroutedTextMode != "silent" {
		t.Fatalf("VKUnroutedTextMode = %q, want silent", cfg.VKUnroutedTextMode)
	}
}

func TestLoadVKDialogModeTTL(t *testing.T) {
	t.Setenv("VK_DIALOG_MODE_TTL", "45m")

	cfg := config.Load()
	if cfg.VKDialogModeTTL != 45*time.Minute {
		t.Fatalf("VKDialogModeTTL = %s, want 45m", cfg.VKDialogModeTTL)
	}
}

func TestLoadTextContextConfig(t *testing.T) {
	t.Setenv("TEXT_CONTEXT_ENABLED", "false")
	t.Setenv("TEXT_CONTEXT_MAX_INPUT_TOKENS", "1700")
	t.Setenv("TEXT_CONTEXT_MAX_OUTPUT_TOKENS", "700")
	t.Setenv("TEXT_CONTEXT_SUMMARY_MAX_TOKENS", "350")
	t.Setenv("TEXT_CONTEXT_RECENT_MESSAGES_LIMIT", "5")
	t.Setenv("TEXT_CONTEXT_SUMMARIZE_AFTER_MESSAGES", "9")
	t.Setenv("TEXT_CONTEXT_SUMMARIZE_AFTER_TOKENS", "1400")

	cfg := config.Load()
	if cfg.TextContextEnabled {
		t.Fatal("TextContextEnabled = true, want false")
	}
	if cfg.TextContextMaxInputTokens != 1700 || cfg.TextContextMaxOutputTokens != 700 || cfg.TextContextSummaryMaxTokens != 350 {
		t.Fatalf("unexpected context token config: input=%d output=%d summary=%d", cfg.TextContextMaxInputTokens, cfg.TextContextMaxOutputTokens, cfg.TextContextSummaryMaxTokens)
	}
	if cfg.TextContextRecentMessagesLimit != 5 || cfg.TextContextSummarizeAfterMessages != 9 || cfg.TextContextSummarizeAfterTokens != 1400 {
		t.Fatalf("unexpected context history config: recent=%d after_messages=%d after_tokens=%d", cfg.TextContextRecentMessagesLimit, cfg.TextContextSummarizeAfterMessages, cfg.TextContextSummarizeAfterTokens)
	}
}

func TestLoadVKMenuFeatureFlags(t *testing.T) {
	t.Setenv("VK_MENU_STUDENTS_ENABLED", "false")
	t.Setenv("VK_MENU_VIDEO_SORA2_EXAMPLES_ENABLED", "false")
	t.Setenv("VK_TOP_UP_RECEIPT_EMAIL", "payments@example.com")
	t.Setenv("VK_TOP_UP_RECEIPT_PHONE", "+79991234567")

	cfg := config.Load()
	if cfg.VKMenuStudentsEnabled {
		t.Fatal("VKMenuStudentsEnabled = true, want false")
	}
	if cfg.VKMenuVideoSora2ExamplesEnabled {
		t.Fatal("VKMenuVideoSora2ExamplesEnabled = true, want false")
	}
	if !cfg.VKMenuVideoEnabled {
		t.Fatal("VKMenuVideoEnabled = false, want default true")
	}
	if cfg.VKMenuAccountEnabled {
		t.Fatal("VKMenuAccountEnabled = true, want default false")
	}
	if cfg.VKMenuTopUpEnabled {
		t.Fatal("VKMenuTopUpEnabled = true, want default false")
	}
	if cfg.VKMenuImageTextEnabled {
		t.Fatal("VKMenuImageTextEnabled = true, want default false")
	}
	if cfg.VKMenuImageReferenceEnabled {
		t.Fatal("VKMenuImageReferenceEnabled = true, want default false")
	}
	if cfg.VKMenuVideoRoutesPreviewEnabled {
		t.Fatal("VKMenuVideoRoutesPreviewEnabled = true, want default false")
	}
	if cfg.VKTopUpReceiptEmail != "payments@example.com" || cfg.VKTopUpReceiptPhone != "+79991234567" {
		t.Fatalf("unexpected VK top-up receipt contact: email=%q phone=%q", cfg.VKTopUpReceiptEmail, cfg.VKTopUpReceiptPhone)
	}
}

func TestLoadVKMenuVideoRoutesPreviewFlag(t *testing.T) {
	t.Setenv("VK_MENU_VIDEO_ROUTES_PREVIEW_ENABLED", "true")

	cfg := config.Load()
	if !cfg.VKMenuVideoRoutesPreviewEnabled {
		t.Fatal("VKMenuVideoRoutesPreviewEnabled = false, want true")
	}
}

func TestLoadReferralConfig(t *testing.T) {
	t.Setenv("VK_REFERRAL_LINK_BASE", "https://vk.com/write-1")
	t.Setenv("VK_REFERRAL_SHARE_BASE", "https://vk.com/share.php")
	t.Setenv("REFERRAL_CODE_LENGTH", "12")
	t.Setenv("REFERRAL_REFERRER_SIGNUP_REWARD_CREDITS", "15")
	t.Setenv("REFERRAL_REFERRED_SIGNUP_REWARD_CREDITS", "3")
	t.Setenv("REFERRAL_REWARD_ON_ACTIVATION", "false")

	cfg := config.Load()
	if cfg.VKReferralLinkBase != "https://vk.com/write-1" || cfg.VKReferralShareBase != "https://vk.com/share.php" {
		t.Fatalf("unexpected referral links: base=%q share=%q", cfg.VKReferralLinkBase, cfg.VKReferralShareBase)
	}
	if cfg.ReferralCodeLength != 12 {
		t.Fatalf("ReferralCodeLength = %d, want 12", cfg.ReferralCodeLength)
	}
	if cfg.ReferralReferrerSignupRewardCredits != 15 || cfg.ReferralReferredSignupRewardCredits != 3 {
		t.Fatalf("unexpected referral rewards: referrer=%d referred=%d", cfg.ReferralReferrerSignupRewardCredits, cfg.ReferralReferredSignupRewardCredits)
	}
	if cfg.ReferralRewardOnActivation {
		t.Fatal("ReferralRewardOnActivation = true, want false")
	}
}

func TestReferralRewardOnActivationDefaultEnabled(t *testing.T) {
	restore := clearEnv(t, "REFERRAL_REWARD_ON_ACTIVATION")
	defer restore()

	cfg := config.Load()
	if !cfg.ReferralRewardOnActivation {
		t.Fatal("ReferralRewardOnActivation = false, want default true")
	}
}

func TestLoadPaymentConfig(t *testing.T) {
	t.Setenv("PUBLIC_VK_BASE_URL", "https://vk.neiirohub.ru")
	t.Setenv("PAYMENT_PROVIDER", "yookassa")
	t.Setenv("YOOKASSA_SHOP_ID", "shop-1")
	t.Setenv("YOOKASSA_SECRET_KEY", "secret")
	t.Setenv("YOOKASSA_BASE_URL", "https://example.com/v3")
	t.Setenv("YOOKASSA_RETURN_URL", "https://neiirohub.ru/payments/return")
	t.Setenv("YOOKASSA_RETURN_URL_MINIAPP", "https://vk.com/app54623372?section_type=public_r_app")
	t.Setenv("YOOKASSA_RETURN_URL_VK_BOT", "https://vk.com/write-239332376")
	t.Setenv("YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED", "true")
	t.Setenv("YOOKASSA_WEBHOOK_IP_ALLOWLIST", "203.0.113.0/24,198.51.100.10")
	t.Setenv("PAYMENT_WEBHOOK_REQUIRE_HTTPS", "true")
	t.Setenv("PAYMENT_WEBHOOK_TRUSTED_PROXIES", "10.0.0.0/8,127.0.0.1")
	t.Setenv("PAYMENT_WEBHOOK_ADDR", ":18082")
	t.Setenv("PAYMENT_WEBHOOK_POLL_INTERVAL", "2s")
	t.Setenv("PAYMENT_WEBHOOK_BATCH_LIMIT", "7")
	t.Setenv("PAYMENT_RECONCILIATION_INTERVAL", "3s")
	t.Setenv("PAYMENT_RECONCILIATION_LIMIT", "9")
	t.Setenv("PAYMENT_RECONCILIATION_STALE_AFTER", "4s")

	cfg := config.Load()
	if cfg.PaymentProvider != "yookassa" {
		t.Fatalf("PaymentProvider = %q, want yookassa", cfg.PaymentProvider)
	}
	if cfg.PublicVKBaseURL != "https://vk.neiirohub.ru" {
		t.Fatalf("PublicVKBaseURL = %q", cfg.PublicVKBaseURL)
	}
	if cfg.YooKassaShopID != "shop-1" || cfg.YooKassaSecretKey != "secret" {
		t.Fatalf("unexpected YooKassa credentials: shop=%q secret=%q", cfg.YooKassaShopID, cfg.YooKassaSecretKey)
	}
	if cfg.YooKassaBaseURL != "https://example.com/v3" || cfg.YooKassaReturnURL != "https://neiirohub.ru/payments/return" {
		t.Fatalf("unexpected YooKassa URLs: base=%q return=%q", cfg.YooKassaBaseURL, cfg.YooKassaReturnURL)
	}
	if cfg.YooKassaReturnURLMiniApp != "https://vk.com/app54623372?section_type=public_r_app" || cfg.YooKassaReturnURLVKBot != "https://vk.com/write-239332376" {
		t.Fatalf("unexpected surface YooKassa URLs: miniapp=%q vkbot=%q", cfg.YooKassaReturnURLMiniApp, cfg.YooKassaReturnURLVKBot)
	}
	if !cfg.YooKassaWebhookIPAllowlistEnabled {
		t.Fatal("YooKassaWebhookIPAllowlistEnabled = false, want true")
	}
	if got := strings.Join(cfg.YooKassaWebhookIPAllowlist, ","); got != "203.0.113.0/24,198.51.100.10" {
		t.Fatalf("YooKassaWebhookIPAllowlist = %q", got)
	}
	if !cfg.PaymentWebhookRequireHTTPS || !cfg.PaymentWebhookHTTPSRequired() {
		t.Fatal("payment webhook HTTPS requirement was not loaded")
	}
	if got := strings.Join(cfg.PaymentWebhookTrustedProxies, ","); got != "10.0.0.0/8,127.0.0.1" {
		t.Fatalf("PaymentWebhookTrustedProxies = %q", got)
	}
	if cfg.PaymentWebhookAddr != ":18082" || cfg.PaymentWebhookPollInterval.String() != "2s" || cfg.PaymentWebhookBatchLimit != 7 {
		t.Fatalf("unexpected webhook config: addr=%q interval=%s batch=%d", cfg.PaymentWebhookAddr, cfg.PaymentWebhookPollInterval, cfg.PaymentWebhookBatchLimit)
	}
	if cfg.PaymentReconciliationInterval.String() != "3s" || cfg.PaymentReconciliationLimit != 9 || cfg.PaymentReconciliationStaleAfter.String() != "4s" {
		t.Fatalf("unexpected payment reconciliation config: interval=%s limit=%d stale_after=%s", cfg.PaymentReconciliationInterval, cfg.PaymentReconciliationLimit, cfg.PaymentReconciliationStaleAfter)
	}
}

func TestPaymentWebhookHTTPSRequiredInProduction(t *testing.T) {
	cfg := config.Config{Env: "production"}
	if !cfg.PaymentWebhookHTTPSRequired() {
		t.Fatal("production payment webhooks must require HTTPS")
	}
}

func TestValidatePaymentWebhookAllowlistRequiresConfiguredRanges(t *testing.T) {
	cfg := config.Config{YooKassaWebhookIPAllowlistEnabled: true}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "YOOKASSA_WEBHOOK_IP_ALLOWLIST") {
		t.Fatalf("expected allowlist validation error, got %v", err)
	}

	cfg.YooKassaWebhookIPAllowlist = []string{"not-an-ip"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "invalid IP/CIDR") {
		t.Fatalf("expected invalid IP/CIDR validation error, got %v", err)
	}

	cfg.YooKassaWebhookIPAllowlist = []string{"203.0.113.0/24"}
	cfg.PaymentWebhookTrustedProxies = []string{"10.0.0.0/8"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid webhook ingress config rejected: %v", err)
	}
}

func TestPaymentWebhookHTTPSRequiredInStaging(t *testing.T) {
	cfg := config.Config{Env: "staging"}
	if !cfg.PaymentWebhookHTTPSRequired() {
		t.Fatal("staging payment webhooks must require HTTPS")
	}
}

func TestValidatePaymentProvider(t *testing.T) {
	cfg := config.Config{PaymentProvider: "stripe"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_PROVIDER") {
		t.Fatalf("expected PAYMENT_PROVIDER validation error, got %v", err)
	}
}

func TestValidateLoadTestAllowsOnlySafeMockModes(t *testing.T) {
	cfg := config.Config{
		Env:                "loadtest",
		Provider:           "mock",
		ProviderChain:      []string{"mock"},
		ImageProvider:      "mock",
		VideoProvider:      "mock",
		PaymentProvider:    "mock",
		VKDeliveryMode:     "mock",
		ModerationProvider: "keyword",
		ArtifactScanner:    "none",
	}

	if !cfg.IsLoadTest() {
		t.Fatal("IsLoadTest() = false, want true")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateLoadTestRejectsRealGenerationProviders(t *testing.T) {
	cfg := config.Config{
		Env:                "loadtest",
		Provider:           "deepinfra",
		ProviderChain:      []string{"deepinfra", "mock"},
		ImageProvider:      "mock",
		VideoProvider:      "mock",
		PaymentProvider:    "mock",
		VKDeliveryMode:     "mock",
		ModerationProvider: "keyword",
		ArtifactScanner:    "none",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APP_ENV=loadtest") || !strings.Contains(err.Error(), "PROVIDER=mock") || !strings.Contains(err.Error(), "PROVIDER_CHAIN=mock") {
		t.Fatalf("expected loadtest provider safety error, got %v", err)
	}
}

func TestValidateLoadTestRejectsRealPaymentProvider(t *testing.T) {
	cfg := config.Config{
		Env:                "loadtest",
		Provider:           "mock",
		ProviderChain:      []string{"mock"},
		PaymentProvider:    "yookassa",
		VKDeliveryMode:     "mock",
		ModerationProvider: "keyword",
		ArtifactScanner:    "none",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APP_ENV=loadtest") || !strings.Contains(err.Error(), "PAYMENT_PROVIDER=mock") {
		t.Fatalf("expected loadtest payment safety error, got %v", err)
	}
}

func TestValidateLoadTestRejectsRealVKDelivery(t *testing.T) {
	cfg := config.Config{
		Env:                "load-test",
		Provider:           "mock",
		ProviderChain:      []string{"mock"},
		PaymentProvider:    "mock",
		VKDeliveryMode:     "real",
		ModerationProvider: "keyword",
		ArtifactScanner:    "none",
	}

	if !cfg.IsLoadTest() {
		t.Fatal("IsLoadTest() = false, want true")
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "APP_ENV=loadtest") || !strings.Contains(err.Error(), "VK_DELIVERY_MODE=mock") {
		t.Fatalf("expected loadtest VK delivery safety error, got %v", err)
	}
}

func TestValidatePriceOverridesRejectNonPositiveAmounts(t *testing.T) {
	cfg := config.Config{PriceOverrides: map[string]int64{
		string(domain.OperationImageGenerate): -10,
	}}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PRICES") || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("expected PRICES positive validation error, got %v", err)
	}

	cfg.PriceOverrides = map[string]int64{
		string(domain.OperationTextGenerate): 0,
	}
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PRICES") || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("expected PRICES positive validation error, got %v", err)
	}
}

func TestValidateProductionRealProvidersRequireArtifactScanner(t *testing.T) {
	cfg := config.Config{
		Env:             "production",
		Provider:        "deepinfra",
		ProviderChain:   []string{"deepinfra"},
		ArtifactScanner: "none",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ARTIFACT_SCANNER=openai") {
		t.Fatalf("expected ARTIFACT_SCANNER production validation error, got %v", err)
	}

	cfg.ArtifactScanner = "openai"
	err = cfg.Validate()
	if err == nil || strings.Contains(err.Error(), "ARTIFACT_SCANNER=openai") {
		t.Fatalf("expected scanner guard to pass before other missing-secret errors, got %v", err)
	}
}

func TestValidateModerationSelectors(t *testing.T) {
	cfg := config.Config{ModerationProvider: "bad"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "MODERATION_PROVIDER") {
		t.Fatalf("expected MODERATION_PROVIDER validation error, got %v", err)
	}

	cfg = config.Config{ArtifactScanner: "bad"}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "ARTIFACT_SCANNER") {
		t.Fatalf("expected ARTIFACT_SCANNER validation error, got %v", err)
	}
}

func TestValidateYooKassaRequiresConfig(t *testing.T) {
	cfg := config.Config{Env: "development", PaymentProvider: "yookassa"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	for _, want := range []string{"YOOKASSA_SHOP_ID", "YOOKASSA_SECRET_KEY", "YOOKASSA_RETURN_URL"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %s in validation error, got %q", want, msg)
		}
	}
}

func TestValidateProductionYooKassaRequiresTrustedProxies(t *testing.T) {
	cfg := config.Config{
		Env:                 "production",
		PaymentProvider:     "yookassa",
		YooKassaShopID:      "shop",
		YooKassaSecretKey:   "secret",
		YooKassaReturnURL:   "https://app.example.com",
		VKSecret:            "vk",
		AdminToken:          "admin",
		VKConfirmationToken: "confirm",
		VKAppSecret:         "app",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_WEBHOOK_TRUSTED_PROXIES") {
		t.Fatalf("expected trusted proxy production validation error, got %v", err)
	}

	cfg.PaymentWebhookTrustedProxies = []string{"127.0.0.1"}
	err = cfg.Validate()
	if err != nil && strings.Contains(err.Error(), "PAYMENT_WEBHOOK_TRUSTED_PROXIES") {
		t.Fatalf("expected trusted proxy check to pass, got %v", err)
	}
}

func TestValidateVKMenuButtonMode(t *testing.T) {
	cfg := config.Config{VKMenuButtonMode: "bad"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "VK_MENU_BUTTON_MODE") {
		t.Fatalf("expected VK_MENU_BUTTON_MODE validation error, got %v", err)
	}
}

func TestValidateVKUnroutedTextMode(t *testing.T) {
	cfg := config.Config{VKUnroutedTextMode: "bad"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "VK_UNROUTED_TEXT_MODE") {
		t.Fatalf("expected VK_UNROUTED_TEXT_MODE validation error, got %v", err)
	}
}

func TestLoadReadsDotenvWithoutOverridingEnvironment(t *testing.T) {
	restoreEnv := clearEnv(t, "HTTP_ADDR")
	defer restoreEnv()
	restoreVKVersion := clearEnv(t, "VK_API_VERSION")
	defer restoreVKVersion()

	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/.env", []byte("HTTP_ADDR=:7777\nVK_API_VERSION=5.200\n"), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	t.Setenv("HTTP_ADDR", ":9999")
	cfg := config.Load()

	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("HTTPAddr = %q, want environment value", cfg.HTTPAddr)
	}
	if cfg.VKAPIVersion != "5.200" {
		t.Fatalf("VKAPIVersion = %q, want value from .env", cfg.VKAPIVersion)
	}
}

func TestLoadReadsUnderscoreEnvFallback(t *testing.T) {
	restoreEnv := clearEnv(t, "VK_API_VERSION")
	defer restoreEnv()

	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/_env", []byte("VK_API_VERSION=5.201\n"), 0600); err != nil {
		t.Fatalf("write _env: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	cfg := config.Load()

	if cfg.VKAPIVersion != "5.201" {
		t.Fatalf("VKAPIVersion = %q, want value from _env", cfg.VKAPIVersion)
	}
}

func TestLoadMiniAppJobRateLimit(t *testing.T) {
	t.Setenv("MINIAPP_JOB_RATE_LIMIT_RPS", "2.5")
	t.Setenv("MINIAPP_JOB_RATE_LIMIT_BURST", "7")
	t.Setenv("PAYMENT_REDIRECT_RATE_LIMIT_RPS", "3.5")
	t.Setenv("PAYMENT_REDIRECT_RATE_LIMIT_BURST", "11")

	cfg := config.Load()
	if cfg.MiniAppJobRateLimitRPS != 2.5 {
		t.Fatalf("MiniAppJobRateLimitRPS = %v", cfg.MiniAppJobRateLimitRPS)
	}
	if cfg.MiniAppJobRateLimitBurst != 7 {
		t.Fatalf("MiniAppJobRateLimitBurst = %v", cfg.MiniAppJobRateLimitBurst)
	}
	if cfg.PaymentRedirectRateLimitRPS != 3.5 {
		t.Fatalf("PaymentRedirectRateLimitRPS = %v", cfg.PaymentRedirectRateLimitRPS)
	}
	if cfg.PaymentRedirectRateLimitBurst != 11 {
		t.Fatalf("PaymentRedirectRateLimitBurst = %v", cfg.PaymentRedirectRateLimitBurst)
	}
}

func TestValidateProductionVKTopUpRequiresPublicRedirectBase(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "missing", base: ""},
		{name: "http", base: "http://vk.example.test"},
		{name: "with_query", base: "https://vk.example.test?x=1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := productionDeepInfraConfig()
			cfg.VKMenuTopUpEnabled = true
			cfg.PublicVKBaseURL = tt.base

			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "PUBLIC_VK_BASE_URL") {
				t.Fatalf("expected PUBLIC_VK_BASE_URL validation error, got %v", err)
			}
		})
	}
}

func TestValidateProductionVKTopUpAcceptsHTTPSPublicRedirectBase(t *testing.T) {
	cfg := productionDeepInfraConfig()
	cfg.VKMenuTopUpEnabled = true
	cfg.PublicVKBaseURL = "https://vk.example.test/pay"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadVKAntiSpamConfig(t *testing.T) {
	t.Setenv("VK_ANTISPAM_ENABLED", "false")
	t.Setenv("VK_ANTISPAM_MESSAGE_LIMIT", "41")
	t.Setenv("VK_ANTISPAM_MESSAGE_WINDOW", "61s")
	t.Setenv("VK_ANTISPAM_GPT_LIMIT", "4")
	t.Setenv("VK_ANTISPAM_GPT_WINDOW", "31s")
	t.Setenv("VK_ANTISPAM_IMAGE_DAILY_LIMIT", "101")
	t.Setenv("VK_ANTISPAM_IMAGE_DAILY_WINDOW", "25h")
	t.Setenv("VK_ANTISPAM_COOLDOWN", "32s")
	t.Setenv("VK_ANTISPAM_VIOLATION_LIMIT", "6")
	t.Setenv("VK_ANTISPAM_VIOLATION_WINDOW", "11m")
	t.Setenv("VK_ANTISPAM_BLOCK_DURATION", "16m")
	t.Setenv("VK_ANTISPAM_NEW_USER_AGE", "5h")
	t.Setenv("VK_ANTISPAM_NEW_USER_MESSAGE_LIMIT", "31")
	t.Setenv("VK_ANTISPAM_NEW_USER_GPT_LIMIT", "2")
	t.Setenv("VK_ANTISPAM_NEW_USER_GPT_WINDOW", "16s")
	t.Setenv("VK_ANTISPAM_ACTIVE_GPT_JOB_LIMIT", "3")

	cfg := config.Load()
	if cfg.VKAntiSpamEnabled {
		t.Fatal("VKAntiSpamEnabled = true, want false")
	}
	if cfg.VKAntiSpamMessageLimit != 41 || cfg.VKAntiSpamMessageWindow != 61*time.Second {
		t.Fatalf("unexpected message limit config: %d/%s", cfg.VKAntiSpamMessageLimit, cfg.VKAntiSpamMessageWindow)
	}
	if cfg.VKAntiSpamGPTLimit != 4 || cfg.VKAntiSpamGPTWindow != 31*time.Second {
		t.Fatalf("unexpected gpt limit config: %d/%s", cfg.VKAntiSpamGPTLimit, cfg.VKAntiSpamGPTWindow)
	}
	if cfg.VKAntiSpamImageDailyLimit != 101 || cfg.VKAntiSpamImageDailyWindow != 25*time.Hour {
		t.Fatalf("unexpected image daily limit config: %d/%s", cfg.VKAntiSpamImageDailyLimit, cfg.VKAntiSpamImageDailyWindow)
	}
	if cfg.VKAntiSpamCooldown != 32*time.Second || cfg.VKAntiSpamViolationLimit != 6 || cfg.VKAntiSpamViolationWindow != 11*time.Minute || cfg.VKAntiSpamBlockDuration != 16*time.Minute {
		t.Fatalf("unexpected violation config: cooldown=%s limit=%d window=%s block=%s", cfg.VKAntiSpamCooldown, cfg.VKAntiSpamViolationLimit, cfg.VKAntiSpamViolationWindow, cfg.VKAntiSpamBlockDuration)
	}
	if cfg.VKAntiSpamNewUserAge != 5*time.Hour || cfg.VKAntiSpamNewUserMessageLimit != 31 || cfg.VKAntiSpamNewUserGPTLimit != 2 || cfg.VKAntiSpamNewUserGPTWindow != 16*time.Second {
		t.Fatalf("unexpected new-user config: age=%s message=%d gpt=%d/%s", cfg.VKAntiSpamNewUserAge, cfg.VKAntiSpamNewUserMessageLimit, cfg.VKAntiSpamNewUserGPTLimit, cfg.VKAntiSpamNewUserGPTWindow)
	}
	if cfg.VKAntiSpamActiveGPTJobLimit != 3 {
		t.Fatalf("VKAntiSpamActiveGPTJobLimit = %d, want 3", cfg.VKAntiSpamActiveGPTJobLimit)
	}
}

func TestValidateOpenAIModerationRequiresKey(t *testing.T) {
	cfg := config.Config{
		Env:                "development",
		Provider:           "mock",
		ProviderChain:      []string{"mock"},
		ModerationProvider: "openai",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected OPENAI_API_KEY validation error, got %v", err)
	}
}

func TestValidateImageProviderRequiresKey(t *testing.T) {
	cfg := config.Config{
		Env:           "development",
		Provider:      "mock",
		ProviderChain: []string{"mock"},
		ImageProvider: "openai",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected OPENAI_API_KEY validation error, got %v", err)
	}
}

func TestValidateDeepInfraRequiresKey(t *testing.T) {
	cfg := config.Config{
		Env:           "development",
		Provider:      "mock",
		ProviderChain: []string{"deepinfra", "mock"},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DEEPINFRA_API_KEY") {
		t.Fatalf("expected DEEPINFRA_API_KEY validation error, got %v", err)
	}
}

func TestValidateDeepInfraImageProviderRequiresKey(t *testing.T) {
	cfg := config.Config{
		Env:           "development",
		Provider:      "mock",
		ProviderChain: []string{"mock"},
		ImageProvider: "deepinfra",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DEEPINFRA_API_KEY") {
		t.Fatalf("expected DEEPINFRA_API_KEY validation error, got %v", err)
	}
}

func TestValidateArtifactScannerKnownValues(t *testing.T) {
	cfg := config.Config{
		Env:             "development",
		Provider:        "mock",
		ProviderChain:   []string{"mock"},
		ArtifactScanner: "clamav",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ARTIFACT_SCANNER") {
		t.Fatalf("expected ARTIFACT_SCANNER validation error, got %v", err)
	}
}

func TestValidateProductionRequiresArtifactScanner(t *testing.T) {
	cfg := productionDeepInfraConfig()
	cfg.ArtifactScanner = "none"
	cfg.AllowUnscannedArtifactsInProduction = false

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "ARTIFACT_SCANNER=openai") {
		t.Fatalf("expected production artifact scanner validation error, got %v", err)
	}
}

func TestValidateProductionAllowsUnscannedArtifactsWithExplicitFlag(t *testing.T) {
	cfg := productionDeepInfraConfig()
	cfg.ArtifactScanner = "none"
	cfg.OpenAIAPIKey = ""
	cfg.AllowUnscannedArtifactsInProduction = true

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateStagingAllowsUnscannedArtifactsWithoutOpenAI(t *testing.T) {
	cfg := productionDeepInfraConfig()
	cfg.Env = "staging"
	cfg.ArtifactScanner = "none"
	cfg.OpenAIAPIKey = ""

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateOpenAIArtifactScannerRequiresKey(t *testing.T) {
	cfg := config.Config{
		Env:             "development",
		Provider:        "mock",
		ProviderChain:   []string{"mock"},
		ArtifactScanner: "openai",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected OPENAI_API_KEY validation error, got %v", err)
	}
}

func TestLoadAllowUnscannedArtifactsInProduction(t *testing.T) {
	t.Setenv("ALLOW_UNSCANNED_ARTIFACTS_IN_PRODUCTION", "true")

	cfg := config.Load()
	if !cfg.AllowUnscannedArtifactsInProduction {
		t.Fatal("AllowUnscannedArtifactsInProduction = false, want true")
	}
}

func productionDeepInfraConfig() config.Config {
	return config.Config{
		Env:                          "production",
		VKConfirmationToken:          "prod-confirmation",
		VKSecret:                     "vk-secret",
		VKAppSecret:                  "vk-app-secret",
		AdminToken:                   "admin-token",
		PaymentProvider:              "yookassa",
		YooKassaShopID:               "shop-id",
		YooKassaSecretKey:            "secret-key",
		YooKassaReturnURL:            "https://neiirohub.ru/payment-return",
		PaymentWebhookTrustedProxies: []string{"127.0.0.1"},
		Provider:                     "deepinfra",
		ProviderChain:                []string{"deepinfra"},
		DeepInfraAPIKey:              "deepinfra-key",
		ArtifactScanner:              "openai",
		OpenAIAPIKey:                 "openai-key",
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func clearEnv(t *testing.T, key string) func() {
	t.Helper()
	old, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	return func() {
		if ok {
			_ = os.Setenv(key, old)
			return
		}
		_ = os.Unsetenv(key)
	}
}
