package config_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

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
		Env:                 "production",
		Provider:            "deepinfra",
		ProviderChain:       []string{"deepinfra"},
		DeepInfraAPIKey:     "test-deepinfra-key",
		VKSecret:            "test-vk-secret",
		AdminToken:          "test-admin-token",
		VKConfirmationToken: "test-confirmation-token",
		VKAppSecret:         "test-vk-app-secret",
		PaymentProvider:     "yookassa",
		YooKassaShopID:      "test-shop",
		YooKassaSecretKey:   "test-yookassa-secret",
		YooKassaReturnURL:   "https://example.com",
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
	if cfg.VKTopUpReceiptEmail != "payments@example.com" || cfg.VKTopUpReceiptPhone != "+79991234567" {
		t.Fatalf("unexpected VK top-up receipt contact: email=%q phone=%q", cfg.VKTopUpReceiptEmail, cfg.VKTopUpReceiptPhone)
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
	t.Setenv("PAYMENT_PROVIDER", "yookassa")
	t.Setenv("YOOKASSA_SHOP_ID", "shop-1")
	t.Setenv("YOOKASSA_SECRET_KEY", "secret")
	t.Setenv("YOOKASSA_BASE_URL", "https://example.com/v3")
	t.Setenv("YOOKASSA_RETURN_URL", "https://neiirohub.ru/payments/return")
	t.Setenv("YOOKASSA_WEBHOOK_IP_ALLOWLIST_ENABLED", "false")
	t.Setenv("PAYMENT_WEBHOOK_REQUIRE_HTTPS", "true")
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
	if cfg.YooKassaShopID != "shop-1" || cfg.YooKassaSecretKey != "secret" {
		t.Fatalf("unexpected YooKassa credentials: shop=%q secret=%q", cfg.YooKassaShopID, cfg.YooKassaSecretKey)
	}
	if cfg.YooKassaBaseURL != "https://example.com/v3" || cfg.YooKassaReturnURL != "https://neiirohub.ru/payments/return" {
		t.Fatalf("unexpected YooKassa URLs: base=%q return=%q", cfg.YooKassaBaseURL, cfg.YooKassaReturnURL)
	}
	if cfg.YooKassaWebhookIPAllowlistEnabled {
		t.Fatal("YooKassaWebhookIPAllowlistEnabled = true, want false")
	}
	if !cfg.PaymentWebhookRequireHTTPS || !cfg.PaymentWebhookHTTPSRequired() {
		t.Fatal("payment webhook HTTPS requirement was not loaded")
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

func TestValidatePaymentProvider(t *testing.T) {
	cfg := config.Config{PaymentProvider: "stripe"}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "PAYMENT_PROVIDER") {
		t.Fatalf("expected PAYMENT_PROVIDER validation error, got %v", err)
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

	cfg := config.Load()
	if cfg.MiniAppJobRateLimitRPS != 2.5 {
		t.Fatalf("MiniAppJobRateLimitRPS = %v", cfg.MiniAppJobRateLimitRPS)
	}
	if cfg.MiniAppJobRateLimitBurst != 7 {
		t.Fatalf("MiniAppJobRateLimitBurst = %v", cfg.MiniAppJobRateLimitBurst)
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
