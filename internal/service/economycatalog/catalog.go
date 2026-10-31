// Package economycatalog owns the versioned public economic policy. Research
// facts and product matching never enable provider routes or establish admission.
package economycatalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"vk-ai-aggregator/internal/domain"
)

const (
	Version                  = 20261027
	StarNominalKopecks int64 = 50
	SignupBonusStars   int64 = 60
)

//go:embed policy.json
var policyJSON []byte

type Plan struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Amount       int64  `json:"amount"`
	Stars        int64  `json:"stars"`
	PeriodDays   int    `json:"period_days"`
	Trial        bool   `json:"trial"`
	NextPlanCode string `json:"next_plan_code"`
}

// Mapping identifies a local product family, not an enabled provider route.
// Only exact mappings establish a retail rate for the listed qualities/unit.
type Mapping struct {
	Status     string   `json:"status"`
	ProductIDs []string `json:"product_ids"`
	Qualities  []string `json:"qualities"`
	Unit       string   `json:"unit"`
	Reason     string   `json:"reason"`
}

type Model struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	URL              string  `json:"url"`
	SourceDate       string  `json:"source_date"`
	PricingSourceURL string  `json:"pricing_source_url"`
	CatalogSourceURL string  `json:"catalog_source_url"`
	MinimumStars     *int64  `json:"minimum_stars"`
	CatalogCardStars *int64  `json:"catalog_card_stars"`
	PriceConflict    bool    `json:"price_conflict"`
	Unit             string  `json:"unit"`
	DisplayedUnit    string  `json:"displayed_unit"`
	FreeCategory     bool    `json:"free_category"`
	Mapping          Mapping `json:"mapping"`
}

type OperationFact struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MinimumStars  int64  `json:"minimum_stars"`
	Unit          string `json:"unit"`
	URL           string `json:"url"`
	SourceDate    string `json:"source_date"`
	MappingStatus string `json:"mapping_status"`
	Reason        string `json:"reason"`
}

type Gap struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type Policy struct {
	RenewalRetries            domain.SubscriptionRetryPolicy `json:"renewal_retries"`
	SchemaVersion             int                            `json:"schema_version"`
	Version                   int                            `json:"version"`
	ResearchDate              string                         `json:"research_date"`
	CreditDenominationVersion int                            `json:"credit_denomination_version"`
	StarNominalKopecks        int64                          `json:"star_nominal_kopecks"`
	SignupBonusStars          int64                          `json:"signup_bonus_stars"`
	AmountUnit                string                         `json:"amount_unit"`
	Currency                  string                         `json:"currency"`
	Sources                   []string                       `json:"sources"`
	Verification              string                         `json:"verification"`
	Plans                     []Plan                         `json:"plans"`
	Models                    []Model                        `json:"models"`
	OperationFacts            []OperationFact                `json:"operation_facts"`
	Gaps                      []Gap                          `json:"gaps"`
}

var canonical = mustLoad()

func mustLoad() Policy {
	var policy Policy
	if err := json.Unmarshal(policyJSON, &policy); err != nil {
		panic(fmt.Sprintf("economic policy: %v", err))
	}
	if policy.SchemaVersion != 1 || policy.Version != Version || policy.StarNominalKopecks != StarNominalKopecks || policy.SignupBonusStars != SignupBonusStars || policy.CreditDenominationVersion != 2 || len(policy.Plans) != 5 || len(policy.Models) != 94 {
		panic("economic policy: invalid canonical metadata")
	}
	if !policy.RenewalRetries.Enabled() {
		panic("economic policy: invalid renewal retry policy")
	}
	return policy
}

func RenewalRetries() domain.SubscriptionRetryPolicy { return canonical.RenewalRetries }

func Plans() []Plan { return append([]Plan(nil), canonical.Plans...) }

func FindPlan(code string) (Plan, bool) {
	for _, plan := range canonical.Plans {
		if plan.Code == code {
			return plan, true
		}
	}
	return Plan{}, false
}

func Models() []Model {
	out := make([]Model, len(canonical.Models))
	for i, model := range canonical.Models {
		out[i] = cloneModel(model)
	}
	return out
}

func FindModel(id string) (Model, bool) {
	for _, model := range canonical.Models {
		if model.ID == id {
			return cloneModel(model), true
		}
	}
	return Model{}, false
}

// ImageRate is an exact per-output rate only for verified model/quality pairs.
// Count scaling belongs to the existing quote path and must happen once.
func ImageRate(publicID, quality string) (int64, bool) {
	return ImageRetailRate(publicID, quality)
}

// Current returns a defensive copy suitable for safe public serialization.
func Current() Policy {
	out := canonical
	out.Sources = append([]string(nil), canonical.Sources...)
	out.Plans, out.Models = Plans(), Models()
	out.OperationFacts = append([]OperationFact(nil), canonical.OperationFacts...)
	out.Gaps = append([]Gap(nil), canonical.Gaps...)
	return out
}

func cloneModel(model Model) Model {
	model.Mapping.ProductIDs = append([]string{}, model.Mapping.ProductIDs...)
	model.Mapping.Qualities = append([]string{}, model.Mapping.Qualities...)
	if model.MinimumStars != nil {
		value := *model.MinimumStars
		model.MinimumStars = &value
	}
	if model.CatalogCardStars != nil {
		value := *model.CatalogCardStars
		model.CatalogCardStars = &value
	}
	return model
}

// MissingOrAmbiguous returns independently actionable model gaps. It includes
// formula and variant uncertainty; these facts must never become exact quotes.
func MissingOrAmbiguous() []Model {
	var out []Model
	for _, model := range Models() {
		if model.Mapping.Status != "exact" {
			out = append(out, model)
		}
	}
	return out
}

// IsKnownMappingStatus is shared by validation without conflating availability.
func IsKnownMappingStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "exact", "variant_unverified", "formula_unverified", "version_ambiguous", "missing", "free_unverified":
		return true
	default:
		return false
	}
}
