package economycatalog

import (
	_ "embed"
	"encoding/json"
	"math/big"
)

const EconomyVersion = 2026102802

//go:embed catalog-policy.json
var completeJSON []byte

type ModelEconomy struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Mode   string `json:"mode"`
	Reason string `json:"reason"`
}

type completePolicy struct {
	SchemaVersion    int            `json:"schema_version"`
	Version          int            `json:"version"`
	ReferenceVersion int            `json:"reference_version"`
	Numerator        int64          `json:"cost_multiplier_numerator"`
	Denominator      int64          `json:"cost_multiplier_denominator"`
	Step             int64          `json:"rounding_step_stars"`
	IncomeGuard      bool           `json:"reference_income_guard"`
	Models           []ModelEconomy `json:"models"`
}

var complete = loadCompletePolicy()

func loadCompletePolicy() completePolicy {
	var p completePolicy
	if json.Unmarshal(completeJSON, &p) != nil || p.SchemaVersion != 1 || p.Version != EconomyVersion || p.ReferenceVersion != RetailVersion || p.Numerator != 3 || p.Denominator != 1 || p.Step != 5 || !p.IncomeGuard {
		panic("invalid complete economy policy")
	}
	seen := map[string]bool{}
	for _, m := range p.Models {
		if m.ID == "" || seen[m.ID] || m.Reason == "" {
			panic("invalid model economy entry")
		}
		switch m.Mode {
		case "reference_or_cost", "sponsored", "operator_minute", "unavailable":
		default:
			panic("invalid model economy mode")
		}
		seen[m.ID] = true
	}
	return p
}

func ModelEconomicDecision(id string) (ModelEconomy, bool) {
	for _, m := range complete.Models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelEconomy{}, false
}

func ModelEconomicDecisions() []ModelEconomy { return append([]ModelEconomy(nil), complete.Models...) }

// CheapestPaidStarKopecks is rational, never a float. Promotions not sold by our
// plan catalogue cannot silently authorize a cheaper effective star rate.
func CheapestPaidStarKopecks() *big.Rat {
	var cheapest *big.Rat
	for _, p := range Plans() {
		if p.Amount <= 0 || p.Stars <= 0 {
			continue
		}
		price := new(big.Rat).SetFrac(big.NewInt(p.Amount), big.NewInt(p.Stars))
		if cheapest == nil || price.Cmp(cheapest) < 0 {
			cheapest = price
		}
	}
	if cheapest == nil {
		panic("missing paid offer for economy income guard")
	}
	return cheapest
}

func TopUpIncomeAllowed(amountKopecks, stars int64) bool {
	if amountKopecks <= 0 || stars <= 0 {
		return false
	}
	return new(big.Rat).SetFrac(big.NewInt(amountKopecks), big.NewInt(stars)).Cmp(CheapestPaidStarKopecks()) >= 0
}
