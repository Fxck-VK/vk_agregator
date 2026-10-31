package economycatalog

import (
	_ "embed"
	"encoding/json"
	"math/big"
	"slices"
	"strings"
)

const RetailVersion = 20261028

//go:embed retail-policy.json
var retailJSON []byte

type ImageRetailRule struct {
	SourceID  string   `json:"source_id"`
	ModelIDs  []string `json:"model_ids"`
	Qualities []string `json:"qualities"`
	Stars     int64    `json:"stars"`
	Note      string   `json:"note"`
}

type VideoRetailRule struct {
	SourceID          string            `json:"source_id"`
	ModelIDs          []string          `json:"model_ids"`
	Resolutions       []string          `json:"resolutions"`
	Qualities         []string          `json:"qualities"`
	Unit              string            `json:"unit"`
	Rate              string            `json:"rate"`
	RatesByResolution map[string]string `json:"rates_by_resolution"`
}

type MusicRetailRule struct {
	SourceOperation string   `json:"source_operation"`
	ModelIDs        []string `json:"model_ids"`
	Actions         []string `json:"actions"`
	Unit            string   `json:"unit"`
	Stars           int64    `json:"stars"`
	MaxMode         bool     `json:"max_mode"`
}

type RetailCoverageEntry struct {
	SourceID        string   `json:"source_id"`
	Name            string   `json:"name"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason"`
	LocalProductIDs []string `json:"local_product_ids"`
}

type retailPolicy struct {
	SchemaVersion  int                   `json:"schema_version"`
	Version        int                   `json:"version"`
	ResearchPath   string                `json:"research_path"`
	ResearchSHA256 string                `json:"research_sha256"`
	Images         []ImageRetailRule     `json:"image_rules"`
	Videos         []VideoRetailRule     `json:"video_rules"`
	Music          []MusicRetailRule     `json:"music_rules"`
	Coverage       []RetailCoverageEntry `json:"coverage"`
	Applications   struct {
		Photoshoot struct {
			Standard, HD int64
			MaxCount     int `json:"max_count"`
		} `json:"photoshoot"`
		Slides struct {
			Graphical, Editable  int64
			CustomStyleSurcharge int64 `json:"custom_style_surcharge"`
		} `json:"slides"`
		Seller struct {
			Base           int64
			IncludedImages int   `json:"included_images"`
			ExtraImage     int64 `json:"extra_image"`
			AllowedCounts  []int `json:"allowed_counts"`
		} `json:"seller"`
	} `json:"applications"`
}

var retail = loadRetailPolicy()

func loadRetailPolicy() retailPolicy {
	var p retailPolicy
	if json.Unmarshal(retailJSON, &p) != nil || p.SchemaVersion != 1 || p.Version != RetailVersion || len(p.Coverage) != 94 || len(p.ResearchSHA256) != 64 {
		panic("invalid retail policy metadata")
	}
	seen := map[string]bool{}
	for _, row := range p.Coverage {
		if row.SourceID == "" || seen[row.SourceID] || row.Reason == "" {
			panic("invalid retail coverage")
		}
		seen[row.SourceID] = true
	}
	for _, r := range p.Images {
		if r.Stars <= 0 || len(r.ModelIDs) == 0 || len(r.Qualities) == 0 || !seen[r.SourceID] {
			panic("invalid image retail rule")
		}
	}
	imageKeys := map[string]bool{}
	for _, r := range p.Images {
		for _, model := range r.ModelIDs {
			for _, quality := range r.Qualities {
				key := model + "/" + quality
				if model == "" || imageKeys[key] {
					panic("ambiguous image retail rule")
				}
				imageKeys[key] = true
			}
		}
	}
	videoKeys := map[string]bool{}
	for _, r := range p.Videos {
		if len(r.ModelIDs) == 0 || len(r.Resolutions) == 0 || len(r.Qualities) == 0 || !seen[r.SourceID] || (r.Unit != "result" && r.Unit != "second") {
			panic("invalid video retail rule")
		}
		if r.Rate != "" {
			if _, ok := ceilRetailRate(r.Rate, 1); !ok {
				panic("invalid retail rate")
			}
		}
		for _, res := range r.Resolutions {
			for _, model := range r.ModelIDs {
				for _, quality := range r.Qualities {
					key := model + "/" + quality + "/" + res
					if model == "" || videoKeys[key] {
						panic("ambiguous video retail rule")
					}
					videoKeys[key] = true
				}
			}
			rate := r.Rate
			if rate == "" {
				rate = r.RatesByResolution[res]
			}
			if _, ok := ceilRetailRate(rate, 1); !ok {
				panic("missing resolution retail rate")
			}
		}
	}
	musicKeys := map[string]bool{}
	for _, r := range p.Music {
		if r.Stars <= 0 || len(r.ModelIDs) == 0 || len(r.Actions) == 0 || (r.Unit != "track" && r.Unit != "operation") || r.MaxMode {
			panic("invalid music retail rule")
		}
		for _, model := range r.ModelIDs {
			for _, action := range r.Actions {
				key := model + "/" + action
				if model == "" || action == "" || musicKeys[key] {
					panic("ambiguous music retail rule")
				}
				musicKeys[key] = true
			}
		}
	}
	return p
}

func ImageRetailRate(model, quality string) (int64, bool) {
	for _, r := range retail.Images {
		if slices.Contains(r.ModelIDs, model) && slices.Contains(r.Qualities, quality) {
			return r.Stars, true
		}
	}
	return 0, false
}

// VideoRetailRate prices already-admitted dimensions. It neither creates keys
// nor enables media inputs, new resolutions or inferred similarly-named versions.
func VideoRetailRate(model, quality, resolution string, seconds int) (int64, bool) {
	if seconds <= 0 {
		return 0, false
	}
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	for _, r := range retail.Videos {
		if !slices.Contains(r.ModelIDs, model) || !slices.Contains(r.Qualities, quality) || !slices.Contains(r.Resolutions, resolution) {
			continue
		}
		rate := r.Rate
		if rate == "" {
			rate = r.RatesByResolution[resolution]
		}
		count := 1
		if r.Unit == "second" {
			count = seconds
		}
		return ceilRetailRate(rate, count)
	}
	return 0, false
}

func ceilRetailRate(rate string, quantity int) (int64, bool) {
	if quantity <= 0 {
		return 0, false
	}
	r, ok := new(big.Rat).SetString(rate)
	if !ok || r.Sign() <= 0 {
		return 0, false
	}
	r.Mul(r, new(big.Rat).SetInt64(int64(quantity)))
	q, rem := new(big.Int), new(big.Int)
	q.QuoRem(r.Num(), r.Denom(), rem)
	if rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() || q.Sign() <= 0 {
		return 0, false
	}
	return q.Int64(), true
}

// A track-priced request guarantees at least one playable output in a new Job.
// Additional provider alternatives remain available without extra charges.
// Batching uses independent idempotent Jobs, not a provider count parameter.
func MusicRetailRate(model, action string, maxMode bool) (stars int64, unit string, ok bool) {
	for _, r := range retail.Music {
		if !maxMode && slices.Contains(r.ModelIDs, model) && slices.Contains(r.Actions, action) {
			return r.Stars, r.Unit, true
		}
	}
	return 0, "", false
}

func RetailCoverage() []RetailCoverageEntry {
	out := append([]RetailCoverageEntry(nil), retail.Coverage...)
	for i := range out {
		out[i].LocalProductIDs = append([]string(nil), out[i].LocalProductIDs...)
	}
	return out
}

// Workflow fees stay separate from arbitrary image/music Jobs. A future
// workflow must validate its inputs/admission before using one of these quotes.
func PhotoshootPrice(model, quality string, count int) (int64, bool) {
	if count < 1 || count > retail.Applications.Photoshoot.MaxCount {
		return 0, false
	}
	if !slices.Contains([]string{"gpt_image_2_5_sunburst", "nano_banana_2_1", "qwen_image_2_1", "grok_image_2_0"}, model) {
		return 0, false
	}
	rate := retail.Applications.Photoshoot.Standard
	if quality == "hd" {
		if model != "gpt_image_2_5_sunburst" && model != "nano_banana_2_1" {
			return 0, false
		}
		rate = retail.Applications.Photoshoot.HD
	} else if quality != "standard" {
		return 0, false
	}
	return rate * int64(count), true
}

func SlidesPrice(editable, customStyle bool) int64 {
	price := retail.Applications.Slides.Graphical
	if editable {
		price = retail.Applications.Slides.Editable
	}
	if customStyle {
		price += retail.Applications.Slides.CustomStyleSurcharge
	}
	return price
}

func SellerPrice(count int) (int64, bool) {
	p := retail.Applications.Seller
	if !slices.Contains(p.AllowedCounts, count) {
		return 0, false
	}
	return p.Base + int64(max(0, count-p.IncludedImages))*p.ExtraImage, true
}
