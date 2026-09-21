package productcatalog

import (
	"slices"
	"strconv"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/modelcontract"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

func WorkspaceCatalog(cfg WorkspaceConfig) WorkspaceModelList {
	out := WorkspaceModelList{SchemaVersion: 1, Items: []WorkspaceModel{}}
	seen := map[string]bool{}
	registry := providermodels.StaticRegistry()
	add := func(id, name, description string, op WorkspaceOperation, free bool) {
		if seen[id] || id == "" || name == "" {
			return
		}
		seen[id] = true
		categories := []string{"popular"}
		switch op.Kind {
		case "text":
			categories = append(categories, "text", "study-work")
		case "image":
			categories = append(categories, "images")
		case "video", "audio":
			categories = append(categories, "video-audio")
		}
		if free {
			categories = append(categories, "free")
		}
		model := WorkspaceModel{ID: id, Name: name, Description: description, Kind: op.Kind, Categories: categories, Verification: "legacy-unverified", Operations: []WorkspaceOperation{op}}
		if contract, ok := registry.Contracts[id]; ok && contract.ValidateReady() == nil {
			model.Verification, model.Version = "verified-contract", contract.Revision
			model.Categories = append([]string(nil), contract.Categories...)
			for _, sourceOp := range contract.Operations {
				if sourceOp.Kind != op.Kind || sourceOp.ID != op.ID {
					continue
				}
				model.Operations[0].Inputs = sourceOp.Inputs
				// Owned web uploads are not wired yet. Documented provider support
				// does not by itself enable an input in this product surface.
				inputs := &model.Operations[0].Inputs
				inputs.Images.Enabled, inputs.Video.Enabled, inputs.Audio.Enabled, inputs.Documents.Enabled = false, false, false, false
				if sourceOp.Text != nil && op.Text != nil {
					op.Text.ContextTokens = sourceOp.Text.ContextTokens
				}
				break
			}
		}
		model.Categories = workspacePriceCategories(model.Categories, free)
		if cfg.ImageReferenceUploads && op.Image != nil && op.Image.SupportsReferenceImage && op.Image.MaxReferenceImages > 0 {
			// Reuse the admitted runtime reference path. This adds a web transport,
			// without enabling new provider formats, models, or reference-only routes.
			input := modelcontract.Input{Support: modelcontract.Supported, Enabled: true, Processing: "native", MaxCount: op.Image.MaxReferenceImages, MaxBytes: WebReferenceMaxBytes, MaxWidth: WebReferenceMaxDimension, MaxHeight: WebReferenceMaxDimension, Formats: []modelcontract.FileFormat{{Extension: ".png", MIME: "image/png"}, {Extension: ".jpg", MIME: "image/jpeg"}, {Extension: ".jpeg", MIME: "image/jpeg"}}}
			model.Operations[0].Inputs.Images = input
			model.Operations[0].Inputs.MaxTotalBytes = int64(input.MaxCount) * input.MaxBytes
		}
		model.Capabilities = workspaceCapabilities(id, model.Operations[0])
		out.Items = append(out.Items, model)
	}
	resolver := imagegeneration.NewResolver(cfg.ImageModels, cfg.Pricing)
	for _, image := range cfg.ImageModels {
		controls, ok := WorkspaceImageControls(image, resolver)
		if !ok {
			continue
		}
		inputs := unknownWorkspaceInputs()
		if cfg.ImageReferenceUploads {
			controls.SupportsReferenceImage = image.SupportsReferenceImage
			controls.MaxReferenceImages = image.MaxReferenceImages
		}
		inputs.Images.MaxCount = image.MaxReferenceImages
		add(image.ID, image.Name, "Создание изображений по текстовому описанию.", WorkspaceOperation{ID: "generate", Kind: "image", Enabled: true, Inputs: inputs, Image: &controls}, false)
	}
	for _, model := range cfg.TextModels {
		if model.ID != providermodels.PublicTextChatGPT && (cfg.Pricing == nil || model.EstimateCredits <= 0) {
			continue
		}
		text := WorkspaceText{EstimateCredits: model.EstimateCredits, MaxPromptBytes: model.MaxPromptBytes, MaxOutputTokens: model.MaxOutputTokens}
		add(model.ID, model.Name, "Ответы на вопросы и работа с текстом в текущем диалоге", WorkspaceOperation{ID: "reply", Kind: "text", Enabled: true, Inputs: unknownWorkspaceInputs(), Text: &text}, model.EstimateCredits == 0)
	}
	for _, route := range cfg.VideoRoutes {
		controls, ok := WorkspaceVideoControls(route, cfg.Pricing)
		if !ok {
			continue
		}
		add(route.Alias, route.Name, "Создание видео по текстовому описанию. Разрешения: "+strings.Join(controls.AllowedResolutions, ", ")+".", WorkspaceOperation{ID: "generate", Kind: "video", Enabled: true, Inputs: unknownWorkspaceInputs(), Video: &controls}, false)
	}
	if seen[providermodels.PublicTextChatGPT] {
		out.DefaultModelID = providermodels.PublicTextChatGPT
	} else if len(out.Items) > 0 {
		out.DefaultModelID = out.Items[0].ID
	}
	if cfg.IncludePendingMedia {
		out.Items = append(out.Items, pendingMediaWorkspaceModels()...)
	}
	return out
}

func unknownWorkspaceInputs() modelcontract.Inputs {
	u := modelcontract.Input{Support: modelcontract.Unknown}
	return modelcontract.Inputs{Images: u, Video: u, Audio: u, Documents: u}
}

// Editorial categories cannot override the authoritative price/free state.
func workspacePriceCategories(categories []string, free bool) []string {
	out := make([]string, 0, len(categories)+1)
	for _, category := range categories {
		if category != "free" {
			out = append(out, category)
		}
	}
	if free {
		out = append(out, "free")
	}
	return out
}

func WorkspaceImageControls(model imagegeneration.PublicModel, resolver imagegeneration.Resolver) (WorkspaceImage, bool) {
	out := WorkspaceImage{QualityLabel: "Разрешение", ShowOutputCount: true, PriceByQuality: map[string]int64{}, PriceByVariant: map[string]int64{}, MaxOutputCount: max(model.MaxOutputCount, 1)}
	if model.ID == providermodels.PublicImageMidjourneyV7 {
		out.QualityLabel, out.ShowOutputCount = "Режим", false
	}
	if pricingcatalog.IsGPTImage25(model.ID) {
		out.MaxPromptBytes = domain.GPTImage25MaxPromptBytes
	}
	if !model.Enabled || !model.Ready || strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.Name) == "" {
		return out, false
	}
	ratios := model.AllowedAspectRatios
	if len(ratios) == 0 {
		ratios = imagegeneration.SupportedAspectRatios()
	}
	for _, quality := range model.QualityOptions {
		for _, ratio := range ratios {
			r, err := resolver.Resolve(imagegeneration.Request{ModelID: model.ID, Quality: quality, AspectRatio: ratio})
			if err != nil || r.PricingSnapshot.InternalCredits <= 0 {
				continue
			}
			q := r.Public.ImageQuality
			if !slices.Contains(out.QualityOptions, q) {
				out.QualityOptions = append(out.QualityOptions, q)
			}
			if !slices.Contains(out.AllowedAspectRatios, ratio) {
				out.AllowedAspectRatios = append(out.AllowedAspectRatios, ratio)
			}
			out.PriceByVariant[q+":"+ratio] = r.PricingSnapshot.InternalCredits
			if out.PriceByQuality[q] == 0 || r.PricingSnapshot.InternalCredits < out.PriceByQuality[q] {
				out.PriceByQuality[q] = r.PricingSnapshot.InternalCredits
			}
		}
	}
	if len(out.QualityOptions) == 0 {
		return out, false
	}
	out.DefaultQuality = out.QualityOptions[0]
	if slices.Contains(out.QualityOptions, model.DefaultQuality) {
		out.DefaultQuality = model.DefaultQuality
	}
	out.DefaultAspectRatio = workspaceImageDefaultAspect(out)
	return out, true
}

func workspaceImageDefaultAspect(controls WorkspaceImage) string {
	if controls.PriceByVariant[controls.DefaultQuality+":"+imagegeneration.DefaultAspectRatio] > 0 {
		return imagegeneration.DefaultAspectRatio
	}
	for _, ratio := range controls.AllowedAspectRatios {
		if controls.PriceByVariant[controls.DefaultQuality+":"+ratio] > 0 {
			return ratio
		}
	}
	return ""
}

func WorkspaceVideoControls(route VideoRoute, prices imagegeneration.SnapshotCatalog) (WorkspaceVideo, bool) {
	out := WorkspaceVideo{PriceByOption: map[string]int64{}, StartImage: "unsupported", EndImage: "unsupported"}
	if prices == nil || !route.Enabled || route.RequiresStartImage || route.RequiresReferenceVideo || route.AutomaticDuration || len(route.AllowedReferenceImageCounts) > 0 && !slices.Contains(route.AllowedReferenceImageCounts, 0) {
		return out, false
	}
	registered, known := providermodels.StaticRegistry().VideoRoute(domain.VideoRouteAlias(route.Alias))
	for _, resolution := range route.AllowedResolutions {
		if known && !slices.Contains(registered.Spec.AllowedResolutions, resolution) {
			continue
		}
		for _, duration := range route.AllowedDurationsSec {
			if known && !workspaceVideoDurationAllowed(registered.Spec, resolution, duration) {
				continue
			}
			snapshot, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(route.Alias), Resolution: resolution, DurationSec: duration})
			if err != nil || !snapshot.Valid() {
				continue
			}
			variantCount := len(out.Variants)
			for _, ratio := range route.AllowedAspectRatios {
				if known && !slices.Contains(registered.Spec.AllowedAspectRatios, ratio) {
					continue
				}
				out.Variants = append(out.Variants, WorkspaceVideoVariant{Resolution: resolution, DurationSec: duration, AspectRatio: ratio})
				if !slices.Contains(out.AllowedAspectRatios, ratio) {
					out.AllowedAspectRatios = append(out.AllowedAspectRatios, ratio)
				}
			}
			if len(out.Variants) == variantCount {
				continue
			}
			if !slices.Contains(out.AllowedResolutions, resolution) {
				out.AllowedResolutions = append(out.AllowedResolutions, resolution)
			}
			if !slices.Contains(out.AllowedDurationsSec, duration) {
				out.AllowedDurationsSec = append(out.AllowedDurationsSec, duration)
			}
			out.PriceByOption[resolution+":"+strconv.Itoa(duration)] = snapshot.InternalCredits
		}
	}
	if len(out.Variants) == 0 {
		return out, false
	}
	def := out.Variants[0]
	for _, v := range out.Variants {
		if v.Resolution == route.DefaultResolution && v.DurationSec == route.DefaultDurationSec && v.AspectRatio == route.DefaultAspectRatio {
			def = v
			break
		}
	}
	out.DefaultResolution, out.DefaultDurationSec, out.DefaultAspectRatio = def.Resolution, def.DurationSec, def.AspectRatio
	return out, true
}

func workspaceVideoDurationAllowed(spec domain.VideoRouteSpec, resolution string, duration int) bool {
	if !slices.Contains(spec.AllowedDurationsSec, duration) {
		return false
	}
	for key, durations := range spec.ResolutionDurationsSec {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(resolution)) {
			return slices.Contains(durations, duration)
		}
	}
	return true
}
