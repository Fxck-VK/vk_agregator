// Package pricingcatalog defines backend-owned generation pricing contracts.
//
// The package is intentionally independent from website/VK bot handlers and
// provider adapters. Frontends select public product dimensions only; provider
// floors, multipliers and calculated credits stay backend-owned.
package pricingcatalog

import (
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/economycatalog"
)

const (
	// StaticSource is the initial code-backed catalog source. Runtime DB-backed
	// sources can layer over it in a later PR without changing public DTOs.
	StaticSource = "static"
	// RuntimeDBSource is the runtime DB-backed generation pricing source.
	RuntimeDBSource = "runtime_db"

	// DefaultMultiplier is the product markup for the initial static catalog.
	// It is represented as a rational value to avoid float math for money or
	// provider floors.
	DefaultMultiplierNumerator   int64 = 3
	DefaultMultiplierDenominator int64 = 1

	// MinorUnitsPerCredit is the fixed scale for credit-like floor units.
	// One provider/internal credit is represented as 1_000_000 micros.
	MinorUnitsPerCredit int64 = 1_000_000
)

var (
	ErrInvalidProductKey       = errors.New("pricing catalog product key invalid")
	ErrInvalidFloor            = errors.New("pricing catalog floor invalid")
	ErrInvalidMultiplier       = errors.New("pricing catalog multiplier invalid")
	ErrInvalidCap              = errors.New("pricing catalog cap invalid")
	ErrInvalidSnapshot         = errors.New("pricing catalog snapshot invalid")
	ErrPriceNotFound           = errors.New("pricing catalog price not found")
	ErrDuplicatePrice          = errors.New("pricing catalog duplicate product price")
	ErrCreditCalculation       = errors.New("pricing catalog credit calculation invalid")
	ErrInvalidProductPrice     = errors.New("pricing catalog product price invalid")
	ErrInvalidRuntimePrice     = errors.New("pricing catalog runtime price invalid")
	ErrRuntimePricingOff       = errors.New("pricing catalog runtime pricing disabled")
	ErrRuntimePricingNotLoaded = errors.New("pricing catalog runtime pricing not loaded")
	ErrNoActiveVersion         = errors.New("pricing catalog active runtime price version not found")
	ErrActiveVersionOverlap    = errors.New("pricing catalog active runtime price version overlap")
)

// FloorUnit names an exact integer minor unit for a provider floor. Units are
// provider-specific when provider credits are not interchangeable.
type FloorUnit string

const (
	FloorUnitUSDMicros       FloorUnit = "usd_micros"
	FloorUnitPoYoCredits     FloorUnit = "poyo_credit_micros"    // #nosec G101 -- pricing unit identifier, not a credential.
	FloorUnitAPIMartCredits  FloorUnit = "apimart_credit_micros" // #nosec G101 -- pricing unit identifier, not a credential.
	FloorUnitRunwayCredits   FloorUnit = "runway_credit_micros"
	FloorUnitInternalCredits FloorUnit = "internal_credit_micros"
)

// Valid reports whether the unit is one of the known exact minor units.
func (u FloorUnit) Valid() bool {
	switch u {
	case FloorUnitUSDMicros,
		FloorUnitPoYoCredits,
		FloorUnitAPIMartCredits,
		FloorUnitRunwayCredits,
		FloorUnitInternalCredits:
		return true
	default:
		return false
	}
}

// ProductKey is the public product variant key used for user-facing price
// lookup. It intentionally has no provider, provider model id, floor or
// multiplier fields.
type ProductKey struct {
	AudioModelID    string                 `json:"audio_model_id,omitempty"`
	AudioAction     string                 `json:"audio_action,omitempty"`
	AudioMax        bool                   `json:"audio_max,omitempty"`
	TextModelID     string                 `json:"text_model_id,omitempty"`
	Operation       domain.OperationType   `json:"operation"`
	Modality        domain.Modality        `json:"modality"`
	ImageModelID    string                 `json:"image_model_id,omitempty"`
	VideoRouteAlias domain.VideoRouteAlias `json:"video_route_alias,omitempty"`
	Quality         string                 `json:"quality,omitempty"`
	Resolution      string                 `json:"resolution,omitempty"`
	DurationSec     int                    `json:"duration_sec,omitempty"`
}

// Normalize returns a trimmed copy suitable for stable lookup and snapshots.
func (k ProductKey) Normalize() ProductKey {
	k.AudioModelID = strings.TrimSpace(k.AudioModelID)
	k.AudioAction = strings.TrimSpace(k.AudioAction)
	k.TextModelID = strings.TrimSpace(k.TextModelID)
	k.ImageModelID = strings.TrimSpace(k.ImageModelID)
	k.VideoRouteAlias = domain.VideoRouteAlias(strings.TrimSpace(string(k.VideoRouteAlias)))
	k.Quality = strings.TrimSpace(k.Quality)
	k.Resolution = strings.TrimSpace(k.Resolution)
	return k
}

// Valid reports whether the key has the minimum public dimensions for the
// operation. It does not prove a price exists; lookup does that in a later PR.
func (k ProductKey) Valid() bool {
	k = k.Normalize()
	if !k.Operation.Valid() || !k.Modality.Valid() {
		return false
	}
	if k.Operation == domain.OperationAudioTTS || k.Operation == domain.OperationAudioSTT {
		pair := k.Operation == domain.OperationAudioTTS && k.Modality == domain.ModalityAudio && k.AudioAction == "speak" || k.Operation == domain.OperationAudioSTT && k.Modality == domain.ModalityText && k.AudioAction == "transcribe"
		duration := k.DurationSec == 0 || k.Operation == domain.OperationAudioSTT && k.AudioModelID == "whisper_1" && k.DurationSec >= 60 && k.DurationSec <= domain.WhisperMaxInputDurationSec && k.DurationSec%60 == 0
		return pair && duration && k.AudioModelID != "" && !k.AudioMax && k.TextModelID == "" && k.ImageModelID == "" && k.VideoRouteAlias == "" && k.Quality == "" && k.Resolution == ""
	}
	if k.Operation == domain.OperationAudioMusic {
		return k.Modality == domain.ModalityAudio && k.AudioModelID != "" && k.AudioAction != "" && k.TextModelID == "" && k.ImageModelID == "" && k.VideoRouteAlias == "" && k.Quality == "" && k.Resolution == "" && k.DurationSec == 0
	}
	if k.AudioModelID != "" || k.AudioAction != "" || k.AudioMax {
		return false
	}
	switch k.Operation {
	case domain.OperationTextGenerate:
		return k.Modality == domain.ModalityText && k.TextModelID != "" && k.ImageModelID == "" && k.VideoRouteAlias == "" && k.Quality == "" && k.Resolution == "" && k.DurationSec == 0
	case domain.OperationImageGenerate, domain.OperationImageEdit, domain.OperationImageUpscale:
		return k.Modality == domain.ModalityImage && k.TextModelID == "" &&
			k.ImageModelID != "" &&
			k.VideoRouteAlias == "" &&
			k.DurationSec == 0
	case domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return k.Modality == domain.ModalityVideo && k.TextModelID == "" &&
			k.VideoRouteAlias != "" &&
			k.ImageModelID == "" &&
			k.DurationSec > 0
	default:
		return false
	}
}

// PriceFloor is the provider-side exact floor represented in integer minor
// units. Amount must be positive; Unit carries the provider/currency scale.
type PriceFloor struct {
	Amount int64     `json:"amount"`
	Unit   FloorUnit `json:"unit"`
}

// Valid reports whether the floor can safely participate in integer math.
func (f PriceFloor) Valid() bool {
	return f.Amount > 0 && f.Unit.Valid()
}

// Multiplier is an exact rational product multiplier.
type Multiplier struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

// DefaultMultiplier returns the initial x3 multiplier without using floats.
func DefaultMultiplier() Multiplier {
	return Multiplier{
		Numerator:   DefaultMultiplierNumerator,
		Denominator: DefaultMultiplierDenominator,
	}
}

// Valid reports whether the multiplier can safely participate in ceiling math.
func (m Multiplier) Valid() bool {
	return m.Numerator > 0 && m.Denominator > 0
}

// UnitConversion maps one floor unit to internal credits before product markup.
// It keeps provider credit systems and money units explicit instead of treating
// all provider credits as equal.
type UnitConversion struct {
	InternalCreditUnits int64 `json:"internal_credit_units"`
	FloorUnits          int64 `json:"floor_units"`
}

// IdentityUnitConversion maps one floor unit to one internal credit.
func IdentityUnitConversion() UnitConversion {
	return UnitConversion{InternalCreditUnits: 1, FloorUnits: 1}
}

// Valid reports whether the conversion can safely participate in exact math.
func (c UnitConversion) Valid() bool {
	return c.InternalCreditUnits > 0 && c.FloorUnits > 0
}

// SafetyCaps holds backend-only bounds. InternalCreditCap is the product price
// cap in internal credits; FloorAmountCap is a spend guardrail in the same unit
// as PriceFloor.
type SafetyCaps struct {
	InternalCreditCap int64 `json:"internal_credit_cap,omitempty"`
	FloorAmountCap    int64 `json:"floor_amount_cap,omitempty"`
}

// ValidFor reports whether configured caps are non-negative and compatible
// with a positive calculated price.
func (c SafetyCaps) ValidFor(internalCredits int64, floor PriceFloor) bool {
	if c.InternalCreditCap < 0 || c.FloorAmountCap < 0 {
		return false
	}
	if c.InternalCreditCap > 0 && internalCredits > c.InternalCreditCap {
		return false
	}
	if c.FloorAmountCap > 0 && (!floor.Valid() || floor.Amount > c.FloorAmountCap) {
		return false
	}
	return true
}

// ProductPrice is a static catalog entry. Prompt 3 adds concrete tariff
// entries; this shape already owns the backend-only price math.
type ProductPrice struct {
	EconomicBasis         string         `json:"economic_basis,omitempty"`
	RetailPolicyVersion   int            `json:"retail_policy_version,omitempty"`
	Key                   ProductKey     `json:"key"`
	Version               int            `json:"version"`
	Source                string         `json:"source"`
	Floor                 PriceFloor     `json:"floor"`
	Multiplier            Multiplier     `json:"multiplier"`
	UnitConversion        UnitConversion `json:"unit_conversion"`
	Caps                  SafetyCaps     `json:"caps,omitempty"`
	Enabled               bool           `json:"enabled"`
	DefaultDisplayCredits int64          `json:"default_display_credits,omitempty"`
}

// Normalize returns a copy with stable source and product key fields.
func (p ProductPrice) Normalize() ProductPrice {
	p.Key = p.Key.Normalize()
	p.Source = strings.TrimSpace(p.Source)
	if p.Source == "" {
		p.Source = StaticSource
	}
	return p
}

// Valid reports whether the entry shape is safe. It intentionally does not
// expose provider/floor/multiplier details to frontend DTOs.
func (p ProductPrice) Valid() bool {
	_, err := p.InternalCredits()
	return err == nil
}

// InternalCredits returns the exact backend-owned charge in whole internal
// credits. It never uses float math.
func (p ProductPrice) InternalCredits() (int64, error) {
	p = p.Normalize()
	if err := validateProductPrice(p); err != nil {
		return 0, err
	}
	return CalculateInternalCredits(p.Floor, p.UnitConversion, p.Multiplier, p.Caps)
}

// DisplayEstimateCredits returns a backend-provided catalog hint. It may differ
// from the exact cost estimate used by backend estimates and reservation paths.
func (p ProductPrice) DisplayEstimateCredits() (int64, error) {
	p = p.Normalize()
	exact, err := p.InternalCredits()
	if err != nil {
		return 0, err
	}
	if p.DefaultDisplayCredits > 0 {
		return p.DefaultDisplayCredits, nil
	}
	return exact, nil
}

// Snapshot returns the immutable backend pricing facts for a product entry.
func (p ProductPrice) Snapshot() (PricingSnapshot, error) {
	p = p.Normalize()
	internalCredits, err := p.InternalCredits()
	if err != nil {
		return PricingSnapshot{}, err
	}
	snapshot := PricingSnapshot{
		EconomicBasis:             p.EconomicBasis,
		RetailPolicyVersion:       p.RetailPolicyVersion,
		Version:                   p.Version,
		CreditDenominationVersion: domain.CurrentCreditDenominationVersion,
		Source:                    p.Source,
		Key:                       p.Key,
		Floor:                     p.Floor,
		Multiplier:                p.Multiplier,
		UnitConversion:            p.UnitConversion,
		InternalCredits:           internalCredits,
		InternalCreditCap:         p.Caps.InternalCreditCap,
		FloorAmountCap:            p.Caps.FloorAmountCap,
		DefaultDisplayCredits:     p.DefaultDisplayCredits,
	}
	if p.Key.TextModelID != "" {
		snapshot.TextInputTokenCap = TextMaxInputTokens
		snapshot.TextOutputTokenCap = TextMaxOutputTokens
	}
	if IsBoundedAPIMartImage(p.Key.ImageModelID) {
		snapshot.ImageOutputCount = 1
	}
	if IsGPTImage25(p.Key.ImageModelID) {
		snapshot.ImagePromptByteCap = domain.GPTImage25MaxPromptBytes
		snapshot.ImageTextInputFloor = int64(domain.GPTImage25MaxPromptBytes+512) * 4
		snapshot.ImageAspectRatio = "1:1"
	}
	if p.RetailPolicyVersion >= economycatalog.RetailVersion && p.Key.Operation == domain.OperationAudioMusic {
		if _, unit, ok := economycatalog.MusicRetailRate(p.Key.AudioModelID, p.Key.AudioAction, p.Key.AudioMax); ok && unit == "track" {
			snapshot.MusicPaidTrackCount = 1
		}
	}
	if !snapshot.Valid() {
		return PricingSnapshot{}, ErrInvalidSnapshot
	}
	return snapshot, nil
}

// PricingSnapshot is the immutable price record to persist with new jobs once
// consumers migrate to pricingcatalog. It contains public product dimensions
// and exact backend pricing facts, but no prompt, provider payload, private URL
// or provider-native model id.
type PricingSnapshot struct {
	EconomicBasis             string         `json:"economic_basis,omitempty"`
	RetailPolicyVersion       int            `json:"retail_policy_version,omitempty"`
	MusicPaidTrackCount       int            `json:"music_paid_track_count,omitempty"`
	ImageTextInputFloor       int64          `json:"image_text_input_floor,omitempty"`
	ImageOutputCount          int            `json:"image_output_count,omitempty"`
	ImagePromptByteCap        int            `json:"image_prompt_byte_cap,omitempty"`
	ImageAspectRatio          string         `json:"image_aspect_ratio,omitempty"`
	ImageReferenceCount       int            `json:"image_reference_count,omitempty"`
	TextInputTokenCap         int            `json:"text_input_token_cap,omitempty"`
	TextOutputTokenCap        int            `json:"text_output_token_cap,omitempty"`
	Version                   int            `json:"version"`
	CreditDenominationVersion int            `json:"credit_denomination_version"`
	Source                    string         `json:"source"`
	Key                       ProductKey     `json:"key"`
	Floor                     PriceFloor     `json:"floor"`
	Multiplier                Multiplier     `json:"multiplier"`
	UnitConversion            UnitConversion `json:"unit_conversion"`
	InternalCredits           int64          `json:"internal_credits"`
	InternalCreditCap         int64          `json:"internal_credit_cap,omitempty"`
	FloorAmountCap            int64          `json:"floor_amount_cap,omitempty"`
	DefaultDisplayCredits     int64          `json:"default_display_credits,omitempty"`
	CalculationDescription    string         `json:"calculation_description,omitempty"`
}

// Valid reports whether a snapshot has the minimum data needed to keep an old
// job price stable after catalog changes.
func (s PricingSnapshot) Valid() bool {
	switch s.EconomicBasis {
	case "", "study24", "cost_plus", "protected_reference", "operator_fixed":
	default:
		return false
	}
	// This is the first snapshot schema with a minimum paid-track contract,
	// not the current retail-table version. Later repricing must not invalidate
	// previously reserved Jobs or consult the current list of retail rules.
	const paidTrackSnapshotVersion = 20261028
	if s.MusicPaidTrackCount != 0 && (s.MusicPaidTrackCount != 1 || s.Key.Operation != domain.OperationAudioMusic || s.Key.AudioModelID != "suno_v6" || s.Key.AudioMax || s.RetailPolicyVersion < paidTrackSnapshotVersion) {
		return false
	}
	if s.MusicPaidTrackCount != 0 {
		switch s.Key.AudioAction {
		case "generate", "cover", "upload_cover", "mashup", "extend", "upload_extend", "add_vocals", "add_instrumental", "inspo":
		default:
			return false
		}
	}
	if IsBoundedAPIMartImage(s.Key.ImageModelID) && (s.ImageOutputCount < 1 || s.ImageOutputCount > 15) {
		return false
	}
	if IsGPTImage25(s.Key.ImageModelID) && (s.ImagePromptByteCap <= 0 || s.ImagePromptByteCap > domain.GPTImage25MaxPromptBytes || s.ImageTextInputFloor <= 0 || s.ImageTextInputFloor >= s.Floor.Amount) {
		return false
	}
	if s.Key.TextModelID != "" && (s.TextInputTokenCap <= 512 || s.TextInputTokenCap > TextMaxInputTokens || s.TextOutputTokenCap <= 0 || s.TextOutputTokenCap > TextMaxOutputTokens) {
		return false
	}
	s.Source = strings.TrimSpace(s.Source)
	return s.Version > 0 && s.RetailPolicyVersion >= 0 &&
		(s.CreditDenominationVersion == 0 ||
			s.CreditDenominationVersion == domain.LegacyCreditDenominationVersion ||
			s.CreditDenominationVersion == domain.CurrentCreditDenominationVersion) &&
		s.Source != "" &&
		s.Key.Valid() &&
		s.Floor.Valid() &&
		s.Multiplier.Valid() &&
		s.UnitConversion.Valid() &&
		s.InternalCredits > 0 &&
		SafetyCaps{
			InternalCreditCap: s.InternalCreditCap,
			FloorAmountCap:    s.FloorAmountCap,
		}.ValidFor(s.InternalCredits, s.Floor)
}

// Catalog is a pure in-memory pricing lookup table. Runtime consumers are wired
// to it in a later PR; this package does not call providers or trust frontend
// price input.
type Catalog struct {
	mu           sync.RWMutex
	prices       map[ProductKey]ProductPrice
	supplemental map[ProductKey]ProductPrice
}

// AddSupplemental installs server-owned candidate prices without overriding any
// runtime price (including a disabled one). A DB refresh preserves this overlay.
func (c *Catalog) AddSupplemental(prices []ProductPrice) error {
	if c == nil {
		return ErrInvalidRuntimePrice
	}
	validated, err := NewCatalog(prices)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.supplemental = validated.prices
	return nil
}

// NewCatalog validates and indexes backend-owned price entries by public
// product key.
func NewCatalog(prices []ProductPrice) (*Catalog, error) {
	index := make(map[ProductKey]ProductPrice, len(prices))
	for _, price := range prices {
		price = price.Normalize()
		if err := validateProductPrice(price); err != nil {
			return nil, err
		}
		if _, err := price.InternalCredits(); err != nil {
			return nil, err
		}
		// Apply only to already-valid existing rows. Runtime DB reloads use this
		// same boundary, so a stale DB multiplier cannot silently undo policy.
		price = applyEconomicRetailPolicy(price)
		if _, err := price.InternalCredits(); err != nil {
			return nil, err
		}
		if _, exists := index[price.Key]; exists {
			return nil, fmt.Errorf("%w: %+v", ErrDuplicatePrice, price.Key)
		}
		index[price.Key] = price
	}
	return &Catalog{prices: index}, nil
}

// Lookup returns the backend-owned price entry for a public product key.
func (c *Catalog) Lookup(key ProductKey) (ProductPrice, error) {
	key = key.Normalize()
	if !key.Valid() {
		return ProductPrice{}, ErrInvalidProductKey
	}
	if c == nil {
		return ProductPrice{}, fmt.Errorf("%w: %+v", ErrPriceNotFound, key)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.prices == nil {
		return ProductPrice{}, fmt.Errorf("%w: %+v", ErrPriceNotFound, key)
	}
	price, ok := c.prices[key]
	if !ok {
		price, ok = c.supplemental[key]
	}
	if !ok {
		return ProductPrice{}, fmt.Errorf("%w: %+v", ErrPriceNotFound, key)
	}
	if err := validateProductPrice(price); err != nil {
		return ProductPrice{}, err
	}
	return price, nil
}

// Prices returns a stable copy of active product prices for backend read-only
// visibility. Callers must still sanitize backend-only fields before exposing
// data outside trusted operator APIs.
func (c *Catalog) Prices() []ProductPrice {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	prices := make([]ProductPrice, 0, len(c.prices))
	for _, price := range c.prices {
		prices = append(prices, price)
	}
	for key, price := range c.supplemental {
		if _, exists := c.prices[key]; !exists {
			prices = append(prices, price)
		}
	}
	c.mu.RUnlock()
	sort.Slice(prices, func(i, j int) bool {
		return productKeySortKey(prices[i].Key) < productKeySortKey(prices[j].Key)
	})
	return prices
}

// CostEstimateCredits returns the exact backend-owned charge for a public
// product key. This is the value future backend estimates and reservation paths
// should use.
func (c *Catalog) CostEstimateCredits(key ProductKey) (int64, error) {
	price, err := c.Lookup(key)
	if err != nil {
		return 0, err
	}
	return price.InternalCredits()
}

// DisplayEstimateCredits returns the catalog display hint for a public product
// key. It is intentionally separate from exact cost estimates.
func (c *Catalog) DisplayEstimateCredits(key ProductKey) (int64, error) {
	price, err := c.Lookup(key)
	if err != nil {
		return 0, err
	}
	return price.DisplayEstimateCredits()
}

// Snapshot returns a stable immutable pricing snapshot for a public product key.
func (c *Catalog) Snapshot(key ProductKey) (PricingSnapshot, error) {
	price, err := c.Lookup(key)
	if err != nil {
		return PricingSnapshot{}, err
	}
	return price.Snapshot()
}

// ReplaceWith atomically swaps catalog contents while preserving the Catalog
// pointer held by runtime consumers.
func (c *Catalog) ReplaceWith(next *Catalog) error {
	if c == nil || next == nil {
		return fmt.Errorf("%w: catalog missing", ErrInvalidRuntimePrice)
	}
	next.mu.RLock()
	prices := make(map[ProductKey]ProductPrice, len(next.prices))
	for key, price := range next.prices {
		prices[key] = price
	}
	next.mu.RUnlock()
	if len(prices) == 0 {
		return fmt.Errorf("%w: empty catalog", ErrInvalidRuntimePrice)
	}
	c.mu.Lock()
	c.prices = prices
	c.mu.Unlock()
	return nil
}

func productKeySortKey(key ProductKey) string {
	key = key.Normalize()
	return strings.Join([]string{
		string(key.Operation),
		string(key.Modality),
		key.AudioModelID,
		key.AudioAction,
		fmt.Sprint(key.AudioMax),
		key.TextModelID,
		key.ImageModelID,
		string(key.VideoRouteAlias),
		key.Quality,
		key.Resolution,
		fmt.Sprintf("%010d", key.DurationSec),
	}, "\x00")
}

// CalculateInternalCredits converts an exact floor and multiplier into whole
// internal credits using ceiling math. Caps are guardrails: exceeding them fails
// closed instead of silently clipping the user-facing price.
func CalculateInternalCredits(floor PriceFloor, conversion UnitConversion, multiplier Multiplier, caps SafetyCaps) (int64, error) {
	if !floor.Valid() {
		return 0, ErrInvalidFloor
	}
	if !conversion.Valid() {
		return 0, ErrCreditCalculation
	}
	if !multiplier.Valid() {
		return 0, ErrInvalidMultiplier
	}
	if caps.InternalCreditCap < 0 || caps.FloorAmountCap < 0 {
		return 0, ErrInvalidCap
	}
	if caps.FloorAmountCap > 0 && floor.Amount > caps.FloorAmountCap {
		return 0, fmt.Errorf("%w: floor amount %d exceeds cap %d", ErrInvalidCap, floor.Amount, caps.FloorAmountCap)
	}
	credits, err := ceilScaledProduct(floor.Amount, conversion, multiplier)
	if err != nil {
		return 0, err
	}
	if !caps.ValidFor(credits, floor) {
		return 0, fmt.Errorf("%w: internal credits %d exceed cap %d", ErrInvalidCap, credits, caps.InternalCreditCap)
	}
	return credits, nil
}

func validateProductPrice(p ProductPrice) error {
	if !p.Enabled {
		return fmt.Errorf("%w: disabled", ErrInvalidProductPrice)
	}
	if p.Version <= 0 || p.RetailPolicyVersion < 0 {
		return fmt.Errorf("%w: version %d", ErrInvalidProductPrice, p.Version)
	}
	if p.Source == "" {
		return fmt.Errorf("%w: source empty", ErrInvalidProductPrice)
	}
	if !p.Key.Valid() {
		return ErrInvalidProductKey
	}
	if !p.Floor.Valid() {
		return ErrInvalidFloor
	}
	if !p.Multiplier.Valid() {
		return ErrInvalidMultiplier
	}
	if !p.UnitConversion.Valid() {
		return ErrCreditCalculation
	}
	if p.DefaultDisplayCredits < 0 {
		return fmt.Errorf("%w: display credits %d", ErrInvalidCap, p.DefaultDisplayCredits)
	}
	if p.Caps.InternalCreditCap < 0 || p.Caps.FloorAmountCap < 0 {
		return ErrInvalidCap
	}
	return nil
}

// applyEconomicRetailPolicy changes new retail quotes only. It preserves the
// runtime source/version, provider floor/conversion and provider-spend cap.
// The internal-credit cap is the exact retail quote limit; its new value is
// persisted with the policy version. Historical snapshots are never rewritten.
func applyEconomicRetailPolicy(price ProductPrice) ProductPrice {
	retail, basis, managed, err := completeRetail(price.Key, price.Floor, price.UnitConversion, 1)
	if !managed {
		return price
	}
	if err != nil {
		price.Multiplier = Multiplier{}
		return price
	}
	price.RetailPolicyVersion = economycatalog.EconomyVersion
	price.EconomicBasis = basis
	if basis == "operator_fixed" {
		return price
	}
	// Explicit runtime operator tariffs remain authoritative when they cover
	// cost. They are an audited override, not an unlabelled legacy fallback.
	if price.Source == RuntimeDBSource && basis == "cost_plus" {
		if existing, err := price.InternalCredits(); err == nil && ReferenceIncomeCoversFloor(existing, price.Floor, price.UnitConversion) {
			price.EconomicBasis = "operator_fixed"
			return price
		}
	}
	price.Multiplier = exactRetailMultiplier(price.Floor.Amount, price.UnitConversion, retail)
	price.Caps.InternalCreditCap = retail
	price.DefaultDisplayCredits = retail
	return price
}

func economicRetailAmount(key ProductKey) (int64, bool) {
	switch {
	case key.Operation == domain.OperationImageGenerate && key.Modality == domain.ModalityImage && key.Resolution == "":
		return economycatalog.ImageRetailRate(key.ImageModelID, key.Quality)
	case key.Operation == domain.OperationVideoGenerate && key.Modality == domain.ModalityVideo:
		return economycatalog.VideoRetailRate(string(key.VideoRouteAlias), key.Quality, key.Resolution, key.DurationSec)
	case key.Operation == domain.OperationAudioMusic && key.Modality == domain.ModalityAudio:
		amount, _, ok := economycatalog.MusicRetailRate(key.AudioModelID, key.AudioAction, key.AudioMax)
		return amount, ok
	default:
		return 0, false
	}
}

// Called only while constructing a new parameter-aware quote. Never apply to
// a persisted Job: its original amount/version remain the capture authority.
func applyEconomicRetailSnapshot(s PricingSnapshot) PricingSnapshot {
	retail, basis, managed, err := completeRetail(s.Key, s.Floor, s.UnitConversion, max(1, s.ImageOutputCount))
	if !managed {
		return s
	}
	if err != nil {
		s.Multiplier = Multiplier{}
		return s
	}
	s.RetailPolicyVersion = economycatalog.EconomyVersion
	s.EconomicBasis = basis
	if basis == "operator_fixed" {
		return s
	}
	s.InternalCredits, s.InternalCreditCap, s.DefaultDisplayCredits = retail, retail, retail
	s.Multiplier = exactRetailMultiplier(s.Floor.Amount, s.UnitConversion, retail)
	return s
}

func ceilScaledProduct(amount int64, conversion UnitConversion, multiplier Multiplier) (int64, error) {
	if amount <= 0 {
		return 0, ErrInvalidFloor
	}
	if !conversion.Valid() {
		return 0, ErrCreditCalculation
	}
	if !multiplier.Valid() {
		return 0, ErrInvalidMultiplier
	}

	product := new(big.Int).Mul(big.NewInt(amount), big.NewInt(conversion.InternalCreditUnits))
	product.Mul(product, big.NewInt(multiplier.Numerator))
	divisor := new(big.Int).Mul(big.NewInt(conversion.FloorUnits), big.NewInt(multiplier.Denominator))
	divisor.Mul(divisor, big.NewInt(MinorUnitsPerCredit))
	if divisor.Sign() <= 0 {
		return 0, ErrInvalidMultiplier
	}

	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, divisor, remainder)
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() || quotient.Sign() <= 0 {
		return 0, ErrCreditCalculation
	}
	return quotient.Int64(), nil
}
