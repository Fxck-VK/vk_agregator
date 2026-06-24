package admin

import (
	"net/http"
	"time"

	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func (h *Handler) getOperatorPricing(w http.ResponseWriter, r *http.Request) {
	if h.deps.PricingCache == nil {
		writeError(w, http.StatusServiceUnavailable, "runtime pricing unavailable")
		return
	}
	catalog, selection, err := h.deps.PricingCache.Current()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "runtime pricing unavailable")
		return
	}
	entries, err := newOperatorPricingEntries(catalog.Prices())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "runtime pricing invalid")
		return
	}
	writeJSON(w, http.StatusOK, OperatorPricingDTO{
		GeneratedAt:    time.Now().UTC(),
		Source:         selection.Source,
		Version:        selection.Version,
		StaticFallback: selection.StaticFallback,
		LoadedAt:       selection.LoadedAt,
		EffectiveFrom:  optionalTime(selection.EffectiveFrom),
		EffectiveUntil: cloneTime(selection.EffectiveUntil),
		Entries:        entries,
		EntryCount:     len(entries),
		Notes: []string{
			"Entries are read-only and derived from the runtime pricing catalog used by Mini App, VK bot and job creation.",
		},
	})
}

func newOperatorPricingEntries(prices []pricingcatalog.ProductPrice) ([]OperatorPricingEntryDTO, error) {
	out := make([]OperatorPricingEntryDTO, 0, len(prices))
	for _, price := range prices {
		price = price.Normalize()
		cost, err := price.InternalCredits()
		if err != nil {
			return nil, err
		}
		display, err := price.DisplayEstimateCredits()
		if err != nil {
			return nil, err
		}
		out = append(out, OperatorPricingEntryDTO{
			Operation:              string(price.Key.Operation),
			Modality:               string(price.Key.Modality),
			ImageModelID:           price.Key.ImageModelID,
			VideoRouteAlias:        string(price.Key.VideoRouteAlias),
			Quality:                price.Key.Quality,
			Resolution:             price.Key.Resolution,
			DurationSec:            price.Key.DurationSec,
			CostEstimateCredits:    cost,
			DisplayEstimateCredits: display,
			Enabled:                price.Enabled,
		})
	}
	return out, nil
}

func optionalTime(in time.Time) *time.Time {
	if in.IsZero() {
		return nil
	}
	return cloneTime(&in)
}

func cloneTime(in *time.Time) *time.Time {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}
