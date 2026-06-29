// Package assistantfacts renders backend-owned public NeuroHub facts for the
// text assistant. It intentionally exposes only product-level catalog data.
package assistantfacts

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
)

const header = "NeuroHub facts"

// BalanceProvider reads the backend-owned user balance for assistant facts.
type BalanceProvider interface {
	BalanceForEstimate(ctx context.Context, userID uuid.UUID) (int64, error)
}

// CatalogSource returns the current public product catalog. It lets callers
// rebuild product visibility from a refreshed runtime pricing catalog.
type CatalogSource func(ctx context.Context) (*productcatalog.Catalog, error)

// Config wires the public catalog sources used by Builder.
type Config struct {
	Catalog       *productcatalog.Catalog
	CatalogSource CatalogSource
	// PricingCatalog is used only for public display estimates.
	PricingCatalog *pricingcatalog.Catalog
	Balance        BalanceProvider
}

// Input is one text-assistant request.
type Input struct {
	UserID uuid.UUID
	Prompt string
}

// Facts is the rendered public context for the text model.
type Facts struct {
	Relevant bool
	Text     string
}

// Builder creates public facts for product-aware assistant answers.
type Builder struct {
	catalog       *productcatalog.Catalog
	catalogSource CatalogSource
	pricing       *pricingcatalog.Catalog
	balance       BalanceProvider
}

// New returns a facts builder. PricingCatalog is accepted in Config to make
// the source contract explicit; productcatalog items already contain public
// backend-computed display prices.
func New(cfg Config) *Builder {
	return &Builder{
		catalog:       cfg.Catalog,
		catalogSource: cfg.CatalogSource,
		pricing:       cfg.PricingCatalog,
		balance:       cfg.Balance,
	}
}

// Build returns a compact public facts block only for NeuroHub/product intents.
func (b *Builder) Build(ctx context.Context, in Input) (Facts, error) {
	intent := classify(in.Prompt)
	if !intent.relevant {
		return Facts{}, nil
	}

	var lines []string
	lines = append(lines,
		header+":",
		"- Ты НейроХаб. На вопросы о моделях, генерации, ценах, качестве, длительностях, референсах и балансе отвечай только по этим фактам.",
		"- Не перечисляй мировые AI-модели, если их нет в каталоге НейроХаб.",
		"- Не раскрывай внутренние маршруты, API, системные настройки и технические идентификаторы.",
		"- Текстовый ассистент: НейроХаб, бесплатный чат внутри продукта.",
	)

	imageLines, videoLines, err := b.catalogLines(ctx)
	if err != nil {
		return Facts{}, err
	}
	if len(imageLines) == 0 && len(videoLines) == 0 {
		lines = append(lines, "- Каталог генерации НейроХаб сейчас пуст или недоступен.")
	} else {
		if len(imageLines) > 0 {
			lines = append(lines, "- Доступные модели изображений в НейроХаб:")
			lines = append(lines, imageLines...)
		}
		if len(videoLines) > 0 {
			lines = append(lines, "- Доступные видео-модели в НейроХаб:")
			lines = append(lines, videoLines...)
		}
	}

	if intent.balance {
		lines = append(lines, b.balanceLine(ctx, in.UserID))
	}

	return Facts{Relevant: true, Text: strings.Join(lines, "\n")}, nil
}

// Attach prepends facts to an already-rendered prompt without changing the
// user text stored by dialog history.
func Attach(facts, prompt string) string {
	facts = strings.TrimSpace(facts)
	prompt = strings.TrimSpace(prompt)
	switch {
	case facts == "":
		return prompt
	case prompt == "":
		return facts
	default:
		return facts + "\n\nAssistant instruction: use the facts above when they answer the user's product question. If a requested product fact is absent, say that it is currently unavailable in NeuroHub.\n\n" + prompt
	}
}

type intent struct {
	relevant bool
	balance  bool
}

func classify(prompt string) intent {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return intent{}
	}
	balance := containsAny(p, []string{
		"баланс",
		"остаток",
		"balance",
		"сколько у меня кредит",
		"сколько осталось кредит",
		"мои кредиты",
	})
	product := balance || containsAny(p, []string{
		"нейрохаб",
		"нейро хаб",
		"neurohub",
		"neirohub",
		"модель",
		"модели",
		"model",
		"models",
		"генерац",
		"умеешь",
		"умеет",
		"можешь",
		"доступн",
		"available",
		"цена",
		"стоимость",
		"сколько стоит",
		"price",
		"кредит",
		"качество",
		"quality",
		"длитель",
		"секунд",
		"duration",
		"референс",
		"reference",
		"фото",
		"изображ",
		"картин",
		"видео",
		"что ты за",
		"кто ты",
	})
	return intent{relevant: product, balance: balance}
}

func containsAny(s string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}

func (b *Builder) currentCatalog(ctx context.Context) (*productcatalog.Catalog, error) {
	if b == nil {
		return nil, nil
	}
	if b.catalogSource != nil {
		return b.catalogSource(ctx)
	}
	return b.catalog, nil
}

func (b *Builder) catalogLines(ctx context.Context) ([]string, []string, error) {
	catalog, err := b.currentCatalog(ctx)
	if err != nil {
		return nil, nil, err
	}
	if catalog == nil {
		return nil, nil, nil
	}

	images := catalog.ImageModels()
	imageLines := make([]string, 0, len(images))
	for _, model := range images {
		if !model.Enabled {
			continue
		}
		credits := model.EstimateCredits
		if b.pricing != nil && model.DefaultQuality != "" {
			if display, err := b.pricing.DisplayEstimateCredits(pricingcatalog.ProductKey{
				Operation:    domain.OperationImageGenerate,
				Modality:     domain.ModalityImage,
				ImageModelID: model.ID,
				Quality:      model.DefaultQuality,
			}); err == nil && display > 0 {
				credits = display
			}
		}
		parts := []string{
			model.Name,
			formatPrice(credits),
		}
		if len(model.QualityOptions) > 0 {
			parts = append(parts, "качества: "+strings.Join(model.QualityOptions, ", "))
		}
		parts = append(parts, "референсы: "+formatReferenceSupport(model.SupportsReferenceImage, model.MaxReferenceImages))
		imageLines = append(imageLines, "  - "+strings.Join(parts, "; "))
	}

	videos := catalog.VideoRoutes()
	videoLines := make([]string, 0, len(videos))
	for _, route := range videos {
		if !route.Enabled {
			continue
		}
		credits := route.EstimateCredits
		if b.pricing != nil && route.DefaultResolution != "" && route.DefaultDurationSec > 0 {
			if display, err := b.pricing.DisplayEstimateCredits(pricingcatalog.ProductKey{
				Operation:       domain.OperationVideoGenerate,
				Modality:        domain.ModalityVideo,
				VideoRouteAlias: domain.VideoRouteAlias(route.Alias),
				Resolution:      route.DefaultResolution,
				DurationSec:     route.DefaultDurationSec,
			}); err == nil && display > 0 {
				credits = display
			}
		}
		parts := []string{
			route.Name,
			formatPrice(credits),
		}
		if len(route.AllowedDurationsSec) > 0 {
			parts = append(parts, "длительности: "+formatInts(route.AllowedDurationsSec)+" сек")
		}
		if len(route.AllowedResolutions) > 0 {
			parts = append(parts, "разрешения: "+strings.Join(route.AllowedResolutions, ", "))
		}
		if len(route.AllowedAspectRatios) > 0 {
			parts = append(parts, "форматы: "+strings.Join(route.AllowedAspectRatios, ", "))
		}
		if route.RequiresStartImage {
			parts = append(parts, "стартовое изображение: обязательно")
		}
		parts = append(parts, "референсы: "+formatReferenceSupport(route.SupportsReferenceImage, route.MaxReferenceImages))
		videoLines = append(videoLines, "  - "+strings.Join(parts, "; "))
	}
	return imageLines, videoLines, nil
}

func (b *Builder) balanceLine(ctx context.Context, userID uuid.UUID) string {
	if b == nil || b.balance == nil || userID == uuid.Nil {
		return "- Баланс пользователя: временно недоступен."
	}
	balance, err := b.balance.BalanceForEstimate(ctx, userID)
	if err != nil {
		return "- Баланс пользователя: временно недоступен."
	}
	return "- Баланс пользователя: " + formatCredits(balance) + "."
}

func formatReferenceSupport(supported bool, max int) string {
	if !supported {
		return "нет"
	}
	if max > 0 {
		return fmt.Sprintf("да, до %d", max)
	}
	return "да"
}

func formatInts(values []int) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprintf("%d", value))
	}
	return strings.Join(out, ", ")
}

func formatCredits(credits int64) string {
	return fmt.Sprintf("%d кредитов", credits)
}

func formatPrice(credits int64) string {
	if credits <= 0 {
		return "цена сейчас недоступна"
	}
	return "цена от " + formatCredits(credits)
}
