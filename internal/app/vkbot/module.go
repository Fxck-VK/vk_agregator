// Package vkbot wires the VK text bot app surface onto the shared backend core.
package vkbot

import (
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	vkinbound "vk-ai-aggregator/internal/adapter/inbound/vk"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/dialogstate"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/referralservice"
)

// Deps are shared backend-core collaborators required by the VK bot surface.
type Deps struct {
	Redis        *redis.Client
	Idempotency  domain.IdempotencyRepository
	Inbound      domain.InboundEventRepository
	Users        domain.UserRepository
	Jobs         domain.JobRepository
	Commands     domain.CommandRepository
	Billing      *billingservice.Service
	Payment      *paymentservice.Service
	Referrals    domain.ReferralRepository
	Orchestrator *joborchestrator.Orchestrator
	Router       *commandrouter.Router
	Logger       *slog.Logger
}

// NewHandler builds the VK callback HTTP handler without owning core business
// decisions. Provider calls, billing mutations and job processing remain in the
// shared orchestrator/worker/billing layers.
func NewHandler(cfg config.Config, deps Deps) http.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	vkDialogState := dialogstate.New(redisqueue.NewDialogStateStore(deps.Redis), dialogstate.Config{
		TTL: cfg.VKDialogModeTTL,
	})
	vkAntiSpam := antispam.New(redisqueue.NewAntiSpamStore(deps.Redis), deps.Jobs, antispam.Config{
		Enabled:             cfg.VKAntiSpamEnabled,
		MessageLimit:        cfg.VKAntiSpamMessageLimit,
		MessageWindow:       cfg.VKAntiSpamMessageWindow,
		GPTLimit:            cfg.VKAntiSpamGPTLimit,
		GPTWindow:           cfg.VKAntiSpamGPTWindow,
		ImageDailyLimit:     cfg.VKAntiSpamImageDailyLimit,
		ImageDailyWindow:    cfg.VKAntiSpamImageDailyWindow,
		Cooldown:            cfg.VKAntiSpamCooldown,
		ViolationLimit:      cfg.VKAntiSpamViolationLimit,
		ViolationWindow:     cfg.VKAntiSpamViolationWindow,
		BlockDuration:       cfg.VKAntiSpamBlockDuration,
		NewUserAge:          cfg.VKAntiSpamNewUserAge,
		NewUserMessageLimit: cfg.VKAntiSpamNewUserMessageLimit,
		NewUserGPTLimit:     cfg.VKAntiSpamNewUserGPTLimit,
		NewUserGPTWindow:    cfg.VKAntiSpamNewUserGPTWindow,
		ActiveGPTJobLimit:   cfg.VKAntiSpamActiveGPTJobLimit,
	})

	var vkControl vkdelivery.ControlClient
	var vkProfile vkdelivery.UserProfileClient
	if cfg.VKAccessToken != "" {
		vkClient := vkdelivery.NewHTTPClient(vkdelivery.HTTPConfig{
			AccessToken: cfg.VKAccessToken,
			APIVersion:  cfg.VKAPIVersion,
			BaseURL:     cfg.VKAPIBaseURL,
		})
		vkControl = vkClient
		vkProfile = vkClient
		logger.Info("using real vk control delivery client")
	} else {
		logger.Warn("vk control responses disabled because VK_ACCESS_TOKEN is empty")
	}

	referrals := referralservice.New(deps.Referrals, deps.Billing, referralservice.Config{
		CodeLength:                  cfg.ReferralCodeLength,
		ReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		ReferredSignupRewardCredits: cfg.ReferralReferredSignupRewardCredits,
	})

	return vkinbound.NewHandler(vkinbound.Config{
		ConfirmationToken:                   cfg.VKConfirmationToken,
		Secret:                              cfg.VKSecret,
		WelcomeAttachment:                   cfg.VKWelcomeAttachment,
		MenuButtonMode:                      cfg.VKMenuButtonMode,
		UnroutedTextMode:                    cfg.VKUnroutedTextMode,
		MenuFeatures:                        menuFeatures(cfg),
		ReferralLinkBase:                    cfg.VKReferralLinkBase,
		ReferralShareBase:                   cfg.VKReferralShareBase,
		ReferralReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		TopUpReceiptEmail:                   cfg.VKTopUpReceiptEmail,
		TopUpReceiptPhone:                   cfg.VKTopUpReceiptPhone,
	}, vkinbound.Deps{
		Idempotency:  deps.Idempotency,
		Inbound:      deps.Inbound,
		Users:        deps.Users,
		Jobs:         deps.Jobs,
		Commands:     deps.Commands,
		Billing:      deps.Billing,
		Payment:      deps.Payment,
		Referrals:    referrals,
		Orchestrator: deps.Orchestrator,
		Router:       deps.Router,
		Control:      vkControl,
		Profile:      vkProfile,
		DialogState:  vkDialogState,
		AntiSpam:     vkAntiSpam,
		Logger:       logger,
	})
}

func menuFeatures(cfg config.Config) vkinbound.MenuFeatureFlags {
	disabled := map[domain.CommandType]bool{}
	disableWhenFalse := func(enabled bool, commands ...domain.CommandType) {
		if enabled {
			return
		}
		for _, command := range commands {
			disabled[command] = true
		}
	}

	disableWhenFalse(cfg.VKMenuVideoEnabled, domain.CommandMenuVideo)
	disableWhenFalse(cfg.VKMenuImageEnabled, domain.CommandMenuImage)
	disableWhenFalse(cfg.VKMenuGPTEnabled, domain.CommandMenuText)
	disableWhenFalse(cfg.VKMenuStudentsEnabled, domain.CommandMenuStudents)
	disableWhenFalse(cfg.VKMenuAccountEnabled, domain.CommandAccount)
	disableWhenFalse(cfg.VKMenuTopUpEnabled, domain.CommandTopUp)
	disableWhenFalse(cfg.VKMenuVideoSora2Enabled, domain.CommandMenuVideoSora2)
	disableWhenFalse(cfg.VKMenuVideoSora2StartEnabled, domain.CommandMenuVideoSora2Start)
	disableWhenFalse(cfg.VKMenuVideoSora2ExamplesEnabled, domain.CommandMenuVideoSora2Examples)
	disableWhenFalse(cfg.VKMenuVideoKling21Enabled, domain.CommandMenuVideoKling21)
	disableWhenFalse(cfg.VKMenuVideoKling21StartEnabled, domain.CommandMenuVideoKling21Start)
	disableWhenFalse(cfg.VKMenuVideoKling21ExamplesEnabled, domain.CommandMenuVideoKling21Examples)
	disableWhenFalse(cfg.VKMenuVideoSeedance1Enabled, domain.CommandMenuVideoSeedance1)
	disableWhenFalse(cfg.VKMenuVideoSeedance1LiteEnabled, domain.CommandMenuVideoSeedance1Lite)
	disableWhenFalse(cfg.VKMenuVideoSeedance1ProEnabled, domain.CommandMenuVideoSeedance1Pro)
	disableWhenFalse(cfg.VKMenuVideoHaiuo02Enabled, domain.CommandMenuVideoHaiuo02)
	disableWhenFalse(cfg.VKMenuVideoHaiuo02StandardEnabled, domain.CommandMenuVideoHaiuo02Standard)
	disableWhenFalse(cfg.VKMenuVideoHaiuo02FastEnabled, domain.CommandMenuVideoHaiuo02Fast)
	disableWhenFalse(cfg.VKMenuImageTextEnabled, domain.CommandMenuImageText)
	disableWhenFalse(cfg.VKMenuImageReferenceEnabled, domain.CommandMenuImageReference)
	disableWhenFalse(cfg.VKMenuStudentsSolverEnabled, domain.CommandMenuStudentSolver)
	disableWhenFalse(cfg.VKMenuStudentsPresentationEnabled, domain.CommandMenuStudentPresentation)
	disableWhenFalse(cfg.VKMenuStudentsReportEnabled, domain.CommandMenuStudentReport)
	disableWhenFalse(cfg.VKMenuStudentsQAEnabled, domain.CommandMenuStudentQA)

	return vkinbound.MenuFeatureFlags{DisabledCommands: disabled}
}
