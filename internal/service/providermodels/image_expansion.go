package providermodels

import "vk-ai-aggregator/internal/domain"

const imageExpansionCheckedAt = "2026-09-30"

func imageExpansionCandidates() []MediaCandidate {
	ratios := []string{"1:1", "4:3", "3:4", "16:9", "9:16", "3:2", "2:3"}
	out := []MediaCandidate{}
	for _, spec := range []struct {
		id, name, native, path string
		resolutions            []string
		refs, outputs          int
		formats                []string
		note                   string
	}{
		{"seedream_5_0_flash", "Seedream 5.0 Flash", "seedream-5-0-flash", "seedream-5-0-flash", []string{"1K", "1.5K", "2K"}, 10, 1, []string{"jpeg", "png", "webp", "bmp", "tiff", "gif", "heic", "heif"}, "Референс до 30 МБ и 36 МП. API также поддерживает точные пиксели, прозрачность и разложение на слои отдельными режимами. Тариф сайта расходится с документацией: предварительная цена рассчитана по верхнему документированному тарифу."},
		{"z_image_turbo", "Z-Image Turbo", "z-image-turbo", "z-image-turbo", []string{"1K", "2K"}, 0, 1, nil, "Промпт до 800 символов; ровно одно изображение. Параметр n не поддерживается. Перезапись промпта оплачивается отдельно."},
		{"flux_2_max", "FLUX.2 Max", "flux-2-max", "flux-2", []string{"1MP", "2MP", "3MP", "4MP"}, 8, 1, nil, "До 8 референсов; выход до 4 МП, выход вместе с референсами до 9 МП. МП не равны уровням 1K/2K/4K. Форматы входных файлов в документации не перечислены."},
		{"flux_2_flex", "FLUX.2 Flex", "flux-2-flex", "flux-2", []string{"1MP", "2MP", "3MP", "4MP"}, 8, 1, nil, "До 8 референсов; выход до 4 МП, выход вместе с референсами до 9 МП. API допускает steps 1–50 и guidance 1.5–10. Форматы входных файлов в документации не перечислены."},
		{"qwen_image_3_pro", "Qwen Image 3.0 Pro", "qwen-image-3.0-pro", "qwen-image-3.0", []string{"1K", "2K"}, 3, 6, []string{"jpg", "jpeg", "png", "bmp", "tiff", "webp", "gif"}, "До 3 референсов по 10 МБ; промпт до примерно 4500 токенов. API поддерживает 1–6 результатов, негативный промпт и перезапись. Стоимость референсов на сайте расходится с документацией; загрузка выключена."},
	} {
		api := ImageCapabilities{Images: noInput(), AspectRatios: append([]string(nil), ratios...), Resolutions: spec.resolutions, MaxOutputCount: integer(spec.outputs)}
		if spec.refs > 0 {
			api.Images = inputCapability(Supported, integer(spec.refs), spec.formats...)
		}
		if spec.id == "seedream_5_0_flash" {
			api.AspectRatios = append(api.AspectRatios, "2:1", "1:2", "21:9")
		}
		if spec.id == "flux_2_max" || spec.id == "flux_2_flex" {
			api.AspectRatios = append(api.AspectRatios, "21:9", "9:21")
		}
		app := api
		app.Images = noInput()
		app.MaxOutputCount = integer(1)
		app.AspectRatios = append([]string(nil), api.AspectRatios...)
		if spec.id == "seedream_5_0_flash" || spec.id == "flux_2_max" || spec.id == "flux_2_flex" {
			api.AspectRatios = append(api.AspectRatios, "auto")
		}
		out = append(out, MediaCandidate{PublicID: spec.id, Name: spec.name, Kind: "image", Provider: domain.ProviderAPIMart, ModelCode: spec.native, Documentation: apimartDocsBase + "images/" + spec.path + "/generation", CheckedAt: imageExpansionCheckedAt, Capabilities: ModelCapabilities{SchemaVersion: 1, API: CapabilityProfile{Image: &api, Notes: []string{spec.note}}, Application: CapabilityProfile{Image: &app, Notes: []string{"Подготовлена генерация одного изображения по тексту с выбранными пропорциями и разрешением. Референсы, точные пиксели, слои и настройка перезаписи выключены. Реальный результат ещё не проверен."}}}})
	}
	return out
}

func ImageCandidateQualities(c MediaCandidate) []string {
	if c.Capabilities.Application.Image != nil && len(c.Capabilities.Application.Image.Resolutions) > 0 {
		return append([]string(nil), c.Capabilities.Application.Image.Resolutions...)
	}
	return []string{"standard"}
}

func imageExpansionRoute(provider domain.ProviderName, model string) bool {
	if provider != domain.ProviderAPIMart {
		return false
	}
	for _, c := range imageExpansionCandidates() {
		if c.ModelCode == model {
			return true
		}
	}
	return false
}
