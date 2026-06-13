package domain

import (
	"fmt"
	"strings"
)

// ProviderIdempotencyGuarantee describes what duplicate-submit protection a
// provider/model contract can rely on. Keep values stable and bounded because
// they may be used for internal policy decisions.
type ProviderIdempotencyGuarantee string

const (
	ProviderIdempotencyNone          ProviderIdempotencyGuarantee = "none"
	ProviderIdempotencyAdapterOnly   ProviderIdempotencyGuarantee = "adapter_only"
	ProviderIdempotencyProviderSide  ProviderIdempotencyGuarantee = "provider_side"
	ProviderIdempotencyProviderExact ProviderIdempotencyGuarantee = "provider_side_exact"
)

// ProviderMediaContract is the product-level allowlist for risky media
// provider/model combinations. It constrains what the worker may submit before
// paid provider work starts; VK Bot and Mini App surfaces must not build this
// provider-native policy themselves.
type ProviderMediaContract struct {
	Provider ProviderName `json:"provider"`
	Model    string       `json:"model"`
	// ModelClass is the curated bounded label used for metrics instead of raw
	// provider-native model ids.
	ModelClass string   `json:"model_class"`
	Modality   Modality `json:"modality"`

	AllowedDurationsSec []int    `json:"allowed_durations_sec,omitempty"`
	AllowedAspectRatios []string `json:"allowed_aspect_ratios,omitempty"`
	AllowedResolutions  []string `json:"allowed_resolutions,omitempty"`

	ExpectedContainer string `json:"expected_container,omitempty"`
	ExpectedCodec     string `json:"expected_codec,omitempty"`
	ExpectedMaxBytes  int64  `json:"expected_max_bytes,omitempty"`

	DeliveryReadyOutput bool `json:"delivery_ready_output"`
	RequiresProbe       bool `json:"requires_probe"`
	RequiresTranscode   bool `json:"requires_transcode"`
	TranscodeAllowed    bool `json:"transcode_allowed"`

	SupportsProviderIdempotency  bool                         `json:"supports_provider_idempotency"`
	ProviderIdempotencyGuarantee ProviderIdempotencyGuarantee `json:"provider_idempotency_guarantee"`

	MaxProviderAttempts    int   `json:"max_provider_attempts"`
	MaxFallbackAttempts    int   `json:"max_fallback_attempts"`
	MaxProviderCostCredits int64 `json:"max_provider_cost_credits"`
}

// Validate reports product-policy mistakes before the worker can use an unsafe
// contract.
func (c ProviderMediaContract) Validate() error {
	if strings.TrimSpace(string(c.Provider)) == "" {
		return fmt.Errorf("provider media contract: provider is required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("provider media contract %s: model is required", c.Provider)
	}
	if strings.TrimSpace(c.ModelClass) == "" {
		return fmt.Errorf("provider media contract %s/%s: model_class is required", c.Provider, c.Model)
	}
	if c.Modality == "" {
		return fmt.Errorf("provider media contract %s/%s: modality is required", c.Provider, c.Model)
	}
	if c.MaxProviderAttempts <= 0 {
		return fmt.Errorf("provider media contract %s/%s: max_provider_attempts must be positive", c.Provider, c.Model)
	}
	if c.MaxFallbackAttempts < 0 {
		return fmt.Errorf("provider media contract %s/%s: max_fallback_attempts must be non-negative", c.Provider, c.Model)
	}
	if c.MaxProviderCostCredits < 0 {
		return fmt.Errorf("provider media contract %s/%s: max_provider_cost_credits must be non-negative", c.Provider, c.Model)
	}
	if c.RequiresTranscode && !c.TranscodeAllowed {
		return fmt.Errorf("provider media contract %s/%s: requires_transcode needs transcode_allowed", c.Provider, c.Model)
	}
	if !c.DeliveryReadyOutput && !c.RequiresTranscode {
		return fmt.Errorf("provider media contract %s/%s: non-delivery-ready output needs transcode", c.Provider, c.Model)
	}
	if c.DeliveryReadyOutput {
		if strings.TrimSpace(c.ExpectedContainer) == "" {
			return fmt.Errorf("provider media contract %s/%s: expected_container is required for delivery-ready output", c.Provider, c.Model)
		}
		if strings.TrimSpace(c.ExpectedCodec) == "" {
			return fmt.Errorf("provider media contract %s/%s: expected_codec is required for delivery-ready output", c.Provider, c.Model)
		}
		if c.ExpectedMaxBytes <= 0 {
			return fmt.Errorf("provider media contract %s/%s: expected_max_bytes must be positive for delivery-ready output", c.Provider, c.Model)
		}
	}
	guarantee := c.ProviderIdempotencyGuarantee
	if guarantee == "" {
		guarantee = ProviderIdempotencyNone
	}
	switch guarantee {
	case ProviderIdempotencyNone, ProviderIdempotencyAdapterOnly, ProviderIdempotencyProviderSide, ProviderIdempotencyProviderExact:
	default:
		return fmt.Errorf("provider media contract %s/%s: unsupported provider_idempotency_guarantee %q", c.Provider, c.Model, guarantee)
	}
	if !c.SupportsProviderIdempotency && guarantee != ProviderIdempotencyNone {
		return fmt.Errorf("provider media contract %s/%s: idempotency guarantee requires supports_provider_idempotency", c.Provider, c.Model)
	}
	if !c.SupportsProviderIdempotency && c.MaxProviderAttempts > 1 {
		return fmt.Errorf("provider media contract %s/%s: max_provider_attempts > 1 requires provider idempotency", c.Provider, c.Model)
	}
	if !c.SupportsProviderIdempotency && c.MaxFallbackAttempts > 0 {
		return fmt.Errorf("provider media contract %s/%s: max_fallback_attempts > 0 requires provider idempotency", c.Provider, c.Model)
	}
	return nil
}
