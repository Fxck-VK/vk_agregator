package assistantfacts

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/videorouter"
)

type balanceProviderFunc func(context.Context, uuid.UUID) (int64, error)

func (f balanceProviderFunc) BalanceForEstimate(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f(ctx, userID)
}

func TestBuildIncludesEnabledNeuroHubCatalogFacts(t *testing.T) {
	catalog := productcatalog.New(productcatalog.Config{
		ImageProviderReady: map[domain.ProviderName]bool{
			domain.ProviderAPIMart: true,
			domain.ProviderPoYo:    true,
		},
		EnabledImageModels: map[string]bool{
			pricingcatalog.PublicImageNanoBanana2:   true,
			pricingcatalog.PublicImageNanoBananaPro: true,
			pricingcatalog.PublicImageGPTImage2:     false,
		},
		VideoRoutes: []videorouter.PublicRoute{
			{
				Alias:                  domain.VideoRouteKlingO3Standard,
				DefaultResolution:      pricingcatalog.VideoResolution720p,
				AllowedResolutions:     []string{pricingcatalog.VideoResolution720p},
				AllowedDurationsSec:    []int{5, 10},
				AllowedAspectRatios:    []string{"16:9"},
				DefaultDurationSec:     5,
				DefaultAspectRatio:     "16:9",
				SupportsReferenceImage: true,
				MaxReferenceImages:     1,
			},
		},
		PricingCatalog: staticPricingCatalog(t),
	})
	builder := New(Config{Catalog: catalog, PricingCatalog: staticPricingCatalog(t)})

	facts, err := builder.Build(context.Background(), Input{
		UserID: uuid.New(),
		Prompt: "Какие модели доступны в НейроХаб и можно ли с референсом?",
	})
	if err != nil {
		t.Fatalf("build facts: %v", err)
	}
	if !facts.Relevant {
		t.Fatal("expected facts to be relevant")
	}
	for _, want := range []string{
		"Факты НейроХаб",
		"НейроХаб",
		"Nano Banana 2",
		"Nano Banana Pro",
		"Kling O3 Standard",
		"качества: 1K, 2K, 4K",
		"референсы: да",
		"длительности: 5, 10 сек",
		"цена от",
	} {
		if !strings.Contains(facts.Text, want) {
			t.Fatalf("facts missing %q:\n%s", want, facts.Text)
		}
	}
	for _, forbidden := range []string{
		"GPT Image 2",
		"NeuroHub",
		"продукт",
		"deepseek",
		"deepinfra",
		"provider",
		"floor",
		"multiplier",
		"apimart",
		"poyo",
	} {
		if strings.Contains(strings.ToLower(facts.Text), strings.ToLower(forbidden)) {
			t.Fatalf("facts leaked %q:\n%s", forbidden, facts.Text)
		}
	}
}

func TestBuildIncludesBalanceOnlyForBalanceIntent(t *testing.T) {
	calls := 0
	builder := New(Config{
		Catalog:        productcatalog.New(productcatalog.Config{PricingCatalog: staticPricingCatalog(t)}),
		PricingCatalog: staticPricingCatalog(t),
		Balance: balanceProviderFunc(func(context.Context, uuid.UUID) (int64, error) {
			calls++
			return 777, nil
		}),
	})

	noBalance, err := builder.Build(context.Background(), Input{UserID: uuid.New(), Prompt: "Какие модели есть?"})
	if err != nil {
		t.Fatalf("build catalog facts: %v", err)
	}
	if strings.Contains(noBalance.Text, "Баланс пользователя") || calls != 0 {
		t.Fatalf("balance should not be included for catalog-only prompt; calls=%d facts=%s", calls, noBalance.Text)
	}

	withBalance, err := builder.Build(context.Background(), Input{UserID: uuid.New(), Prompt: "Какой у меня баланс?"})
	if err != nil {
		t.Fatalf("build balance facts: %v", err)
	}
	if !strings.Contains(withBalance.Text, "Баланс пользователя: 777 кредитов") || calls != 1 {
		t.Fatalf("balance facts missing or unexpected calls=%d:\n%s", calls, withBalance.Text)
	}
}

func TestBuildUsesCatalogSourceForEachRelevantPrompt(t *testing.T) {
	prices := staticPricingCatalog(t)
	calls := 0
	builder := New(Config{
		PricingCatalog: prices,
		CatalogSource: func(context.Context) (*productcatalog.Catalog, error) {
			calls++
			enabled := map[string]bool{
				pricingcatalog.PublicImageNanoBanana2: true,
			}
			if calls > 1 {
				enabled[pricingcatalog.PublicImageNanoBananaPro] = true
			}
			return productcatalog.New(productcatalog.Config{
				ImageProviderReady: map[domain.ProviderName]bool{
					domain.ProviderAPIMart: true,
					domain.ProviderPoYo:    true,
				},
				EnabledImageModels: enabled,
				PricingCatalog:     prices,
			}), nil
		},
	})

	first, err := builder.Build(context.Background(), Input{UserID: uuid.New(), Prompt: "Какие модели есть?"})
	if err != nil {
		t.Fatalf("first build facts: %v", err)
	}
	if !strings.Contains(first.Text, "Nano Banana 2") || strings.Contains(first.Text, "Nano Banana Pro") {
		t.Fatalf("first facts should use first catalog snapshot:\n%s", first.Text)
	}

	second, err := builder.Build(context.Background(), Input{UserID: uuid.New(), Prompt: "Какие модели есть?"})
	if err != nil {
		t.Fatalf("second build facts: %v", err)
	}
	if !strings.Contains(second.Text, "Nano Banana Pro") || calls != 2 {
		t.Fatalf("second facts should refresh catalog source; calls=%d:\n%s", calls, second.Text)
	}
}

func TestBuildReturnsEmptyForUnrelatedPrompt(t *testing.T) {
	builder := New(Config{Catalog: productcatalog.New(productcatalog.Config{PricingCatalog: staticPricingCatalog(t)})})
	facts, err := builder.Build(context.Background(), Input{UserID: uuid.New(), Prompt: "Напиши поздравление с днем рождения"})
	if err != nil {
		t.Fatalf("build facts: %v", err)
	}
	if facts.Relevant || facts.Text != "" {
		t.Fatalf("unexpected facts for unrelated prompt: %+v", facts)
	}
}

func TestAttachDoesNotStoreFactsAsUserText(t *testing.T) {
	rendered := Attach("Факты НейроХаб:\n- список моделей", "Current user request:\nКакие модели есть?")
	if !strings.Contains(rendered, "Факты НейроХаб") || !strings.Contains(rendered, "Current user request") {
		t.Fatalf("attached prompt missing sections:\n%s", rendered)
	}
	if strings.Contains(rendered, "NeuroHub") || strings.Contains(strings.ToLower(rendered), "product") {
		t.Fatalf("attached prompt should not contain English brand/product wording:\n%s", rendered)
	}
	if strings.Index(rendered, "Факты НейроХаб") > strings.Index(rendered, "Current user request") {
		t.Fatalf("facts must precede user request:\n%s", rendered)
	}
}

func staticPricingCatalog(t *testing.T) *pricingcatalog.Catalog {
	t.Helper()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("static pricing catalog: %v", err)
	}
	return catalog
}
