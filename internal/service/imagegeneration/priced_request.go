package imagegeneration

import (
	"strings"
	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

// ValidatePricedRequest protects persisted execution boundaries independently
// of current feature flags or current tariff values. Retries use the original
// snapshot; they never reprice a Job against today's catalog.
func ValidatePricedRequest(s pricingcatalog.PricingSnapshot, req Request, provider domain.ProviderName, modelCode, resolution string) error {
	if !pricingcatalog.IsBoundedAPIMartImage(s.Key.ImageModelID) && !pricingcatalog.IsBoundedAPIMartImage(req.ModelID) && !providermodels.IsNewAPIMartImageRoute(provider, modelCode) {
		return nil
	}
	model, ok := modelcatalog.ResolvePublicModel(domain.OperationImageGenerate, req.ModelID)
	if !ok || !pricingcatalog.IsBoundedAPIMartImage(req.ModelID) || model.Provider != provider || model.ModelCode != modelCode || !s.Valid() {
		return ErrPriceUnavailable
	}
	quality, ok := modelcatalog.NormalizeImageQuality(req.Quality)
	key := pricingcatalog.ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: req.ModelID, Quality: quality}
	count := req.OutputCount
	if count == 0 {
		count = 1
	}
	if !ok || s.Key != key || count < 1 || count > model.MaxOutputCount || s.ImageOutputCount != count || s.ImageReferenceCount != req.ReferenceCount || req.ReferenceCount < 0 || req.ReferenceCount > model.MaxReferenceImages || (req.ReferenceCount > 0 && !model.SupportsReferenceImage) || WorkerResolution(req.ModelID, quality) != resolution {
		return ErrPriceUnavailable
	}
	ratio := strings.TrimSpace(req.AspectRatio)
	if ratio == "" {
		ratio = "1:1"
	}
	allowed := false
	for _, candidate := range model.AllowedAspectRatios {
		allowed = allowed || ratio == candidate
	}
	if !allowed {
		return ErrUnsupportedAspectRatio
	}
	if req.ModelID == modelcatalog.MiniAppImageSeedream50Lite && req.ReferenceCount+count > 15 {
		return ErrOutputCountLimit
	}
	if pricingcatalog.IsGPTImage25(req.ModelID) && (s.ImageAspectRatio != ratio || len(req.Prompt) > s.ImagePromptByteCap) {
		return ErrPriceUnavailable
	}
	return nil
}
