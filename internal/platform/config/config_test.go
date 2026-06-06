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

func TestLoadDeepInfraConfig(t *testing.T) {
	t.Setenv("PROVIDER", "deepinfra")
	t.Setenv("DEEPINFRA_API_KEY", "test-key")
	t.Setenv("DEEPINFRA_BASE_URL", "https://example.com/v1/openai")
	t.Setenv("DEEPINFRA_TEXT_MODEL", "deepseek-ai/DeepSeek-V4-Flash")
	t.Setenv("DEEPINFRA_TEXT_PRICE", "2")

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
}

func TestLoadReferralConfig(t *testing.T) {
	t.Setenv("VK_REFERRAL_LINK_BASE", "https://vk.com/im?sel=-1")
	t.Setenv("VK_REFERRAL_SHARE_BASE", "https://vk.com/share.php")
	t.Setenv("REFERRAL_CODE_LENGTH", "12")
	t.Setenv("REFERRAL_REFERRER_SIGNUP_REWARD_CREDITS", "15")
	t.Setenv("REFERRAL_REFERRED_SIGNUP_REWARD_CREDITS", "3")

	cfg := config.Load()
	if cfg.VKReferralLinkBase != "https://vk.com/im?sel=-1" || cfg.VKReferralShareBase != "https://vk.com/share.php" {
		t.Fatalf("unexpected referral links: base=%q share=%q", cfg.VKReferralLinkBase, cfg.VKReferralShareBase)
	}
	if cfg.ReferralCodeLength != 12 {
		t.Fatalf("ReferralCodeLength = %d, want 12", cfg.ReferralCodeLength)
	}
	if cfg.ReferralReferrerSignupRewardCredits != 15 || cfg.ReferralReferredSignupRewardCredits != 3 {
		t.Fatalf("unexpected referral rewards: referrer=%d referred=%d", cfg.ReferralReferrerSignupRewardCredits, cfg.ReferralReferredSignupRewardCredits)
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
	t.Setenv("VK_ANTISPAM_MESSAGE_LIMIT", "11")
	t.Setenv("VK_ANTISPAM_MESSAGE_WINDOW", "61s")
	t.Setenv("VK_ANTISPAM_GPT_LIMIT", "4")
	t.Setenv("VK_ANTISPAM_GPT_WINDOW", "31s")
	t.Setenv("VK_ANTISPAM_COOLDOWN", "32s")
	t.Setenv("VK_ANTISPAM_VIOLATION_LIMIT", "6")
	t.Setenv("VK_ANTISPAM_VIOLATION_WINDOW", "11m")
	t.Setenv("VK_ANTISPAM_BLOCK_DURATION", "16m")
	t.Setenv("VK_ANTISPAM_NEW_USER_AGE", "5h")
	t.Setenv("VK_ANTISPAM_NEW_USER_MESSAGE_LIMIT", "6")
	t.Setenv("VK_ANTISPAM_NEW_USER_GPT_LIMIT", "2")
	t.Setenv("VK_ANTISPAM_NEW_USER_GPT_WINDOW", "16s")
	t.Setenv("VK_ANTISPAM_ACTIVE_GPT_JOB_LIMIT", "3")

	cfg := config.Load()
	if cfg.VKAntiSpamEnabled {
		t.Fatal("VKAntiSpamEnabled = true, want false")
	}
	if cfg.VKAntiSpamMessageLimit != 11 || cfg.VKAntiSpamMessageWindow != 61*time.Second {
		t.Fatalf("unexpected message limit config: %d/%s", cfg.VKAntiSpamMessageLimit, cfg.VKAntiSpamMessageWindow)
	}
	if cfg.VKAntiSpamGPTLimit != 4 || cfg.VKAntiSpamGPTWindow != 31*time.Second {
		t.Fatalf("unexpected gpt limit config: %d/%s", cfg.VKAntiSpamGPTLimit, cfg.VKAntiSpamGPTWindow)
	}
	if cfg.VKAntiSpamCooldown != 32*time.Second || cfg.VKAntiSpamViolationLimit != 6 || cfg.VKAntiSpamViolationWindow != 11*time.Minute || cfg.VKAntiSpamBlockDuration != 16*time.Minute {
		t.Fatalf("unexpected violation config: cooldown=%s limit=%d window=%s block=%s", cfg.VKAntiSpamCooldown, cfg.VKAntiSpamViolationLimit, cfg.VKAntiSpamViolationWindow, cfg.VKAntiSpamBlockDuration)
	}
	if cfg.VKAntiSpamNewUserAge != 5*time.Hour || cfg.VKAntiSpamNewUserMessageLimit != 6 || cfg.VKAntiSpamNewUserGPTLimit != 2 || cfg.VKAntiSpamNewUserGPTWindow != 16*time.Second {
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
