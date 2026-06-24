// Package pricingcatalog defines backend-owned generation pricing contracts.
//
// The package is intentionally independent from Mini App/VK bot handlers and
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
	FloorUnitPoYoCredits     FloorUnit = "poyo_credit_micros"
	FloorUnitAPIMartCredits  FloorUnit = "apimart_credit_micros"
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
	switch k.Operation {
	case domain.OperationImageGenerate, domain.OperationImageEdit, domain.OperationImageUpscale:
		return k.Modality == domain.ModalityImage &&
			k.ImageModelID != "" &&
			k.VideoRouteAlias == "" &&
			k.DurationSec == 0
	case domain.OperationVideoGenerate, domain.OperationVideoImageToVideo, domain.OperationVideoExtend:
		return k.Modality == domain.ModalityVideo &&
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
// from the exact cost estimate used by /miniapp/estimate and reservation paths.
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
		Version:               p.Version,
		Source:                p.Source,
		Key:                   p.Key,
		Floor:                 p.Floor,
		Multiplier:            p.Multiplier,
		UnitConversion:        p.UnitConversion,
		InternalCredits:       internalCredits,
		InternalCreditCap:     p.Caps.InternalCreditCap,
		FloorAmountCap:        p.Caps.FloorAmountCap,
		DefaultDisplayCredits: p.DefaultDisplayCredits,
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
	Version                int            `json:"version"`
	Source                 string         `json:"source"`
	Key                    ProductKey     `json:"key"`
	Floor                  PriceFloor     `json:"floor"`
	Multiplier             Multiplier     `json:"multiplier"`
	UnitConversion         UnitConversion `json:"unit_conversion"`
	InternalCredits        int64          `json:"internal_credits"`
	InternalCreditCap      int64          `json:"internal_credit_cap,omitempty"`
	FloorAmountCap         int64          `json:"floor_amount_cap,omitempty"`
	DefaultDisplayCredits  int64          `json:"default_display_credits,omitempty"`
	CalculationDescription string         `json:"calculation_description,omitempty"`
}

// Valid reports whether a snapshot has the minimum data needed to keep an old
// job price stable after catalog changes.
func (s PricingSnapshot) Valid() bool {
	s.Source = strings.TrimSpace(s.Source)
	return s.Version > 0 &&
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
	mu     sync.RWMutex
	prices map[ProductKey]ProductPrice
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
	c.mu.RUnlock()
	sort.Slice(prices, func(i, j int) bool {
		return productKeySortKey(prices[i].Key) < productKeySortKey(prices[j].Key)
	})
	return prices
}

// CostEstimateCredits returns the exact backend-owned charge for a public
// product key. This is the value future /miniapp/estimate and reservation paths
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
	if p.Version <= 0 {
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
