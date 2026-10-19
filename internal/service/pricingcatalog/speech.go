package pricingcatalog

import "vk-ai-aggregator/internal/domain"

// WhisperDEVFixedPriceSource identifies the operator-approved retail policy of
// 2026-10-19. This is not a provider USD tariff or a claim about audio tokenization.
const WhisperDEVFixedPriceSource = "dev_whisper_fixed_minute_v1"
const WhisperDEVCreditsPerStartedMinute int64 = 25

func WhisperBillableDurationSec(durationMS int64) (int, error) {
	if durationMS <= 0 || durationMS > domain.WhisperMaxInputDurationSec*1000 {
		return 0, ErrInvalidProductKey
	}
	return int((durationMS+59_999)/60_000) * 60, nil
}

// WhisperDEVQuote prices a complete minute bucket in internal credits only.
// Runtime bootstrap adds these quotes exclusively to the explicit DEV overlay.
func WhisperDEVQuote(seconds int) (PricingSnapshot, error) {
	if seconds < 60 || seconds > domain.WhisperMaxInputDurationSec || seconds%60 != 0 {
		return PricingSnapshot{}, ErrInvalidProductKey
	}
	credits := WhisperDEVCreditsPerStartedMinute * int64(seconds/60)
	return (ProductPrice{
		Key:     ProductKey{Operation: domain.OperationAudioSTT, Modality: domain.ModalityText, AudioModelID: "whisper_1", AudioAction: "transcribe", DurationSec: seconds},
		Version: 1, Source: WhisperDEVFixedPriceSource, Enabled: true,
		Floor:      PriceFloor{Unit: FloorUnitInternalCredits, Amount: credits * MinorUnitsPerCredit},
		Multiplier: Multiplier{Numerator: 1, Denominator: 1}, UnitConversion: IdentityUnitConversion(),
		Caps: SafetyCaps{InternalCreditCap: credits, FloorAmountCap: credits * MinorUnitsPerCredit},
	}).Snapshot()
}
