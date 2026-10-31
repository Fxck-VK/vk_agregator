package pricingcatalog

import (
	"math/big"
	"sync"
	"vk-ai-aggregator/internal/service/economycatalog"
)

var defaultEconomicCatalog = sync.OnceValues(NewStaticCatalog)

// EconomicVideoSnapshot is the offline default for standalone video resolvers.
// Runtime wiring injects its current catalogue instead, including DB reloads.
func EconomicVideoSnapshot(key ProductKey) (PricingSnapshot, error) {
	c, err := defaultEconomicCatalog()
	if err != nil {
		return PricingSnapshot{}, err
	}
	s, err := c.Snapshot(key)
	if err == nil {
		return s, nil
	}
	return MediaVideoCandidateQuote(string(key.VideoRouteAlias), key.Quality, key.Resolution, key.DurationSec)
}

func economicModelID(key ProductKey) string {
	if key.ImageModelID != "" {
		return key.ImageModelID
	}
	if key.TextModelID != "" {
		return key.TextModelID
	}
	if key.AudioModelID != "" {
		return key.AudioModelID
	}
	return string(key.VideoRouteAlias)
}

// CostBasedRetail uses the existing declared markup, not a competitor minimum.
// The complete generation rounds once; floor and conversion stay exact.
func CostBasedRetail(floor PriceFloor, conversion UnitConversion) (int64, error) {
	credits, err := CalculateInternalCredits(floor, conversion, DefaultMultiplier(), SafetyCaps{})
	if err != nil {
		return 0, err
	}
	if credits > int64(^uint64(0)>>1)-4 {
		return 0, ErrCreditCalculation
	}
	return ((credits + 4) / 5) * 5, nil
}

// ReferenceIncomeCoversFloor compares revenue at the cheapest paid offer with
// the stored nominal cost conversion. It does not change provider spend caps,
// claim a live exchange rate or guarantee margin after external payment fees.
func ReferenceIncomeCoversFloor(retail int64, floor PriceFloor, conversion UnitConversion) bool {
	if retail <= 0 || !floor.Valid() || !conversion.Valid() {
		return false
	}
	income := new(big.Rat).Mul(new(big.Rat).SetInt64(retail), economycatalog.CheapestPaidStarKopecks())
	n := new(big.Int).Mul(big.NewInt(floor.Amount), big.NewInt(conversion.InternalCreditUnits))
	n.Mul(n, big.NewInt(economycatalog.StarNominalKopecks))
	d := new(big.Int).Mul(big.NewInt(conversion.FloorUnits), big.NewInt(MinorUnitsPerCredit))
	return income.Cmp(new(big.Rat).SetFrac(n, d)) > 0
}

func completeRetail(key ProductKey, floor PriceFloor, conversion UnitConversion, count int) (retail int64, basis string, managed bool, err error) {
	model, ok := economycatalog.ModelEconomicDecision(economicModelID(key))
	if !ok || model.Mode == "sponsored" || model.Mode == "unavailable" {
		return 0, "", false, nil
	}
	if model.Mode == "operator_minute" {
		return 0, "operator_fixed", true, nil
	}
	retail, reference := economicRetailAmount(key)
	if reference {
		if count > 1 {
			if retail > int64(^uint64(0)>>1)/int64(count) {
				return 0, "", true, ErrCreditCalculation
			}
			retail *= int64(count)
		}
		if ReferenceIncomeCoversFloor(retail, floor, conversion) {
			return retail, "study24", true, nil
		}
	}
	retail, err = CostBasedRetail(floor, conversion)
	basis = "cost_plus"
	if reference {
		basis = "protected_reference"
	}
	return retail, basis, true, err
}
