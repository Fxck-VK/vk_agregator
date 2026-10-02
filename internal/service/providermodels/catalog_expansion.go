package providermodels

import "vk-ai-aggregator/internal/domain"

const catalogExpansionCheckedAt = "2026-09-28"

func catalogExpansionCandidates() []MediaCandidate {
	nanoAPI := ImageCapabilities{
		Images:       inputCapability(Supported, integer(14), "jpg", "jpeg", "png", "webp"),
		AspectRatios: []string{"1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9"},
		Resolutions:  []string{"1K"}, MaxOutputCount: integer(1),
	}
	nanoApp := nanoAPI
	nanoApp.Images = noInput()
	out := []MediaCandidate{{
		PublicID: "nano_banana", Name: "Nano Banana", Kind: "image",
		Provider: domain.ProviderAPIMart, ModelCode: "gemini-2.5-flash-image-preview",
		Documentation: apimartDocsBase + "images/gemini-2.5-flash/generation",
		CheckedAt:     catalogExpansionCheckedAt,
		Capabilities: ModelCapabilities{
			SchemaVersion: 1,
			API:           CapabilityProfile{Image: &nanoAPI, Notes: []string{"Промпт до 1000 символов; до 14 изображений, JPEG/PNG/WebP, до 10 МБ каждое. Выход: одно изображение 1K."}},
			Application:   CapabilityProfile{Image: &nanoApp, Notes: []string{"Ожидает допуска. Подготовлена генерация по тексту; загрузка референсов не подключена."}},
		},
	}}
	for _, spec := range []struct {
		id, name, native, path   string
		min, max, images, videos int
		res, ratios              []string
	}{
		{"grok_imagine_1_5_video", "Grok Imagine 1.5 Video", "grok-imagine-1.5-video-ext", "videos/grok-imagine/generation", 6, 15, 7, 0, []string{"480p", "720p"}, []string{"16:9", "9:16", "1:1", "3:2", "2:3"}},
		{"kling_2_6", "Kling 2.6", "kling-v2-6", "videos/kling-v2-6/generation", 5, 10, 2, 0, []string{"720p", "1080p"}, []string{"16:9", "9:16", "1:1"}},
		{"seedance_2_0", "Seedance 2.0", "seedance-2.0", "videos/seedance-2-0/generation", 4, 15, 9, 3, []string{"480p", "720p", "1080p", "4k"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}},
		{"seedance_2_0_mini", "Seedance 2.0 Mini", "seedance-2.0-mini", "videos/seedance-2-0/generation", 4, 15, 9, 3, []string{"480p", "720p"}, []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"}},
	} {
		c := nextVideoCandidate(spec.id, spec.name, spec.native, apimartDocsBase+spec.path, spec.min, spec.max, spec.res, spec.images, spec.videos)
		c.CheckedAt = catalogExpansionCheckedAt
		api, app := c.Capabilities.API.Video, c.Capabilities.Application.Video
		// Counts are documented; unlisted file formats must not be inferred from other models.
		api.Images.Extensions = []string{}
		api.Videos.Extensions = []string{}
		api.AspectRatios = append([]string(nil), spec.ratios...)
		app.AspectRatios = append([]string(nil), spec.ratios...)
		switch spec.id {
		case "grok_imagine_1_5_video":
			api.Audio = VideoAudioCapability{Mode: videoAudioUnknown}
			api.StartFrame = "unknown"
			api.EndFrame = "unknown"
			c.Capabilities.API.Notes = []string{"До 7 изображений по публичным URL, без Base64. Режим звука и семантика первого/последнего кадра не документированы."}
		case "kling_2_6":
			api.Duration.AllowedSeconds = []int{5, 10}
			app.Duration.AllowedSeconds = []int{5, 10}
			api.QualityModes = []string{"std", "pro"}
			app.QualityModes = []string{"std", "pro"}
			c.Capabilities.API.Notes = []string{"Промпт до 2500 символов. Std: 720p без звука. Pro: 1080p, опциональный звук. Последний кадр только в Pro и несовместим со звуком."}
		default:
			c.Capabilities.API.Notes = []string{"До 9 изображений или до 3 референсных видео; также есть референсное аудио. Первый/последний кадры исключают видео и аудиовход. Standard: промпт до 4000 символов. Для Mini точный лимит символов не указан."}
		}
		c.Capabilities.Application.Notes = []string{"Ожидает допуска. Подготовлена генерация по тексту; вложения и переключение звука не подключены. FPS и фактический результат требуют платной live-проверки."}
		out = append(out, c)
	}
	return out
}

func catalogExpansionFacts(c MediaCandidate) []MediaCandidateOperationFact {
	switch c.PublicID {
	case "nano_banana", "grok_imagine_1_5_video", "kling_2_6", "seedance_2_0", "seedance_2_0_mini", "seedream_5_0_flash", "z_image_turbo", "flux_2_max", "flux_2_flex", "qwen_image_3_pro":
	default:
		return nil
	}
	endpoint, op := "POST /v1/images/generations", "generate"
	if c.Kind == "video" {
		endpoint, op = apimartVideoEndpoint, "text_to_video"
	}
	limits := append([]string(nil), c.Capabilities.API.Notes...)
	// Facts for media inputs live in the API capability profile. Only the implemented
	// text operation is proposed for admission; no unimplemented operation is enabled.
	return []MediaCandidateOperationFact{{ID: op, Kind: c.Kind, Endpoint: endpoint, NativeVersion: c.ModelCode, SourceID: c.PublicID + "_generation", InputModes: []string{"prompt"}, KnownLimits: limits}}
}
