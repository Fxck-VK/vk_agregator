package apimart

import (
	"context"
	"testing"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

// Exercise the actual public projection against the adapter and quote path, so
// a catalog-only change cannot silently publish an unpriceable wire combination.
func TestCatalogExpansionEveryCatalogVariantBuildsAndPrices(t *testing.T) {
	catalog := productcatalog.WorkspaceCatalog(productcatalog.WorkspaceConfig{IncludePendingMedia: true})
	count := 0
	for _, m := range catalog.Items {
		c, ok := providermodels.MediaCandidateByID(m.ID)
		if !ok || catalogExpansionID(c.ModelCode) == "" {
			continue
		}
		count++
		if m.Verification != "pending-verification" || len(m.Operations) != 1 || m.Operations[0].Enabled {
			t.Fatalf("premature admission: %s", m.ID)
		}
		op := m.Operations[0]
		if op.Video != nil {
			for _, v := range op.Video.Variants {
				r := expansionRequest(c.ModelCode)
				r.DurationSec = v.DurationSec
				r.Resolution = v.Resolution
				r.AspectRatio = v.AspectRatio
				if _, err := buildNextVisualBody(r); err != nil {
					t.Fatalf("%s variant %+v: %v", m.ID, v, err)
				}
				q, err := pricingcatalog.MediaVideoCandidateQuote(m.ID, "", v.Resolution, v.DurationSec)
				if err != nil || q.InternalCredits <= 0 {
					t.Fatalf("%s unpriced variant %+v", m.ID, v)
				}
				if _, err := New(Config{}).Estimate(context.Background(), r); err != nil {
					t.Fatal(err)
				}
			}
		} else {
			if op.Image == nil || op.Image.DefaultQuality != "1K" || op.Image.PriceByQuality["1K"] != 10 {
				t.Fatal("Nano 1K controls/price missing")
			}
			for _, ratio := range op.Image.AllowedAspectRatios {
				r := expansionRequest(c.ModelCode)
				r.AspectRatio = ratio
				if _, err := buildNextVisualBody(r); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if count != 5 {
		t.Fatalf("catalog contains %d expansion models", count)
	}
}

func TestCatalogExpansionNativeFramesAndSound(t *testing.T) {
	frames := &domain.VideoMediaRequest{Mode: domain.VideoMediaModeImage, StartFrame: &domain.VideoFrame{URL: "https://example.com/start.png"}, EndFrame: &domain.VideoFrame{URL: "https://example.com/end.png"}}
	for _, model := range []string{ModelSeedance20, ModelSeedance20Mini, ModelKling26} {
		r := expansionRequest(model)
		r.VideoMedia = frames
		if model == ModelKling26 {
			r.Resolution = "1080p"
		}
		body, err := buildNextVisualBody(r)
		if err != nil {
			t.Fatal(err)
		}
		got := decodeNextVisualBody(t, body)
		if model == ModelKling26 {
			if got["mode"] != "pro" || len(got["image_urls"].([]any)) != 2 {
				t.Fatal("Kling pro frames missing")
			}
			r.VideoAudio = true
			if _, err := buildNextVisualBody(r); err == nil {
				t.Fatal("audio and last frame accepted")
			}
			r.VideoMedia = nil
			body, err = buildNextVisualBody(r)
			if err != nil || decodeNextVisualBody(t, body)["audio"] != true {
				t.Fatal("Kling pro sound failed")
			}
			q, err := estimateNextVisual(r)
			if err != nil || q.AmountCredits != 7 {
				t.Fatalf("Kling pro sound underpriced: %+v %v", q, err)
			}
		} else {
			if _, ok := got["image_urls"]; ok {
				t.Fatal("Seedance frame roles lost")
			}
			roles := got["image_with_roles"].([]any)
			if len(roles) != 2 || roles[0].(map[string]any)["role"] != "first_frame" || roles[1].(map[string]any)["role"] != "last_frame" {
				t.Fatal("wrong Seedance roles")
			}
		}
	}
}
