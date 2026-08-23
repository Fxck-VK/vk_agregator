// Package imagegeneration resolves a public image product choice into trusted
// worker routing parameters and an exact immutable pricing snapshot.
//
// It is deliberately channel-neutral. Inbound adapters may pass only a
// server-built public model catalog plus a public model/quality/reference-count
// request; provider selection and price facts remain backend-owned.
package imagegeneration

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/modelcatalog"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

var (
	ErrPublicModelUnavailable = errors.New("image generation public model unavailable")
	ErrUnsupportedQuality     = errors.New("image generation quality unsupported")
	ErrReferenceUnsupported   = errors.New("image generation references unsupported")
	ErrReferenceLimit         = errors.New("image generation reference limit exceeded")
	ErrInvalidReferenceCount  = errors.New("image generation reference count invalid")
	ErrInvalidOutputCount     = errors.New("image generation output count invalid")
	ErrOutputCountLimit       = errors.New("image generation output count limit exceeded")
	ErrPriceUnavailable       = errors.New("image generation price unavailable")
	ErrUnsupportedAspectRatio = errors.New("image generation aspect ratio unsupported")
)

const (
	DefaultAspectRatio = "16:9"
	DefaultOutputCount = 1
)

var supportedAspectRatios = map[string]struct{}{
	"16:9": {}, "1:1": {}, "21:9": {}, "2:3": {}, "3:2": {},
	"3:4": {}, "4:3": {}, "4:5": {}, "5:4": {}, "9:16": {},
}

// PublicModel is a server-built, safe product catalog entry. The caller must
// obtain it from backend feature/readiness filtering; it intentionally has no
// provider, provider-native model code, or pricing fields.
type PublicModel struct {
	ID                     string
	Name                   string
	Enabled                bool
	Ready                  bool
	QualityOptions         []string
	DefaultQuality         string
	SupportsReferenceImage bool
	MaxReferenceImages     int
	MaxOutputCount         int
}

// Request contains only public product dimensions. It intentionally accepts no
// client-owned provider choice, model code, price, or pricing snapshot.
type Request struct {
	ModelID        string
	Quality        string
	AspectRatio    string
	ReferenceCount int
	OutputCount    int
}

// PublicSelection is the safe subset an inbound adapter may map to its public
// response DTO. It has no provider routing or pricing details.
type PublicSelection struct {
	ModelID      string `json:"model_id"`
	ModelName    string `json:"model_name"`
	ImageQuality string `json:"image_quality,omitempty"`
	AspectRatio  string `json:"aspect_ratio,omitempty"`
	OutputCount  int    `json:"output_count"`
}

// WorkerParams contains trusted execution details for an internal job payload.
// It is not a public response DTO.
type WorkerParams struct {
	ModelID      string
	ModelName    string
	Provider     domain.ProviderName
	ModelCode    string
	Size         string
	Resolution   string
	ImageQuality string
	AspectRatio  string
	OutputCount  int
}

// Resolution is the server-trusted result of resolving one public selection.
type Resolution struct {
	Public          PublicSelection
	Worker          WorkerParams
	PricingSnapshot pricingcatalog.PricingSnapshot
}

// SnapshotCatalog is the narrow backend pricing dependency. It returns an
// immutable server-owned price record for the public product key.
type SnapshotCatalog interface {
	Snapshot(pricingcatalog.ProductKey) (pricingcatalog.PricingSnapshot, error)
}

// Resolver keeps an immutable copy of the current server-built public catalog.
type Resolver struct {
	publicModels []PublicModel
	pricing      SnapshotCatalog
}

// NewResolver constructs a resolver from server-trusted public availability
// data. An empty list is intentionally unavailable: callers must pass the
// server-built ready catalog instead of falling back to private model details.
func NewResolver(publicModels []PublicModel, pricing SnapshotCatalog) Resolver {
	models := make([]PublicModel, 0, len(publicModels))
	for _, model := range publicModels {
		model.QualityOptions = append([]string(nil), model.QualityOptions...)
		models = append(models, model)
	}
	return Resolver{publicModels: models, pricing: pricing}
}

// Resolve validates the public selection, derives private worker routing, and
// obtains the exact immutable price. No client-supplied cost participates in
// this path.
func (r Resolver) Resolve(request Request) (Resolution, error) {
	trustedModel, public, err := r.resolvePublic(request)
	if err != nil {
		return Resolution{}, err
	}

	key := pricingcatalog.ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: trustedModel.ModelID,
		Quality:      public.ImageQuality,
	}.Normalize()
	if !key.Valid() || r.pricing == nil {
		return Resolution{}, ErrPriceUnavailable
	}
	snapshot, err := r.pricing.Snapshot(key)
	if err != nil || !snapshot.Valid() {
		return Resolution{}, fmt.Errorf("%w: %v", ErrPriceUnavailable, err)
	}
	snapshot, err = scalePricingSnapshot(snapshot, public.OutputCount)
	if err != nil {
		return Resolution{}, err
	}

	return Resolution{
		Public: public,
		Worker: WorkerParams{
			ModelID:      trustedModel.ModelID,
			ModelName:    trustedModel.ModelName,
			Provider:     trustedModel.Provider,
			ModelCode:    trustedModel.ModelCode,
			Size:         imageSizeForQuality(trustedModel.Provider, public.ImageQuality),
			Resolution:   public.ImageQuality,
			ImageQuality: public.ImageQuality,
			AspectRatio:  public.AspectRatio,
			OutputCount:  public.OutputCount,
		},
		PricingSnapshot: snapshot,
	}, nil
}

// ResolvePublic validates and normalizes only the public image intent. It
// deliberately does not inspect pricing, so a retry can prove that it is the
// same immutable user intent before an inbound adapter asks for a newer price.
// It returns no provider routing details or pricing information.
func (r Resolver) ResolvePublic(request Request) (PublicSelection, error) {
	_, public, err := r.resolvePublic(request)
	return public, err
}

func (r Resolver) resolvePublic(request Request) (modelcatalog.Model, PublicSelection, error) {
	if request.ReferenceCount < 0 {
		return modelcatalog.Model{}, PublicSelection{}, ErrInvalidReferenceCount
	}
	if request.OutputCount < 0 {
		return modelcatalog.Model{}, PublicSelection{}, ErrInvalidOutputCount
	}

	requestedModelID := strings.TrimSpace(request.ModelID)
	trustedModel, ok := modelcatalog.ResolvePublicModel(domain.OperationImageGenerate, requestedModelID)
	if !ok || !trustedModel.ExposeID || (requestedModelID != "" && requestedModelID != trustedModel.ModelID) {
		return modelcatalog.Model{}, PublicSelection{}, ErrPublicModelUnavailable
	}
	publicModel, ok := r.publicModelFor(trustedModel)
	if !ok {
		return modelcatalog.Model{}, PublicSelection{}, ErrPublicModelUnavailable
	}

	quality, err := normalizeQuality(publicModel, request.Quality)
	if err != nil {
		return modelcatalog.Model{}, PublicSelection{}, err
	}
	aspectRatio, err := NormalizeAspectRatio(request.AspectRatio)
	if err != nil {
		return modelcatalog.Model{}, PublicSelection{}, err
	}
	if err := validateReferenceCount(publicModel, request.ReferenceCount); err != nil {
		return modelcatalog.Model{}, PublicSelection{}, err
	}
	outputCount, err := normalizeOutputCount(publicModel, request.OutputCount)
	if err != nil {
		return modelcatalog.Model{}, PublicSelection{}, err
	}

	return trustedModel, PublicSelection{
		ModelID:      trustedModel.ModelID,
		ModelName:    trustedModel.ModelName,
		ImageQuality: quality,
		AspectRatio:  aspectRatio,
		OutputCount:  outputCount,
	}, nil
}

// NormalizeAspectRatio applies the public default and rejects values that are
// not supported by the image-generation UI and provider contract.
func NormalizeAspectRatio(raw string) (string, error) {
	ratio := strings.TrimSpace(raw)
	if ratio == "" {
		ratio = DefaultAspectRatio
	}
	if _, ok := supportedAspectRatios[ratio]; !ok {
		return "", ErrUnsupportedAspectRatio
	}
	return ratio, nil
}

func (r Resolver) publicModelFor(trusted modelcatalog.Model) (PublicModel, bool) {
	for _, model := range r.publicModels {
		if strings.TrimSpace(model.ID) != trusted.ModelID {
			continue
		}
		if !model.Enabled || !model.Ready {
			return PublicModel{}, false
		}
		model.QualityOptions = append([]string(nil), model.QualityOptions...)
		return model, true
	}
	return PublicModel{}, false
}

func normalizeQuality(publicModel PublicModel, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(publicModel.QualityOptions) == 0 {
		if raw != "" {
			return "", ErrUnsupportedQuality
		}
		return "", nil
	}
	quality := raw
	if quality == "" {
		quality = publicModel.DefaultQuality
	}
	normalized, ok := modelcatalog.NormalizeImageQuality(quality)
	if !ok || !qualityAllowed(normalized, publicModel.QualityOptions) {
		return "", ErrUnsupportedQuality
	}
	return normalized, nil
}

func qualityAllowed(quality string, options []string) bool {
	for _, option := range options {
		normalized, ok := modelcatalog.NormalizeImageQuality(option)
		if ok && normalized == quality {
			return true
		}
	}
	return false
}

func validateReferenceCount(publicModel PublicModel, count int) error {
	if count < 0 {
		return ErrInvalidReferenceCount
	}
	if count > 0 && !publicModel.SupportsReferenceImage {
		return ErrReferenceUnsupported
	}
	if publicModel.MaxReferenceImages > 0 && count > publicModel.MaxReferenceImages {
		return ErrReferenceLimit
	}
	return nil
}

func normalizeOutputCount(publicModel PublicModel, count int) (int, error) {
	if count < 0 {
		return 0, ErrInvalidOutputCount
	}
	if count == 0 {
		count = DefaultOutputCount
	}
	maxOutputCount := publicModel.MaxOutputCount
	if maxOutputCount <= 0 {
		maxOutputCount = DefaultOutputCount
	}
	if count > maxOutputCount {
		return 0, ErrOutputCountLimit
	}
	return count, nil
}

func scalePricingSnapshot(snapshot pricingcatalog.PricingSnapshot, outputCount int) (pricingcatalog.PricingSnapshot, error) {
	if outputCount <= 0 {
		return pricingcatalog.PricingSnapshot{}, ErrInvalidOutputCount
	}
	if outputCount == DefaultOutputCount {
		return snapshot, nil
	}

	count := int64(outputCount)
	var ok bool
	if snapshot.Floor.Amount, ok = checkedMultiply(snapshot.Floor.Amount, count); !ok {
		return pricingcatalog.PricingSnapshot{}, ErrPriceUnavailable
	}
	if snapshot.InternalCredits, ok = checkedMultiply(snapshot.InternalCredits, count); !ok {
		return pricingcatalog.PricingSnapshot{}, ErrPriceUnavailable
	}
	if snapshot.InternalCreditCap, ok = checkedMultiplyOptional(snapshot.InternalCreditCap, count); !ok {
		return pricingcatalog.PricingSnapshot{}, ErrPriceUnavailable
	}
	if snapshot.FloorAmountCap, ok = checkedMultiplyOptional(snapshot.FloorAmountCap, count); !ok {
		return pricingcatalog.PricingSnapshot{}, ErrPriceUnavailable
	}
	if snapshot.DefaultDisplayCredits, ok = checkedMultiplyOptional(snapshot.DefaultDisplayCredits, count); !ok {
		return pricingcatalog.PricingSnapshot{}, ErrPriceUnavailable
	}
	snapshot.CalculationDescription = strings.TrimSpace(fmt.Sprintf("%s; output_count=%d", snapshot.CalculationDescription, outputCount))
	if !snapshot.Valid() {
		return pricingcatalog.PricingSnapshot{}, ErrPriceUnavailable
	}
	return snapshot, nil
}

func checkedMultiply(value, multiplier int64) (int64, bool) {
	if value <= 0 || multiplier <= 0 || value > math.MaxInt64/multiplier {
		return 0, false
	}
	return value * multiplier, true
}

func checkedMultiplyOptional(value, multiplier int64) (int64, bool) {
	if value == 0 {
		return 0, true
	}
	return checkedMultiply(value, multiplier)
}

func imageSizeForQuality(provider domain.ProviderName, quality string) string {
	if quality == "" {
		return ""
	}
	if provider == domain.ProviderDeepInfra {
		switch quality {
		case modelcatalog.ImageQuality2K:
			return "2048x2048"
		case modelcatalog.ImageQuality4K:
			return "4096x4096"
		default:
			return "1024x1024"
		}
	}
	return "1:1"
}
