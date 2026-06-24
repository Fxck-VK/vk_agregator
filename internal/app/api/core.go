// Package api contains bootstrap-only helpers for the cmd/api binary.
package api

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentadapter "vk-ai-aggregator/internal/adapter/payment"
	"vk-ai-aggregator/internal/adapter/storage/postgres"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/platform/config"
	"vk-ai-aggregator/internal/service/billingservice"
	"vk-ai-aggregator/internal/service/commandrouter"
	"vk-ai-aggregator/internal/service/joborchestrator"
	"vk-ai-aggregator/internal/service/paymentservice"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// SharedCore groups backend-core collaborators shared by app surfaces.
type SharedCore struct {
	Users          domain.UserRepository
	Jobs           domain.JobRepository
	Commands       domain.CommandRepository
	Inbound        domain.InboundEventRepository
	Idempotency    domain.IdempotencyRepository
	Deliveries     domain.DeliveryRepository
	Audits         domain.OperatorAuditRepository
	BillingRepo    domain.BillingRepository
	Payments       domain.PaymentRepository
	Referrals      domain.ReferralRepository
	Artifacts      domain.ArtifactRepository
	Moderation     domain.ModerationResultRepository
	Conversations  domain.ConversationRepository
	Maintenance    *postgres.MaintenanceRepository
	Billing        *billingservice.Service
	Payment        *paymentservice.Service
	PaymentOps     *paymentservice.WebhookProcessor
	Orchestrator   *joborchestrator.Orchestrator
	Router         *commandrouter.Router
	PricingCatalog *pricingcatalog.Catalog
}

type sharedCoreOptions struct {
	orchestratorOptions []joborchestrator.Option
	pricingCatalog      *pricingcatalog.Catalog
}

// SharedCoreOption customizes backend-core wiring.
type SharedCoreOption func(*sharedCoreOptions)

// WithOrchestratorOptions forwards safety-policy options into job creation.
func WithOrchestratorOptions(opts ...joborchestrator.Option) SharedCoreOption {
	return func(o *sharedCoreOptions) {
		o.orchestratorOptions = append(o.orchestratorOptions, opts...)
	}
}

// WithPricingCatalog wires the single runtime generation pricing catalog into
// core services that need backend-owned pricing decisions.
func WithPricingCatalog(catalog *pricingcatalog.Catalog) SharedCoreOption {
	return func(o *sharedCoreOptions) {
		o.pricingCatalog = catalog
	}
}

// NewSharedCore wires repositories and services without owning surface behavior.
func NewSharedCore(pool *pgxpool.Pool, cfg config.Config, opts ...SharedCoreOption) (SharedCore, error) {
	var options sharedCoreOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.pricingCatalog == nil {
		return SharedCore{}, errors.New("api core: pricing catalog is required")
	}
	users := postgres.NewUserRepository(pool)
	jobs := postgres.NewJobRepository(pool)
	billingRepo := postgres.NewBillingRepository(pool)
	payments := postgres.NewPaymentRepository(pool)
	billing := billingservice.New(billingRepo, billingservice.WithPriceOverrides(cfg.PriceOverrides))
	paymentProvider, err := paymentadapter.NewProvider(cfg)
	if err != nil {
		return SharedCore{}, err
	}
	paymentSvc := paymentservice.New(payments, paymentProvider, paymentservice.Config{
		ReturnURL:                    cfg.YooKassaReturnURL,
		IncludeDevTestPaymentProduct: cfg.FeatureDevPaymentTestProductEnabled,
	})
	txRunner := paymentservice.TxRunnerFunc(func(ctx context.Context, fn func(context.Context, domain.PaymentRepository, domain.BillingRepository) error) error {
		return postgres.RunInTx(ctx, pool, func(ctx context.Context, tx pgx.Tx) error {
			return fn(ctx, postgres.NewPaymentRepository(tx), postgres.NewBillingRepositoryTx(tx))
		})
	})
	paymentOps := paymentservice.NewWebhookProcessor(payments, paymentProvider, billing, txRunner)

	// The orchestrator records a queued outbox event; the worker's outbox relay
	// publishes it to the queue, so the api process does not enqueue directly
	// (audit A2).
	orchestratorOptions := append([]joborchestrator.Option{
		joborchestrator.WithPricingCatalog(options.pricingCatalog),
	}, options.orchestratorOptions...)
	orch := joborchestrator.New(jobs, postgres.NewUnitOfWork(pool), billing, cfg.MaxJobCost, orchestratorOptions...)

	return SharedCore{
		Users:          users,
		Jobs:           jobs,
		Commands:       postgres.NewCommandRepository(pool),
		Inbound:        postgres.NewInboundEventRepository(pool),
		Idempotency:    postgres.NewIdempotencyRepository(pool),
		Deliveries:     postgres.NewDeliveryRepository(pool),
		Audits:         postgres.NewOperatorAuditRepository(pool),
		BillingRepo:    billingRepo,
		Payments:       payments,
		Referrals:      postgres.NewReferralRepository(pool),
		Artifacts:      postgres.NewArtifactRepository(pool),
		Moderation:     postgres.NewModerationResultRepository(pool),
		Conversations:  postgres.NewConversationRepository(pool),
		Maintenance:    postgres.NewMaintenanceRepository(pool),
		Billing:        billing,
		Payment:        paymentSvc,
		PaymentOps:     paymentOps,
		Orchestrator:   orch,
		Router:         commandrouter.New(),
		PricingCatalog: options.pricingCatalog,
	}, nil
}
