package pricingcatalog

import (
	"math/big"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/economycatalog"
)

// PresentationPreviewPrices are estimates for one bounded text request and
// optionally one 1K cover. They do not admit or authorize a provider route.
type PresentationPreviewPrices struct {
	TextOnly  int64 `json:"textOnly"`
	WithCover int64 `json:"withCover"`
}

// PresentationPreferredTextModelID also anchors the local estimate to the
// model preferred by the DEV quote. Runtime admission remains authoritative.
const PresentationPreferredTextModelID = "gemini_2_5_flash_lite"

// PresentationTextSnapshot retains the admitted model, token caps and floor.
// Its minimum retail price applies only to newly created presentation Jobs.
func PresentationTextSnapshot(snapshot PricingSnapshot) (PricingSnapshot, error) {
	if !snapshot.Valid() || snapshot.Key.Operation != domain.OperationTextGenerate || snapshot.Key.TextModelID == "" {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	minimum, err := presentationRetail(snapshot)
	if err != nil {
		return PricingSnapshot{}, err
	}
	if snapshot.InternalCredits >= minimum {
		return snapshot, nil
	}
	snapshot.InternalCredits = minimum
	snapshot.InternalCreditCap = minimum
	snapshot.DefaultDisplayCredits = minimum
	snapshot.Multiplier = exactRetailMultiplier(snapshot.Floor.Amount, snapshot.UnitConversion, minimum)
	snapshot.EconomicBasis = "cost_plus"
	if !snapshot.Valid() {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	return snapshot, nil
}

func PresentationPreviewQuote() (PresentationPreviewPrices, error) {
	text, err := TextCandidateQuote(PresentationPreferredTextModelID)
	if err != nil {
		return PresentationPreviewPrices{}, err
	}
	catalog, err := defaultEconomicCatalog()
	if err != nil {
		return PresentationPreviewPrices{}, err
	}
	cover, err := catalog.Snapshot(ProductKey{
		Operation: domain.OperationImageGenerate, Modality: domain.ModalityImage,
		ImageModelID: PublicImageGPTImage2, Quality: ImageQuality1K,
	})
	if err != nil {
		return PresentationPreviewPrices{}, err
	}
	textOnly, err := presentationRetail(text)
	if err != nil {
		return PresentationPreviewPrices{}, err
	}
	withCover, err := presentationRetail(text, cover)
	if err != nil {
		return PresentationPreviewPrices{}, err
	}
	return PresentationPreviewPrices{TextOnly: textOnly, WithCover: withCover}, nil
}

func presentationRetail(items ...PricingSnapshot) (int64, error) {
	var cost big.Rat
	for _, item := range items {
		if !item.Floor.Valid() || !item.UnitConversion.Valid() {
			return 0, ErrInvalidSnapshot
		}
		amount := new(big.Int).Mul(big.NewInt(item.Floor.Amount), big.NewInt(item.UnitConversion.InternalCreditUnits))
		divisor := new(big.Int).Mul(big.NewInt(item.UnitConversion.FloorUnits), big.NewInt(MinorUnitsPerCredit))
		cost.Add(&cost, new(big.Rat).SetFrac(amount, divisor))
	}
	if cost.Sign() <= 0 {
		return 0, ErrCreditCalculation
	}
	starPrice := economycatalog.CheapestPaidStarKopecks()
	nominal := big.NewRat(economycatalog.StarNominalKopecks, 1)
	if starPrice.Cmp(nominal) > 0 {
		starPrice = nominal
	}
	// Round the combined floor once. The cheapest paid star must bring in
	// at least three times the cost at the project's nominal conversion.
	const step = 5
	markedUp := new(big.Rat).Mul(&cost, nominal)
	markedUp.Mul(markedUp, big.NewRat(DefaultMultiplierNumerator, DefaultMultiplierDenominator))
	markedUp.Quo(markedUp, starPrice)
	markedUp.Quo(markedUp, big.NewRat(step, 1))
	steps, remainder := new(big.Int), new(big.Int)
	steps.QuoRem(markedUp.Num(), markedUp.Denom(), remainder)
	if remainder.Sign() != 0 {
		steps.Add(steps, big.NewInt(1))
	}
	price := steps.Mul(steps, big.NewInt(step))
	if !price.IsInt64() || price.Sign() <= 0 {
		return 0, ErrCreditCalculation
	}
	return price.Int64(), nil
}
