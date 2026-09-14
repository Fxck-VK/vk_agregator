package providermodels

import "strings"

func imageCapabilities(m ImageModel) *ModelCapabilities {
	app := imageProfile(m)
	api := imageProfile(m)
	// This is a verified subset of the adapter's API contract, not a claim that
	// application limits are exhaustive native limits. Native-only differences
	// are recorded below; undocumented ceilings remain unknown.
	api.Notes = []string{"Указаны подтверждённые параметры подключённого API. Полный набор возможностей и неуказанные ограничения требуют проверки документации."}
	api.Image.Images.Extensions = []string{}
	api.Image.Images.MaxCount = nil
	api.Image.MaxOutputCount = nil
	if !m.Limits.SupportsReferenceImage {
		api.Image.Images = unknownInput()
	}
	switch m.PublicID {
	case PublicImageFlux2Pro:
		// https://docs.apimart.ai/ru/api-reference/images/flux-2/generation
		// Checked 2026-09-14: image_urls <= 8, n=1; output + refs <= 9 MP.
		api.Image.Images = inputCapability(Supported, integer(8))
		api.Image.MaxOutputCount = integer(1)
		api.Image.AspectRatios = append(api.Image.AspectRatios, "auto")
		api.Notes = []string{"API принимает до 8 фото; суммарно выход и референсы — до 9 МП. Поддерживает точные размеры до 4 МП. Загрузка референсов в этой интеграции ещё не подключена."}
	case PublicImageMidjourneyV7:
		// Imagine documents references but no maximum. A single result is a grid.
		api.Notes = []string{"Imagine принимает фото; точный максимум не подтверждён. Relax/Fast/Turbo — скорость. Разрешение 1K/2K/4K этим запросом не выбирается."}
		app.Notes = []string{"Relax/Fast/Turbo — скорость генерации. Один результат Imagine может содержать сетку изображений."}
	case PublicImageSeedream50Lite:
		api.Image.Images.MaxCount = integer(14)
		api.Image.MaxOutputCount = integer(15)
		api.Image.AspectRatios = append(api.Image.AspectRatios, "auto")
	case PublicImageSeedream50Pro:
		api.Image.Images.MaxCount = integer(10)
		api.Image.MaxOutputCount = integer(1)
	case PublicImageNanoBanana2:
		api.Image.AspectRatios = append(api.Image.AspectRatios, "1:4", "4:1", "1:8", "8:1")
	case PublicImageNanoBananaPro:
		api.Image.AspectRatios = append(api.Image.AspectRatios, "auto")
	case PublicImageGPTImage2:
		api.Image.AspectRatios = append(api.Image.AspectRatios, "2:1", "1:2", "3:1", "1:3", "9:21")
	case PublicImageGPTImage25Flare, PublicImageGPTImage25Sunburst:
		// The adapter implements native image input; product pricing deliberately
		// rejects it until reference input cost has a verified bounded quote.
		api.Image.Images = inputCapability(Supported, nil)
		api.Notes = []string{"API предусматривает референсы; максимальное количество требует подтверждения. В приложении референсы отключены до проверки их тарификации."}
	}
	return &ModelCapabilities{SchemaVersion: 1, API: api, Application: app}
}

func imageProfile(m ImageModel) CapabilityProfile {
	input := noInput()
	if m.Limits.SupportsReferenceImage {
		input = inputCapability(Supported, integer(m.Limits.MaxReferenceImages), "jpg", "jpeg", "png")
	}
	c := &ImageCapabilities{Images: input, AspectRatios: append([]string{}, m.Limits.AllowedAspectRatios...), MaxOutputCount: integer(m.Limits.MaxOutputCount)}
	setImageOptions(c, m.PublicID, m.Limits.AllowedQualities)
	if m.PublicID == PublicImageSeedream50Lite {
		c.MaxCombinedImages = integer(15)
	}
	return CapabilityProfile{Image: c}
}

func setImageOptions(c *ImageCapabilities, id string, options []string) {
	c.Resolutions, c.QualityModes, c.SpeedModes = []string{}, []string{}, []string{}
	for _, option := range options {
		switch id {
		case PublicImageMidjourneyV7:
			c.SpeedModes = appendUnique(c.SpeedModes, option)
		case PublicImageGrokImage15, PublicImageGrokImage20:
			c.QualityModes = appendUnique(c.QualityModes, option)
		case PublicImageGPTImage25Flare, PublicImageGPTImage25Sunburst:
			resolution, quality, ok := strings.Cut(option, "-")
			if ok {
				c.Resolutions = appendUnique(c.Resolutions, resolution)
				c.QualityModes = appendUnique(c.QualityModes, quality)
			}
		default:
			c.Resolutions = appendUnique(c.Resolutions, option)
		}
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// ImageCapabilitiesForSurface narrows application metadata to an actual public
// request contract and its priced options. It never modifies the API profile.
func ImageCapabilitiesForSurface(id, surface string, pricedOptions []string) *ModelCapabilities {
	c := Capabilities(id)
	if c == nil || c.Application.Image == nil {
		return nil
	}
	setImageOptions(c.Application.Image, id, pricedOptions)
	switch surface {
	case "miniapp":
		c.Application.Image.MaxOutputCount = integer(1)
		c.Application.Image.AspectRatios = []string{"1:1"}
		c.Application.Notes = append(c.Application.Notes, "В Mini App сейчас доступен один квадратный результат за запрос.")
	case "web":
		c.Application.Image.Images = noInput()
		c.Application.Notes = append(c.Application.Notes, "На сайте загрузка фото в запрос генерации ещё не подключена.")
	}
	return c
}
