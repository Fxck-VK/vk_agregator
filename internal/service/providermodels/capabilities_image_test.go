package providermodels

import (
	"reflect"
	"testing"
)

func TestImageCapabilitiesSeparateSizeDetailAndSpeed(t *testing.T) {
	gpt := Capabilities(PublicImageGPTImage25Flare).Application.Image
	if !reflect.DeepEqual(gpt.Resolutions, []string{"1K", "2K", "4K"}) || len(gpt.QualityModes) != 5 || len(gpt.SpeedModes) != 0 {
		t.Fatalf("GPT image size and detail are mixed: %+v", gpt)
	}
	mj := Capabilities(PublicImageMidjourneyV7).Application.Image
	if len(mj.Resolutions) != 0 || !reflect.DeepEqual(mj.SpeedModes, []string{"relax", "fast", "turbo"}) {
		t.Fatalf("Midjourney speed must not be advertised as resolution: %+v", mj)
	}
	flux := Capabilities(PublicImageFlux2Pro)
	if flux.API.Image.Images.Support != Supported || *flux.API.Image.Images.MaxCount != 8 || flux.Application.Image.Images.Support != Unsupported {
		t.Fatalf("Flux native reference support must remain separate from integration: %+v", flux)
	}
}

func TestImageSurfaceCapabilitiesMatchActualRequestPaths(t *testing.T) {
	mini := ImageCapabilitiesForSurface(PublicImageSeedream50Lite, "miniapp", []string{"2K"})
	web := ImageCapabilitiesForSurface(PublicImageSeedream50Lite, "web", []string{"2K"})
	if *mini.Application.Image.MaxOutputCount != 1 || !reflect.DeepEqual(mini.Application.Image.AspectRatios, []string{"1:1"}) || mini.Application.Image.Images.Support != Supported {
		t.Fatal("Mini App must expose one square output and supported references")
	}
	if web.Application.Image.Images.Support != Unsupported || *web.Application.Image.Images.MaxCount != 0 {
		t.Fatal("Web prepare API has no image-reference input")
	}
	if !reflect.DeepEqual(web.Application.Image.Resolutions, []string{"2K"}) || len(web.API.Image.Resolutions) != 3 {
		t.Fatal("Runtime price filtering must affect only application options")
	}
	mini.API.Image.Resolutions[0] = "changed"
	if Capabilities(PublicImageSeedream50Lite).API.Image.Resolutions[0] == "changed" {
		t.Fatal("Capabilities must return independent values")
	}
}
