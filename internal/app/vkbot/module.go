// Package vkbot wires the VK text bot app surface onto the shared backend core.
package vkbot

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"

	vkdelivery "vk-ai-aggregator/internal/adapter/delivery/vk"
	vkinbound "vk-ai-aggregator/internal/adapter/inbound/vk"
	redisqueue "vk-ai-aggregator/internal/adapter/queue/redis"
	s3store "vk-ai-aggregator/internal/adapter/storage/s3"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/antispam"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/dialogstate"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/productcatalog"
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
	Artifacts    domain.ArtifactRepository
	Orchestrator *joborchestrator.Orchestrator
	Router       *commandrouter.Router
	Logger       *slog.Logger
}

// NewHandler builds the VK callback HTTP handler without owning core business
// decisions. Provider calls, billing mutations and job processing remain in the
// shared orchestrator/worker/billing layers.
func NewHandler(ctx context.Context, cfg config.Config, deps Deps) http.Handler {
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

	var objectStore vkinbound.ObjectStore
	if !cfg.MediaReferenceUploadsEnabled {
		logger.Warn("vk video reference uploads disabled by MEDIA_REFERENCE_UPLOADS_ENABLED")
	} else {
		store, err := s3store.New(ctx, s3store.Config{
			Endpoint:  cfg.S3Endpoint,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
			UseSSL:    cfg.S3UseSSL,
		})
		if err != nil {
			logger.Warn("s3 connect failed; vk video reference uploads disabled", "error", err)
		} else {
			objectStore = store
		}
	}

	referrals := referralservice.New(deps.Referrals, deps.Billing, referralservice.Config{
		CodeLength:                  cfg.ReferralCodeLength,
		ReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		ReferredSignupRewardCredits: cfg.ReferralReferredSignupRewardCredits,
		RewardOnActivation:          cfg.ReferralRewardOnActivation,
	})
	runtimeCatalog, err := productcatalog.FromConfig(cfg)
	if err != nil {
		logger.Warn("vk bot video route catalog disabled", "error", err)
	}

	return vkinbound.NewHandler(vkinbound.Config{
		ConfirmationToken:                   cfg.VKConfirmationToken,
		Secret:                              cfg.VKSecret,
		WelcomeAttachment:                   cfg.VKWelcomeAttachment,
		MenuButtonMode:                      cfg.VKMenuButtonMode,
		UnroutedTextMode:                    cfg.VKUnroutedTextMode,
		MenuFeatures:                        menuFeatures(cfg, runtimeCatalog),
		ImageModels:                         runtimeCatalog.ImageModels(),
		VideoRoutes:                         runtimeCatalog.VideoRoutes(),
		ReferralLinkBase:                    cfg.VKReferralLinkBase,
		ReferralShareBase:                   cfg.VKReferralShareBase,
		ReferralReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		TopUpReceiptEmail:                   cfg.VKTopUpReceiptEmail,
		TopUpReceiptPhone:                   cfg.VKTopUpReceiptPhone,
		TopUpReturnURL:                      firstNonEmpty(cfg.YooKassaReturnURLVKBot, cfg.YooKassaReturnURL),
		TopUpStatusEditEnabled:              cfg.FeatureVKTopUpStatusEditEnabled,
		ReferenceUploadsDisabled:            !cfg.MediaReferenceUploadsEnabled,
		ArtifactBucket:                      cfg.S3Bucket,
		MaxUploadBytes:                      cfg.MediaMaxImageUploadBytes,
		MaxUploadImageWidth:                 cfg.MediaMaxImageWidth,
		MaxUploadImageHeight:                cfg.MediaMaxImageHeight,
		MaxUploadImagePixels:                cfg.MediaMaxImagePixels,
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
		Artifacts:    deps.Artifacts,
		Objects:      objectStore,
		Logger:       logger,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func menuFeatures(cfg config.Config, catalog productcatalog.RuntimeCatalog) vkinbound.MenuFeatureFlags {
	disabled := map[domain.CommandType]bool{}
	enabled := map[domain.CommandType]bool{}
	disableWhenFalse := func(enabled bool, commands ...domain.CommandType) {
		if enabled {
			return
		}
		for _, command := range commands {
			disabled[command] = true
		}
	}
	enableWhenTrue := func(ok bool, commands ...domain.CommandType) {
		if !ok {
			return
		}
		for _, command := range commands {
			enabled[command] = true
		}
	}

	imageModels := catalog.ImageModels()
	videoRoutes := catalog.VideoRoutes()
	imageAvailable := func(modelID string) bool {
		for _, model := range imageModels {
			if model.Enabled && model.ID == modelID {
				return true
			}
		}
		return false
	}
	imageReferenceAvailable := func() bool {
		for _, model := range imageModels {
			if model.Enabled && model.SupportsReferenceImage {
				return true
			}
		}
		return false
	}
	videoRouteAvailable := func(alias domain.VideoRouteAlias) bool {
		for _, route := range videoRoutes {
			if route.Enabled && route.Alias == string(alias) {
				return true
			}
		}
		return false
	}

	disableWhenFalse(cfg.VKMenuVideoEnabled && len(videoRoutes) > 0, domain.CommandMenuVideo)
	disableWhenFalse(false, domain.CommandMenuVideoPrunaAI)

	disableWhenFalse(cfg.VKMenuImageEnabled && len(imageModels) > 0, domain.CommandMenuImage)
	disableWhenFalse(imageAvailable(modelcatalog.MiniAppImageNanoBananaPro), domain.CommandMenuImageText)
	disableWhenFalse(cfg.VKMenuGPTEnabled, domain.CommandMenuText)
	disableWhenFalse(cfg.VKMenuStudentsEnabled, domain.CommandMenuStudents)
	disableWhenFalse(cfg.VKMenuAccountEnabled, domain.CommandAccount)
	disableWhenFalse(cfg.VKMenuTopUpEnabled, domain.CommandTopUp)
	runwayGen4TurboReady := videoRouteAvailable(domain.VideoRouteRunwayGen4Turbo)
	disableWhenFalse(cfg.VKMenuVideoSora2Enabled && runwayGen4TurboReady, domain.CommandMenuVideoSora2)
	disableWhenFalse(cfg.VKMenuVideoSora2StartEnabled && runwayGen4TurboReady, domain.CommandMenuVideoSora2Start)
	disableWhenFalse(cfg.VKMenuVideoSora2ExamplesEnabled && runwayGen4TurboReady, domain.CommandMenuVideoSora2Examples)
	enableWhenTrue(runwayGen4TurboReady, domain.CommandMenuVideoSora2, domain.CommandMenuVideoSora2Start, domain.CommandMenuVideoSora2Examples)
	klingO3Ready := videoRouteAvailable(domain.VideoRouteKlingO3Standard)
	disableWhenFalse(cfg.VKMenuVideoKling21Enabled && klingO3Ready, domain.CommandMenuVideoKling21)
	disableWhenFalse(cfg.VKMenuVideoKling21StartEnabled && klingO3Ready, domain.CommandMenuVideoKling21Start)
	disableWhenFalse(cfg.VKMenuVideoKling21ExamplesEnabled && klingO3Ready, domain.CommandMenuVideoKling21Examples)
	enableWhenTrue(klingO3Ready, domain.CommandMenuVideoKling21, domain.CommandMenuVideoKling21Start, domain.CommandMenuVideoKling21Examples)
	seedanceReady := videoRouteAvailable(domain.VideoRouteSeedance20Fast)
	disableWhenFalse(cfg.VKMenuVideoSeedance1Enabled && seedanceReady, domain.CommandMenuVideoSeedance1)
	disableWhenFalse(cfg.VKMenuVideoSeedance1LiteEnabled && seedanceReady, domain.CommandMenuVideoSeedance1Lite)
	disableWhenFalse(cfg.VKMenuVideoSeedance1ProEnabled, domain.CommandMenuVideoSeedance1Pro)
	disableWhenFalse(false, domain.CommandMenuVideoSeedance1Pro)
	enableWhenTrue(seedanceReady, domain.CommandMenuVideoSeedance1, domain.CommandMenuVideoSeedance1Lite)
	hailuoStandardReady := videoRouteAvailable(domain.VideoRouteHailuo23Standard)
	hailuoFastReady := videoRouteAvailable(domain.VideoRouteHailuo23Fast)
	disableWhenFalse(cfg.VKMenuVideoHailuo02Enabled && (hailuoStandardReady || hailuoFastReady), domain.CommandMenuVideoHailuo02)
	disableWhenFalse(cfg.VKMenuVideoHailuo02StandardEnabled && hailuoStandardReady, domain.CommandMenuVideoHailuo02Standard)
	disableWhenFalse(cfg.VKMenuVideoHailuo02FastEnabled && hailuoFastReady, domain.CommandMenuVideoHailuo02Fast)
	enableWhenTrue(hailuoStandardReady || hailuoFastReady, domain.CommandMenuVideoHailuo02)
	enableWhenTrue(hailuoStandardReady, domain.CommandMenuVideoHailuo02Standard)
	enableWhenTrue(hailuoFastReady, domain.CommandMenuVideoHailuo02Fast)
	disableWhenFalse(imageAvailable(modelcatalog.MiniAppImageNanoBanana2), domain.CommandMenuImageNanoBanana2)
	disableWhenFalse(imageAvailable(modelcatalog.MiniAppImageSeedream45), domain.CommandMenuImageDeepInfraSeedream)
	disableWhenFalse(imageAvailable(modelcatalog.MiniAppImageSDXLTurbo), domain.CommandMenuImageDeepInfraSDXL)
	disableWhenFalse(imageAvailable(modelcatalog.MiniAppImageGPTImage2), domain.CommandMenuImageGPTImage2)
	disableWhenFalse(cfg.VKMenuImageReferenceEnabled && imageReferenceAvailable(), domain.CommandMenuImageReference)
	disableWhenFalse(cfg.VKMenuStudentsSolverEnabled, domain.CommandMenuStudentSolver)
	disableWhenFalse(cfg.VKMenuStudentsPresentationEnabled, domain.CommandMenuStudentPresentation)
	disableWhenFalse(cfg.VKMenuStudentsReportEnabled, domain.CommandMenuStudentReport)
	disableWhenFalse(cfg.VKMenuStudentsQAEnabled, domain.CommandMenuStudentQA)

	return vkinbound.MenuFeatureFlags{DisabledCommands: disabled, EnabledCommands: enabled}
}
