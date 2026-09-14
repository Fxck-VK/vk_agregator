package miniapp

import (
	"testing"
	"vk-ai-aggregator/internal/service/providermodels"
)

func TestCatalogImageCapabilitiesDescribeMiniAppContract(t *testing.T) {
	model := ImageModelDTO{ID: providermodels.PublicImageSeedream50Lite, QualityOptions: []string{"2K"}, SupportsReferenceImage: true, MaxReferenceImages: 14}
	item := modelCatalogItemFromImage(model)
	if item.Capabilities == nil {
		t.Fatal("capability metadata missing from public catalog")
	}
	app, api := item.Capabilities.Application.Image, item.Capabilities.API.Image
	if *app.MaxOutputCount != 1 || len(app.AspectRatios) != 1 || app.AspectRatios[0] != "1:1" || len(app.Resolutions) != 1 {
		t.Fatal("Mini App contract was broadened by metadata")
	}
	if *api.MaxOutputCount != 15 || len(api.Resolutions) != 3 {
		t.Fatal("Application restrictions overwrote API metadata")
	}
	model.MaxReferenceImages = 4
	if got := modelCatalogItemFromImage(model); *got.Capabilities.Application.Image.Images.MaxCount != 4 {
		t.Fatal("runtime reference limit was ignored")
	}
}

func TestCatalogVideoCapabilitiesDoNotAdvertiseAutomaticBillingDuration(t *testing.T) {
	item := modelCatalogItemFromVideo(VideoRouteDTO{Alias: "video_gemini_omni_1_1_flash", AutomaticDuration: true, AllowedDurationsSec: []int{10}, AllowedResolutions: []string{"720p"}})
	if item.Capabilities == nil || item.Capabilities.Application.Video.Duration.Mode != "automatic" {
		t.Fatal("automatic duration metadata missing")
	}
	if len(item.Capabilities.Application.Video.Duration.AllowedSeconds) != 0 {
		t.Fatal("Billing duration must not become generated duration")
	}
}
