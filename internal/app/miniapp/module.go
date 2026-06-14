// Package miniapp wires the VK Mini App BFF surface onto the shared backend core.
package miniapp

import (
	"context"
	"log/slog"

	miniappapi "vk-ai-aggregator/internal/adapter/inbound/miniapp"
	s3store "vk-ai-aggregator/internal/adapter/storage/s3"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/platform/ratelimit"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/referralservice"
)

// Deps are shared backend-core collaborators required by the Mini App surface.
type Deps struct {
	Users         domain.UserRepository
	Jobs          domain.JobRepository
	Conversations domain.ConversationRepository
	Artifacts     domain.ArtifactRepository
	Moderation    domain.ModerationResultRepository
	Billing       *billingservice.Service
	BillingRepo   domain.BillingRepository
	Payment       *paymentservice.Service
	Referrals     domain.ReferralRepository
	Orchestrator  *joborchestrator.Orchestrator
	Logger        *slog.Logger
}

// NewHandler builds the Mini App BFF HTTP handler. The surface owns only
// launch-param protected HTTP wiring, rate limiting and artifact read access;
// job creation, pricing, billing and provider execution remain backend-core
// responsibilities.
func NewHandler(ctx context.Context, cfg config.Config, deps Deps) *miniappapi.Handler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	var objectStore miniappapi.ObjectReader
	store, err := s3store.New(ctx, s3store.Config{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		UseSSL:    cfg.S3UseSSL,
	})
	if err != nil {
		logger.Warn("s3 connect failed; miniapp artifact downloads disabled", "error", err)
	} else {
		objectStore = store
	}

	// Per-user rate limiting protects Mini App estimate and billable job
	// creation after launch params have been verified by the BFF.
	miniappJobLimiter := ratelimit.New(cfg.MiniAppJobRateLimitRPS, cfg.MiniAppJobRateLimitBurst)
	uploadLimiter := ratelimit.NewConcurrencyLimiter(cfg.MediaMaxConcurrentUploads)
	referrals := referralservice.New(deps.Referrals, deps.Billing, referralservice.Config{
		CodeLength:                  cfg.ReferralCodeLength,
		ReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		ReferredSignupRewardCredits: cfg.ReferralReferredSignupRewardCredits,
		RewardOnActivation:          cfg.ReferralRewardOnActivation,
	})
	return miniappapi.NewHandler(miniappapi.Config{
		AppSecret:                           cfg.VKAppSecret,
		LaunchParamsMaxAge:                  cfg.MiniAppLaunchParamsMaxAge,
		JobRateLimiter:                      miniappJobLimiter,
		UploadConcurrencyLimiter:            uploadLimiter,
		ReferenceUploadsDisabled:            !cfg.MediaReferenceUploadsEnabled,
		ReferenceWebPEnabled:                cfg.MediaReferenceWebPEnabled,
		MaxUploadBytes:                      cfg.MediaMaxImageUploadBytes,
		MaxUploadImageWidth:                 cfg.MediaMaxImageWidth,
		MaxUploadImageHeight:                cfg.MediaMaxImageHeight,
		MaxUploadImagePixels:                cfg.MediaMaxImagePixels,
		ImageReferenceEnabled:               cfg.DeepInfraImageReferenceEnabled,
		ReferralLinkBase:                    cfg.VKReferralLinkBase,
		ReferralReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		ReferralReferredSignupRewardCredits: cfg.ReferralReferredSignupRewardCredits,
		FrontendTelemetryEnabled:            cfg.FrontendTelemetryEnabled,
		FrontendTelemetryUserHashSecret:     cfg.FrontendTelemetryUserHashSecret,
		PaymentReturnURL:                    firstNonEmpty(cfg.YooKassaReturnURLMiniApp, cfg.YooKassaReturnURL),
	}, miniappapi.Deps{
		Users:         deps.Users,
		Jobs:          deps.Jobs,
		Conversations: deps.Conversations,
		Artifacts:     deps.Artifacts,
		Moderation:    deps.Moderation,
		Objects:       objectStore,
		Billing:       deps.Billing,
		BillingRepo:   deps.BillingRepo,
		Payment:       deps.Payment,
		Referrals:     referrals,
		Orchestrator:  deps.Orchestrator,
		Logger:        logger,
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
