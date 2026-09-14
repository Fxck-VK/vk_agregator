package miniapp

import "vk-ai-aggregator/internal/service/providermodels"

func miniAppImageCapabilities(model ImageModelDTO) *providermodels.ModelCapabilities {
	c := providermodels.ImageCapabilitiesForSurface(model.ID, "miniapp", model.QualityOptions)
	if c == nil {
		return nil
	}
	clampImageInput(&c.Application.Image.Images, model.SupportsReferenceImage, model.MaxReferenceImages)
	return c
}

func miniAppVideoCapabilities(route VideoRouteDTO) *providermodels.ModelCapabilities {
	c := providermodels.VideoCapabilitiesForOptions(route.Alias, route.AllowedDurationsSec, route.AllowedResolutions)
	if c == nil {
		return nil
	}
	v := c.Application.Video
	clampImageInput(&v.Images, route.SupportsReferenceImage, route.MaxReferenceImages)
	if v.Images.MaxCount != nil {
		counts := make([]int, 0, len(v.AllowedImageCounts))
		for _, n := range v.AllowedImageCounts {
			if n > *v.Images.MaxCount {
				continue
			}
			if len(route.AllowedReferenceImageCounts) > 0 {
				found := false
				for _, allowed := range route.AllowedReferenceImageCounts {
					if allowed == n {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			counts = append(counts, n)
		}
		v.AllowedImageCounts = counts
	}
	return c
}

func clampImageInput(input *providermodels.InputCapability, enabled bool, maximum int) {
	if !enabled || maximum <= 0 {
		zero := 0
		*input = providermodels.InputCapability{Support: providermodels.Unsupported, MaxCount: &zero, Extensions: []string{}}
		return
	}
	if input.MaxCount != nil && maximum < *input.MaxCount {
		input.MaxCount = &maximum
	}
}
