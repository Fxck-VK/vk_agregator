package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// APIMart Imagen 4.0 price checked 2026-09-20 from:
// https://api.apimart.ai/api/pricing/model?model=imagen-4.0-apimart
// Candidate quotes are deliberately not inserted into StaticProductPrices.
func ImageCandidateQuote(model string) (PricingSnapshot, error) {
	switch model {
	case "nano_banana":
		// APIMart public discounted price, checked 2026-09-28:
		// https://api.apimart.ai/api/pricing/model?model=gemini-2.5-flash-image-preview
		return candidateUSDQuote(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: model}, 12500)
	case "imagen_4_0":
		return candidateUSDQuote(ProductKey{Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage, ImageModelID: model}, 40000)
	default:
		return PricingSnapshot{}, ErrPriceNotFound
	}
}
