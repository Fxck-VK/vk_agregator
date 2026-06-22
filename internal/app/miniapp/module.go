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
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/referralservice"
	"vk-ai-aggregator/internal/service/videorouter"
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
		Endpoint:        cfg.S3Endpoint,
		AccessKey:       cfg.S3AccessKey,
		SecretKey:       cfg.S3SecretKey,
		UseSSL:          cfg.S3UseSSL,
		Region:          cfg.S3Region,
		AddressingStyle: cfg.S3AddressingStyle,
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
	runtimeCatalog, err := productcatalog.FromConfig(cfg)
	if err != nil {
		logger.Warn("miniapp video route catalog disabled", "error", err)
	}
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
		ImageReferenceEnabled:               runtimeCatalog.ImageReferenceEnabled,
		ReferralLinkBase:                    cfg.VKReferralLinkBase,
		ReferralReferrerSignupRewardCredits: cfg.ReferralReferrerSignupRewardCredits,
		ReferralReferredSignupRewardCredits: cfg.ReferralReferredSignupRewardCredits,
		FrontendTelemetryEnabled:            cfg.FrontendTelemetryEnabled,
		FrontendTelemetryUserHashSecret:     cfg.FrontendTelemetryUserHashSecret,
		PaymentReturnURL:                    firstNonEmpty(cfg.YooKassaReturnURLMiniApp, cfg.YooKassaReturnURL),
		PaymentCancelEnabled:                cfg.FeatureMiniAppPaymentCancelEnabled,
		ImageModels:                         miniAppImageModels(runtimeCatalog.Catalog),
		VideoRoutes:                         miniAppVideoRoutes(runtimeCatalog.Catalog),
		VideoRouteResolver:                  miniAppVideoRouteResolver(runtimeCatalog.VideoRouteCatalog),
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

func miniAppImageModels(catalog *productcatalog.Catalog) []miniappapi.ImageModelDTO {
	if catalog == nil {
		return nil
	}
	models := catalog.ImageModels()
	out := make([]miniappapi.ImageModelDTO, 0, len(models))
	for _, model := range models {
		out = append(out, miniappapi.ImageModelDTO{
			Type:                   model.Type,
			ID:                     model.ID,
			Name:                   model.Name,
			Description:            model.Description,
			EstimateCredits:        model.EstimateCredits,
			Enabled:                model.Enabled,
			QualityOptions:         append([]string(nil), model.QualityOptions...),
			DefaultQuality:         model.DefaultQuality,
			SupportsReferenceImage: model.SupportsReferenceImage,
			MaxReferenceImages:     model.MaxReferenceImages,
		})
	}
	return out
}

func miniAppVideoRoutes(catalog *productcatalog.Catalog) []miniappapi.VideoRouteDTO {
	if catalog == nil {
		return nil
	}
	publicRoutes := catalog.VideoRoutes()
	routes := make([]miniappapi.VideoRouteDTO, 0, len(publicRoutes))
	for _, route := range publicRoutes {
		routes = append(routes, miniappapi.VideoRouteDTO{
			Type:                   route.Type,
			Alias:                  route.Alias,
			Name:                   route.Name,
			Description:            route.Description,
			EstimateCredits:        route.EstimateCredits,
			Enabled:                route.Enabled,
			AllowedDurationsSec:    append([]int(nil), route.AllowedDurationsSec...),
			AllowedResolutions:     append([]string(nil), route.AllowedResolutions...),
			AllowedAspectRatios:    append([]string(nil), route.AllowedAspectRatios...),
			DefaultDurationSec:     route.DefaultDurationSec,
			DefaultResolution:      route.DefaultResolution,
			DefaultAspectRatio:     route.DefaultAspectRatio,
			RequiresStartImage:     route.RequiresStartImage,
			SupportsReferenceImage: route.SupportsReferenceImage,
			MaxReferenceImages:     route.MaxReferenceImages,
		})
	}
	return routes
}

func miniAppVideoRouteResolver(catalog *videorouter.Catalog) joborchestrator.VideoRouteResolver {
	if catalog == nil {
		return nil
	}
	return joborchestrator.VideoRouteResolverFunc(func(ctx context.Context, in joborchestrator.VideoRouteCheckInput) (joborchestrator.VideoRouteResolution, error) {
		resolution, err := catalog.Resolve(ctx, videorouter.Request{
			Source:           in.Source,
			Operation:        in.Operation,
			Modality:         in.Modality,
			Params:           in.Params,
			InputArtifactIDs: in.InputArtifactIDs,
		})
		if err != nil {
			return joborchestrator.VideoRouteResolution{}, err
		}
		return joborchestrator.VideoRouteResolution{
			Resolved:            resolution.Resolved,
			Params:              resolution.Params,
			Snapshot:            resolution.Snapshot,
			InternalCostCredits: resolution.InternalCostCredits,
		}, nil
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
