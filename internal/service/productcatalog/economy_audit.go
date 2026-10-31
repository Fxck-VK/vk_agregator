package productcatalog

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/economycatalog"
	"vk-ai-aggregator/internal/service/imagegeneration"
	"vk-ai-aggregator/internal/service/pricingcatalog"
	"vk-ai-aggregator/internal/service/providermodels"
)

type EconomicModelAudit struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	State    string   `json:"state"`
	MinStars int64    `json:"min_stars"`
	MaxStars int64    `json:"max_stars"`
	Quotes   int      `json:"quotes"`
	Bases    []string `json:"bases"`
	Reason   string   `json:"reason"`
}

type EconomicAudit struct {
	Version       int                  `json:"version"`
	QuotesChecked int                  `json:"quotes_checked"`
	Models        []EconomicModelAudit `json:"models"`
}

// RegisteredEconomicModels includes hidden routes as well as the visible
// preview. Adding a model without a commercial decision fails the audit.
func RegisteredEconomicModels() map[string]string {
	r := providermodels.StaticRegistry()
	out := map[string]string{}
	for _, m := range r.PublicImageModels() {
		out[m.PublicID] = "image"
	}
	for _, m := range r.VideoRoutes() {
		if !m.LoadTestOnly {
			out[string(m.Alias)] = "video"
		}
	}
	for _, m := range r.TextAliasModels() {
		out[m.PublicID] = "text"
	}
	for _, m := range providermodels.MediaCandidates() {
		out[m.PublicID] = m.Kind
	}
	for _, m := range providermodels.TextCandidates() {
		out[m.PublicID] = "text"
	}
	return out
}

func AuditWorkspaceEconomy(c WorkspaceModelList) (EconomicAudit, error) {
	report := EconomicAudit{Version: economycatalog.EconomyVersion}
	prices, err := pricingcatalog.NewStaticCatalog()
	if err != nil {
		return report, err
	}
	rows := map[string]*EconomicModelAudit{}
	bases := map[string]map[string]bool{}
	for id, kind := range RegisteredEconomicModels() {
		decision, ok := economycatalog.ModelEconomicDecision(id)
		if !ok || decision.Kind != kind {
			return report, fmt.Errorf("missing economic decision: %s", id)
		}
		state := "priced"
		if decision.Mode == "sponsored" {
			state = "sponsored"
		}
		if decision.Mode == "unavailable" {
			state = "unavailable"
		}
		rows[id] = &EconomicModelAudit{ID: id, Kind: kind, State: state, Reason: decision.Reason}
		bases[id] = map[string]bool{}
	}
	check := func(id string, q pricingcatalog.PricingSnapshot) error {
		row := rows[id]
		if row == nil {
			return fmt.Errorf("unknown economic product: %s", id)
		}
		if !q.Valid() || q.InternalCredits <= 0 || q.RetailPolicyVersion != economycatalog.EconomyVersion || q.EconomicBasis == "" {
			return fmt.Errorf("invalid economic quote: %s", id)
		}
		if q.EconomicBasis != "operator_fixed" && !pricingcatalog.ReferenceIncomeCoversFloor(q.InternalCredits, q.Floor, q.UnitConversion) {
			return fmt.Errorf("quote below provider cost: %s", id)
		}
		if row.Quotes == 0 || q.InternalCredits < row.MinStars {
			row.MinStars = q.InternalCredits
		}
		if q.InternalCredits > row.MaxStars {
			row.MaxStars = q.InternalCredits
		}
		row.Quotes++
		report.QuotesChecked++
		bases[id][q.EconomicBasis] = true
		return nil
	}
	// Include all already-priced hidden routes and explicit operator products.
	for _, p := range prices.Prices() {
		id := p.Key.ImageModelID
		if id == "" {
			id = p.Key.TextModelID
		}
		if id == "" {
			id = string(p.Key.VideoRouteAlias)
		}
		if rows[id] == nil {
			continue
		}
		q, err := p.Snapshot()
		if err != nil {
			return report, err
		}
		if err = check(id, q); err != nil {
			return report, err
		}
	}
	for _, m := range c.Items {
		row := rows[m.ID]
		if row == nil {
			return report, fmt.Errorf("visible model has no economy: %s", m.ID)
		}
		for _, op := range m.Operations {
			if row.State == "unavailable" {
				if op.Enabled {
					return report, fmt.Errorf("unpriced operation admitted: %s/%s", m.ID, op.ID)
				}
				continue
			}
			if row.State == "sponsored" {
				if op.Text == nil || op.Text.EstimateCredits != 0 {
					return report, fmt.Errorf("sponsored price mismatch: %s", m.ID)
				}
				bases[m.ID]["sponsored"] = true
				continue
			}
			switch {
			case op.Music != nil:
				q, err := pricingcatalog.MusicCandidateQuote(m.ID, op.ID, false)
				if err != nil {
					return report, err
				}
				if q.InternalCredits != op.Music.EstimateCredits {
					return report, fmt.Errorf("music UI price drift: %s/%s", m.ID, op.ID)
				}
				if err = check(m.ID, q); err != nil {
					return report, err
				}
				if op.Music.SupportsMax {
					q, err = pricingcatalog.MusicCandidateQuote(m.ID, op.ID, true)
					if err != nil {
						return report, err
					}
					if q.InternalCredits != op.Music.MaxEstimateCredits {
						return report, fmt.Errorf("music Max price drift: %s/%s", m.ID, op.ID)
					}
					if err = check(m.ID, q); err != nil {
						return report, err
					}
				}
			case op.Text != nil:
				q, err := prices.Snapshot(pricingcatalog.ProductKey{Operation: domain.OperationTextGenerate, Modality: domain.ModalityText, TextModelID: m.ID})
				if err != nil {
					q, err = pricingcatalog.TextCandidateQuote(m.ID)
				}
				if err != nil {
					return report, err
				}
				if q.InternalCredits != op.Text.EstimateCredits {
					return report, fmt.Errorf("text UI price drift: %s", m.ID)
				}
				if err = check(m.ID, q); err != nil {
					return report, err
				}
			case op.Image != nil:
				registered, known := providermodels.StaticRegistry().PublicImageModel(m.ID)
				resolver := imagegeneration.NewResolver([]imagegeneration.PublicModel{{ID: m.ID, Enabled: true, Ready: true, QualityOptions: op.Image.QualityOptions, DefaultQuality: op.Image.DefaultQuality, SupportsReferenceImage: op.Image.SupportsReferenceImage, MaxReferenceImages: op.Image.MaxReferenceImages, MaxOutputCount: op.Image.MaxOutputCount, AllowedAspectRatios: op.Image.AllowedAspectRatios}}, prices)
				for key, want := range op.Image.PriceByVariant {
					quality, ratio, _ := strings.Cut(key, ":")
					if known {
						refs := []int{0, op.Image.MaxReferenceImages}
						if op.Image.MaxReferenceImages > 0 {
							refs = append(refs, 1)
						}
						slices.Sort(refs)
						refs = slices.Compact(refs)
						counts := []int{1, op.Image.MaxOutputCount}
						slices.Sort(counts)
						counts = slices.Compact(counts)
						if op.Image.PriceByCount != nil {
							counts = nil
							for count := 1; count <= op.Image.MaxOutputCount; count++ {
								counts = append(counts, count)
							}
						}
						for _, ref := range refs {
							for _, count := range counts {
								if registered.PublicID == "seedream_5_0_lite" && ref+count > 15 {
									continue
								}
								resolved, err := resolver.Resolve(imagegeneration.Request{Prompt: "Offline economic audit", ModelID: m.ID, Quality: quality, AspectRatio: ratio, ReferenceCount: ref, OutputCount: count})
								if err != nil {
									return report, fmt.Errorf("image quote %s/%s/%d/%d: %w", m.ID, key, ref, count, err)
								}
								if ref == 0 && count == 1 && resolved.PricingSnapshot.InternalCredits != want {
									return report, fmt.Errorf("image UI price drift: %s/%s", m.ID, key)
								}
								if ref == 0 && op.Image.PriceByCount != nil && op.Image.PriceByCount[key+":"+strconv.Itoa(count)] != resolved.PricingSnapshot.InternalCredits {
									return report, fmt.Errorf("image batch UI price drift: %s/%s/%d", m.ID, key, count)
								}
								if err = check(m.ID, resolved.PricingSnapshot); err != nil {
									return report, err
								}
							}
						}
					} else {
						q, err := pricingcatalog.ImageCandidateQualityQuote(m.ID, quality)
						if err == nil {
							q, err = pricingcatalog.QuoteAPIMartImage(q, ratio, 0)
						}
						if err != nil {
							return report, err
						}
						if q.InternalCredits != want {
							return report, fmt.Errorf("pending image price drift: %s/%s", m.ID, key)
						}
						if err = check(m.ID, q); err != nil {
							return report, err
						}
					}
				}
			case op.Video != nil:
				for key, want := range op.Video.PriceByOption {
					res, duration, _ := strings.Cut(key, ":")
					seconds, err := strconv.Atoi(duration)
					if err != nil {
						return report, err
					}
					q, err := pricingcatalog.EconomicVideoSnapshot(pricingcatalog.ProductKey{Operation: domain.OperationVideoGenerate, Modality: domain.ModalityVideo, VideoRouteAlias: domain.VideoRouteAlias(m.ID), Resolution: res, DurationSec: seconds})
					if err != nil {
						return report, err
					}
					if q.InternalCredits != want {
						return report, fmt.Errorf("video UI price drift: %s/%s", m.ID, key)
					}
					if err = check(m.ID, q); err != nil {
						return report, err
					}
				}
			case op.Audio != nil && m.ID == "whisper_1":
				if op.Audio.CreditsPerStartedMinute != pricingcatalog.WhisperDEVCreditsPerStartedMinute {
					return report, fmt.Errorf("speech price drift: %s", m.ID)
				}
				for sec := 60; sec <= domain.WhisperMaxInputDurationSec; sec += 60 {
					q, err := pricingcatalog.WhisperDEVQuote(sec)
					if err != nil {
						return report, err
					}
					if err = check(m.ID, q); err != nil {
						return report, err
					}
				}
			default:
				return report, fmt.Errorf("missing operation economics: %s/%s", m.ID, op.ID)
			}
		}
	}
	for id, row := range rows {
		if row.State == "priced" && row.Quotes == 0 {
			return report, fmt.Errorf("model has no quotes: %s", id)
		}
		for basis := range bases[id] {
			row.Bases = append(row.Bases, basis)
		}
		sort.Strings(row.Bases)
		report.Models = append(report.Models, *row)
	}
	sort.Slice(report.Models, func(i, j int) bool { return report.Models[i].ID < report.Models[j].ID })
	return report, nil
}
