package imagegeneration_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func TestResolver_UsesTrustedModelAndExactPricingSnapshot(t *testing.T) {
	pricing := staticPricingCatalog(t)
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{
		ID:                     modelcatalog.MiniAppImageNanoBanana2,
		Name:                   "Nano Banana 2",
		Enabled:                true,
		Ready:                  true,
		QualityOptions:         []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K, modelcatalog.ImageQuality4K},
		DefaultQuality:         modelcatalog.ImageQuality1K,
		SupportsReferenceImage: true,
		MaxReferenceImages:     4,
	}}, pricing)

	got, err := resolver.Resolve(imagegeneration.Request{
		ModelID:        modelcatalog.MiniAppImageNanoBanana2,
		Quality:        " 2k ",
		ReferenceCount: 2,
	})
	if err != nil {
		t.Fatalf("resolve image: %v", err)
	}

	if got.Public.ModelID != modelcatalog.MiniAppImageNanoBanana2 ||
		got.Public.ModelName != "Nano Banana 2" ||
		got.Public.ImageQuality != modelcatalog.ImageQuality2K {
		t.Fatalf("public selection = %+v", got.Public)
	}
	if got.Worker.Provider == "" || got.Worker.ModelCode == "" ||
		got.Worker.ModelID != modelcatalog.MiniAppImageNanoBanana2 ||
		got.Worker.ModelName != "Nano Banana 2" ||
		got.Worker.ImageQuality != modelcatalog.ImageQuality2K ||
		got.Worker.Resolution != modelcatalog.ImageQuality2K ||
		got.Worker.Size != "1:1" {
		t.Fatalf("trusted worker params = %+v", got.Worker)
	}

	wantSnapshot, err := pricing.Snapshot(pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: modelcatalog.MiniAppImageNanoBanana2,
		Quality:      modelcatalog.ImageQuality2K,
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !reflect.DeepEqual(got.PricingSnapshot, wantSnapshot) {
		t.Fatalf("pricing snapshot = %+v, want %+v", got.PricingSnapshot, wantSnapshot)
	}
	if got.PricingSnapshot.InternalCredits != 60 {
		t.Fatalf("exact server price = %d, want 60", got.PricingSnapshot.InternalCredits)
	}
}

func TestResolver_ResolvePublicSelectionDoesNotRequireCurrentPrice(t *testing.T) {
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{
		ID:                     modelcatalog.MiniAppImageNanoBanana2,
		Name:                   "Nano Banana 2",
		Enabled:                true,
		Ready:                  true,
		QualityOptions:         []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K},
		DefaultQuality:         modelcatalog.ImageQuality1K,
		SupportsReferenceImage: true,
		MaxReferenceImages:     2,
	}}, nil)

	selection, err := resolver.ResolvePublic(imagegeneration.Request{
		ModelID: modelcatalog.MiniAppImageNanoBanana2,
		Quality: " 2k ",
	})

	if err != nil {
		t.Fatalf("resolve public selection: %v", err)
	}
	if selection.ModelID != modelcatalog.MiniAppImageNanoBanana2 || selection.ModelName != "Nano Banana 2" || selection.ImageQuality != modelcatalog.ImageQuality2K {
		t.Fatalf("public selection = %+v", selection)
	}
}

func TestResolver_NormalizesAndValidatesAspectRatio(t *testing.T) {
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{
		ID:             modelcatalog.MiniAppImageNanoBanana2,
		Name:           "Nano Banana 2",
		Enabled:        true,
		Ready:          true,
		QualityOptions: []string{modelcatalog.ImageQuality1K},
		DefaultQuality: modelcatalog.ImageQuality1K,
	}}, staticPricingCatalog(t))

	defaulted, err := resolver.Resolve(imagegeneration.Request{
		ModelID: modelcatalog.MiniAppImageNanoBanana2,
	})
	if err != nil {
		t.Fatalf("resolve default aspect ratio: %v", err)
	}
	if defaulted.Public.AspectRatio != imagegeneration.DefaultAspectRatio || defaulted.Worker.AspectRatio != imagegeneration.DefaultAspectRatio {
		t.Fatalf("default aspect ratio = %q / %q, want %q", defaulted.Public.AspectRatio, defaulted.Worker.AspectRatio, imagegeneration.DefaultAspectRatio)
	}

	selected, err := resolver.Resolve(imagegeneration.Request{
		ModelID:     modelcatalog.MiniAppImageNanoBanana2,
		AspectRatio: " 4:5 ",
	})
	if err != nil {
		t.Fatalf("resolve selected aspect ratio: %v", err)
	}
	if selected.Public.AspectRatio != "4:5" || selected.Worker.AspectRatio != "4:5" {
		t.Fatalf("selected aspect ratio = %q / %q, want 4:5", selected.Public.AspectRatio, selected.Worker.AspectRatio)
	}

	_, err = resolver.Resolve(imagegeneration.Request{
		ModelID:     modelcatalog.MiniAppImageNanoBanana2,
		AspectRatio: "7:5",
	})
	if !errors.Is(err, imagegeneration.ErrUnsupportedAspectRatio) {
		t.Fatalf("unsupported aspect ratio error = %v, want ErrUnsupportedAspectRatio", err)
	}
}

func TestResolver_FailsClosedForDisabledOrUnreadyPublicModel(t *testing.T) {
	pricing := staticPricingCatalog(t)
	tests := []struct {
		name   string
		models []imagegeneration.PublicModel
	}{
		{
			name: "disabled",
			models: []imagegeneration.PublicModel{{
				ID:      modelcatalog.MiniAppImageNanoBanana2,
				Enabled: false,
				Ready:   true,
			}},
		},
		{
			name: "unready",
			models: []imagegeneration.PublicModel{{
				ID:      modelcatalog.MiniAppImageNanoBanana2,
				Enabled: true,
				Ready:   false,
			}},
		},
		{
			name: "not in ready public catalog",
			models: []imagegeneration.PublicModel{{
				ID:      modelcatalog.MiniAppImageSeedream45,
				Enabled: true,
				Ready:   true,
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := imagegeneration.NewResolver(test.models, pricing).Resolve(imagegeneration.Request{
				ModelID: modelcatalog.MiniAppImageNanoBanana2,
			})
			if !errors.Is(err, imagegeneration.ErrPublicModelUnavailable) {
				t.Fatalf("resolve error = %v, want ErrPublicModelUnavailable", err)
			}
		})
	}
}

func TestResolver_FailsClosedWhenReadyPublicCatalogIsMissing(t *testing.T) {
	_, err := imagegeneration.NewResolver(nil, staticPricingCatalog(t)).Resolve(imagegeneration.Request{
		ModelID: modelcatalog.MiniAppImageNanoBanana2,
	})
	if !errors.Is(err, imagegeneration.ErrPublicModelUnavailable) {
		t.Fatalf("resolve error = %v, want ErrPublicModelUnavailable", err)
	}
}

func TestResolverRejectsLegacyAliasThatIsNotInThePublicCatalog(t *testing.T) {
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{
		ID:                     modelcatalog.MiniAppImageNanoBananaPro,
		Name:                   "Nano Banana Pro",
		Enabled:                true,
		Ready:                  true,
		QualityOptions:         []string{modelcatalog.ImageQuality1K},
		DefaultQuality:         modelcatalog.ImageQuality1K,
		SupportsReferenceImage: true,
		MaxReferenceImages:     1,
	}}, staticPricingCatalog(t))

	_, err := resolver.Resolve(imagegeneration.Request{ModelID: "kandinsky"})
	if !errors.Is(err, imagegeneration.ErrPublicModelUnavailable) {
		t.Fatalf("legacy private alias error = %v, want ErrPublicModelUnavailable", err)
	}
}

func TestResolver_RejectsUnsupportedQualityAndReferenceCount(t *testing.T) {
	pricing := staticPricingCatalog(t)
	resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{
		ID:                     modelcatalog.MiniAppImageNanoBanana2,
		Name:                   "Nano Banana 2",
		Enabled:                true,
		Ready:                  true,
		QualityOptions:         []string{modelcatalog.ImageQuality1K, modelcatalog.ImageQuality2K},
		DefaultQuality:         modelcatalog.ImageQuality1K,
		SupportsReferenceImage: true,
		MaxReferenceImages:     2,
	}}, pricing)

	_, err := resolver.Resolve(imagegeneration.Request{
		ModelID: modelcatalog.MiniAppImageNanoBanana2,
		Quality: modelcatalog.ImageQuality4K,
	})
	if !errors.Is(err, imagegeneration.ErrUnsupportedQuality) {
		t.Fatalf("quality error = %v, want ErrUnsupportedQuality", err)
	}

	_, err = resolver.Resolve(imagegeneration.Request{
		ModelID:        modelcatalog.MiniAppImageNanoBanana2,
		ReferenceCount: 3,
	})
	if !errors.Is(err, imagegeneration.ErrReferenceLimit) {
		t.Fatalf("reference error = %v, want ErrReferenceLimit", err)
	}
}

func TestPublicSelectionAndRequestCannotCarryProviderOrClientPrice(t *testing.T) {
	selectionJSON, err := json.Marshal(imagegeneration.PublicSelection{
		ModelID:      modelcatalog.MiniAppImageNanoBanana2,
		ModelName:    "Nano Banana 2",
		ImageQuality: modelcatalog.ImageQuality1K,
	})
	if err != nil {
		t.Fatalf("marshal public selection: %v", err)
	}
	serialized := strings.ToLower(string(selectionJSON))
	for _, forbidden := range []string{"provider", "model_code", "price", "floor", "multiplier"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("public selection leaked %q: %s", forbidden, selectionJSON)
		}
	}

	requestType := reflect.TypeOf(imagegeneration.Request{})
	for _, forbidden := range []string{"Price", "Cost", "Provider", "ModelCode", "Snapshot"} {
		if _, ok := requestType.FieldByName(forbidden); ok {
			t.Fatalf("request must not accept client-owned %s", forbidden)
		}
	}
}

func staticPricingCatalog(t *testing.T) *pricingcatalog.Catalog {
	t.Helper()
	catalog, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		t.Fatalf("new static pricing catalog: %v", err)
	}
	return catalog
}
