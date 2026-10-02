package productcatalog

import "vk-ai-aggregator/internal/service/providermodels"

// Public capability descriptions follow the same priced controls as submission.
// Native support never enables uploads or controls on the web surface.
func workspaceCapabilities(id string, op WorkspaceOperation) *providermodels.ModelCapabilities {
	if c, ok := providermodels.MediaCandidateByID(id); ok {
		if providermodels.IsVideoExpansion(id) && op.Kind == "video" && op.Enabled && c.Capabilities.Application.Video != nil {
			app := c.Capabilities.Application.Video
			c.Capabilities.Application.Notes = []string{"Доступна для ручного тестирования на DEV; проверка результатов ещё не завершена."}
			if op.Inputs.Images.Enabled {
				maximum := op.Inputs.Images.MaxCount
				app.Images = providermodels.InputCapability{Support: providermodels.Supported, MaxCount: &maximum, Extensions: []string{"png", "jpg", "jpeg"}}
				app.AllowedImageCounts = append([]int(nil), op.Inputs.Images.AllowedCounts...)
				c.Capabilities.Application.Notes = append(c.Capabilities.Application.Notes, "Нужны 1–7 фото PNG/JPEG до 20 МиБ, от 128 до 4096 пикселей по стороне, пропорции 1:4–4:1.")
			}
		}
		return &c.Capabilities
	}
	switch op.Kind {
	case "text":
		return providermodels.Capabilities(id)
	case "image":
		if op.Image == nil {
			return nil
		}
		surface := "web"
		if op.Inputs.Images.Enabled {
			surface = "web-references"
		}
		c := providermodels.ImageCapabilitiesForSurface(id, surface, op.Image.QualityOptions)
		if c != nil {
			maximum := op.Image.MaxOutputCount
			c.Application.Image.MaxOutputCount = &maximum
			c.Application.Image.AspectRatios = append([]string(nil), op.Image.AllowedAspectRatios...)
		}
		return c
	case "video":
		if op.Video == nil {
			return nil
		}
		c := providermodels.VideoCapabilitiesForOptions(id, op.Video.AllowedDurationsSec, op.Video.AllowedResolutions)
		if c != nil {
			v := c.Application.Video
			zero := 0
			v.Images = providermodels.InputCapability{Support: providermodels.Unsupported, Extensions: []string{}, MaxCount: &zero}
			v.Videos = providermodels.InputCapability{Support: providermodels.Unsupported, Extensions: []string{}, MaxCount: &zero}
			v.AllowedImageCounts = []int{0}
			v.StartFrame, v.EndFrame = "unsupported", "unsupported"
			v.AspectRatios = append([]string(nil), op.Video.AllowedAspectRatios...)
			v.Audio.Selectable = false
			if v.Audio.Mode == "optional" {
				v.Audio.Mode = "unknown"
			}
			c.Application.Notes = append(c.Application.Notes, "На сайте видеозапрос принимает только текст. Загрузка референсов и переключатель звука ещё не подключены.")
		}
		return c
	default:
		return nil
	}
}
