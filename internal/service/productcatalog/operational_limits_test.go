package productcatalog_test

import (
	"encoding/json"
	"testing"

	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/productcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestWorkspaceCatalogClampsStaleImageReferences(t *testing.T) {
	registry := providermodels.StaticRegistry()
	model, ok := registry.PublicImageModel(providermodels.PublicImageGPTImage2)
	if !ok {
		t.Fatal("GPT Image 2 missing")
	}
	catalog := productcatalog.WorkspaceCatalog(productcatalog.WorkspaceConfig{
		ImageReferenceUploads: true, Pricing: staticPricingCatalog(t),
		ImageModels: []imagegeneration.PublicModel{{ID: model.PublicID, Name: model.DisplayName, Enabled: true, Ready: true, QualityOptions: model.Limits.AllowedQualities, DefaultQuality: model.Limits.AllowedQualities[0], AllowedAspectRatios: model.Limits.AllowedAspectRatios, SupportsReferenceImage: true, MaxReferenceImages: 16, MaxOutputCount: 4}},
	})
	if len(catalog.Items) != 1 {
		t.Fatalf("catalog models: %d", len(catalog.Items))
	}
	operation := catalog.Items[0].Operations[0]
	if operation.Image.MaxReferenceImages != 15 || operation.Inputs.Images.MaxCount != 15 || operation.Image.MaxOutputCount != 1 || operation.Inputs.MaxTotalBytes != 256<<20 {
		t.Fatalf("stale reference controls: %+v %+v", operation.Image, operation.Inputs)
	}
}

func TestWorkspaceCatalogOperationalLimits(t *testing.T) {
	catalog, err := productcatalog.WorkspacePreviewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, model := range catalog.Items {
		for _, op := range model.Operations {
			switch model.ID {
			case "nano_banana_2", "nano_banana_pro", "gpt_image_2":
				seen[model.ID] = true
				if op.Image.MaxOutputCount != 1 || op.Image.ShowOutputCount {
					t.Errorf("%s still offers multi-image selection", model.ID)
				}
				if model.ID == "gpt_image_2" && op.Inputs.MaxTotalBytes != 256<<20 {
					t.Errorf("GPT Image 2 total reference bytes = %d, want %d", op.Inputs.MaxTotalBytes, int64(256<<20))
				}
			case "seedream_4_5", "video_runway_gen4_5", "video_seedance_2_0_fast":
				seen[model.ID] = true
				var controls any = op.Image
				if op.Video != nil {
					controls = op.Video
				}
				raw, _ := json.Marshal(controls)
				var values map[string]any
				_ = json.Unmarshal(raw, &values)
				wantMax := map[string]float64{"seedream_4_5": 3000, "video_runway_gen4_5": 1800, "video_seedance_2_0_fast": 2000}[model.ID]
				if values["max_prompt_chars"] != wantMax {
					t.Errorf("%s max prompt chars = %v, want %v", model.ID, values["max_prompt_chars"], wantMax)
				}
				if model.ID == "video_runway_gen4_5" && (values["automatic_resolution"] != true || len(op.Video.AllowedResolutions) != 1) {
					t.Error("Runway exposes undocumented resolution selection")
				}
				if model.ID == "video_seedance_2_0_fast" && values["min_prompt_chars"] != float64(3) {
					t.Error("Seedance minimum prompt limit missing")
				}
			}
			if op.Video != nil && op.Inputs.Images.Required {
				if op.Inputs.Images.MaxBytes != productcatalog.WebReferenceMaxBytes {
					t.Errorf("%s reference size does not match transport", model.ID)
				}
				for _, format := range op.Inputs.Images.Formats {
					if format.MIME != "image/png" && format.MIME != "image/jpeg" {
						t.Errorf("%s advertises unwired %s", model.ID, format.MIME)
					}
				}
			}
		}
	}
	if len(seen) != 6 {
		t.Fatalf("covered %d corrected models, want 6", len(seen))
	}
}
