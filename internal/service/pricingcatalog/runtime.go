package pricingcatalog

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const RuntimePriceVersionStatusActive = "active"

// RuntimePricingRepository reads DB-backed generation pricing by public product
// dimensions only. Implementations must not expose provider payloads, prompts
// or private URLs.
type RuntimePricingRepository interface {
	ListActivePrices(ctx context.Context) (RuntimePriceSet, error)
	GetActivePrice(ctx context.Context, key ProductKey) (ProductPrice, error)
}

// RuntimeCatalogConfig controls runtime source selection. Static fallback is
// deliberately explicit and is ignored when DB pricing is enabled.
type RuntimeCatalogConfig struct {
	DBEnabled             bool
	StaticFallbackEnabled bool
}

// RuntimeCatalogSelection reports which source was selected.
type RuntimeCatalogSelection struct {
	Source         string
	Version        int
	StaticFallback bool
	LoadedAt       time.Time
}

// RuntimeCatalogCache keeps one runtime Catalog pointer stable for consumers
// while allowing validated DB/static reloads to replace its contents.
type RuntimeCatalogCache struct {
	repo          RuntimePricingRepository
	staticCatalog *Catalog
	config        RuntimeCatalogConfig

	reloadMu sync.Mutex
	mu       sync.RWMutex
	catalog  *Catalog
	selected RuntimeCatalogSelection
	lastErr  error
}

// RuntimeCatalogVersion is the active DB version metadata needed to build a
// runtime catalog. It contains audit-safe identifiers only.
type RuntimeCatalogVersion struct {
	ID             string
	PriceVersion   int
	Status         string
	EffectiveFrom  time.Time
	EffectiveUntil *time.Time
}

// RuntimePriceSet is one validated active DB version and its enabled prices.
type RuntimePriceSet struct {
	Version RuntimeCatalogVersion
	Prices  []ProductPrice
}

func NewRuntimeCatalogCache(repo RuntimePricingRepository, staticCatalog *Catalog, cfg RuntimeCatalogConfig) *RuntimeCatalogCache {
	return &RuntimeCatalogCache{
		repo:          repo,
		staticCatalog: staticCatalog,
		config:        cfg,
	}
}

// Load initializes the cache. It is intentionally the same strict path used by
// Reload so startup fails closed when the configured source is invalid.
func (c *RuntimeCatalogCache) Load(ctx context.Context) error {
	return c.Reload(ctx)
}

// Reload rebuilds the configured catalog. On failure it keeps any previously
// loaded catalog and returns the error to the caller.
func (c *RuntimeCatalogCache) Reload(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("%w: cache missing", ErrInvalidRuntimePrice)
	}
	c.reloadMu.Lock()
	defer c.reloadMu.Unlock()

	next, selected, err := ResolveRuntimeCatalog(ctx, c.repo, c.staticCatalog, c.config)
	if err != nil {
		c.mu.Lock()
		c.lastErr = err
		c.mu.Unlock()
		return err
	}
	selected.LoadedAt = time.Now().UTC()

	c.mu.RLock()
	current := c.catalog
	c.mu.RUnlock()
	if current != nil {
		if err := current.ReplaceWith(next); err != nil {
			c.mu.Lock()
			c.lastErr = err
			c.mu.Unlock()
			return err
		}
		next = current
	}

	c.mu.Lock()
	c.catalog = next
	c.selected = selected
	c.lastErr = nil
	c.mu.Unlock()
	return nil
}

// Current returns the stable runtime Catalog pointer after a successful load.
func (c *RuntimeCatalogCache) Current() (*Catalog, RuntimeCatalogSelection, error) {
	if c == nil {
		return nil, RuntimeCatalogSelection{}, fmt.Errorf("%w: cache missing", ErrRuntimePricingNotLoaded)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.catalog == nil {
		if c.lastErr != nil {
			return nil, c.selected, c.lastErr
		}
		return nil, c.selected, ErrRuntimePricingNotLoaded
	}
	return c.catalog, c.selected, nil
}

// StartAutoRefresh periodically reloads pricing until ctx is canceled. Failed
// refreshes never swap the current catalog.
func (c *RuntimeCatalogCache) StartAutoRefresh(ctx context.Context, interval time.Duration, onError func(error)) func() {
	if c == nil || interval <= 0 {
		return func() {}
	}
	refreshCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				if err := c.Reload(refreshCtx); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
	return cancel
}

// ResolveRuntimeCatalog builds the configured catalog. DB mode fails closed on
// any repository or validation error and never falls back to static implicitly.
func ResolveRuntimeCatalog(ctx context.Context, repo RuntimePricingRepository, staticCatalog *Catalog, cfg RuntimeCatalogConfig) (*Catalog, RuntimeCatalogSelection, error) {
	if cfg.DBEnabled {
		if repo == nil {
			return nil, RuntimeCatalogSelection{}, fmt.Errorf("%w: repository missing", ErrInvalidRuntimePrice)
		}
		set, err := repo.ListActivePrices(ctx)
		if err != nil {
			return nil, RuntimeCatalogSelection{}, err
		}
		catalog, err := set.Catalog()
		if err != nil {
			return nil, RuntimeCatalogSelection{}, err
		}
		return catalog, RuntimeCatalogSelection{
			Source:  RuntimeDBSource,
			Version: set.Version.PriceVersion,
		}, nil
	}

	if !cfg.StaticFallbackEnabled {
		return nil, RuntimeCatalogSelection{}, ErrRuntimePricingOff
	}
	if staticCatalog == nil {
		return nil, RuntimeCatalogSelection{}, fmt.Errorf("%w: static catalog missing", ErrInvalidRuntimePrice)
	}
	return staticCatalog, RuntimeCatalogSelection{
		Source:         StaticSource,
		Version:        StaticCatalogVersion,
		StaticFallback: true,
	}, nil
}

// SelectActiveRuntimeVersion returns the single currently-active version.
func SelectActiveRuntimeVersion(versions []RuntimeCatalogVersion, now time.Time) (RuntimeCatalogVersion, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	active := make([]RuntimeCatalogVersion, 0, 1)
	for _, version := range versions {
		version = version.Normalize()
		if err := version.Valid(); err != nil {
			return RuntimeCatalogVersion{}, err
		}
		if version.ActiveAt(now) {
			active = append(active, version)
		}
	}
	switch len(active) {
	case 0:
		return RuntimeCatalogVersion{}, ErrNoActiveVersion
	case 1:
		return active[0], nil
	default:
		return RuntimeCatalogVersion{}, fmt.Errorf("%w: %d active versions", ErrActiveVersionOverlap, len(active))
	}
}

func (v RuntimeCatalogVersion) Normalize() RuntimeCatalogVersion {
	v.ID = strings.TrimSpace(v.ID)
	v.Status = strings.TrimSpace(v.Status)
	if v.Status == "" {
		v.Status = RuntimePriceVersionStatusActive
	}
	return v
}

func (v RuntimeCatalogVersion) Valid() error {
	v = v.Normalize()
	if v.PriceVersion <= 0 {
		return fmt.Errorf("%w: price version %d", ErrInvalidRuntimePrice, v.PriceVersion)
	}
	if v.Status != RuntimePriceVersionStatusActive {
		return fmt.Errorf("%w: status %q", ErrInvalidRuntimePrice, v.Status)
	}
	if v.EffectiveUntil != nil && !v.EffectiveFrom.IsZero() && !v.EffectiveUntil.After(v.EffectiveFrom) {
		return fmt.Errorf("%w: invalid effective window", ErrInvalidRuntimePrice)
	}
	return nil
}

func (v RuntimeCatalogVersion) ActiveAt(now time.Time) bool {
	v = v.Normalize()
	if v.Status != RuntimePriceVersionStatusActive {
		return false
	}
	if !v.EffectiveFrom.IsZero() && now.Before(v.EffectiveFrom) {
		return false
	}
	return v.EffectiveUntil == nil || now.Before(*v.EffectiveUntil)
}

func (s RuntimePriceSet) Catalog() (*Catalog, error) {
	prices, err := s.ValidatedPrices()
	if err != nil {
		return nil, err
	}
	return NewCatalog(prices)
}

func (s RuntimePriceSet) ValidatedPrices() ([]ProductPrice, error) {
	version := s.Version.Normalize()
	if err := version.Valid(); err != nil {
		return nil, err
	}
	if len(s.Prices) == 0 {
		return nil, fmt.Errorf("%w: active version has no enabled prices", ErrInvalidRuntimePrice)
	}

	prices := make([]ProductPrice, 0, len(s.Prices))
	for idx, price := range s.Prices {
		price = price.Normalize()
		price.Source = RuntimeDBSource
		price.Version = version.PriceVersion
		if err := validateProductPrice(price); err != nil {
			return nil, fmt.Errorf("%w: row %d: %w", ErrInvalidRuntimePrice, idx, err)
		}
		if _, err := price.InternalCredits(); err != nil {
			return nil, fmt.Errorf("%w: row %d: %w", ErrInvalidRuntimePrice, idx, err)
		}
		prices = append(prices, price)
	}
	if _, err := NewCatalog(prices); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRuntimePrice, err)
	}
	return prices, nil
}

// UnitConversionForFloorUnit maps exact minor units to internal credits without
// accepting ambiguous generic provider credits.
func UnitConversionForFloorUnit(unit FloorUnit) (UnitConversion, bool) {
	switch unit {
	case FloorUnitUSDMicros:
		return UnitConversion{InternalCreditUnits: 200, FloorUnits: 1}, true
	case FloorUnitPoYoCredits, FloorUnitInternalCredits:
		return IdentityUnitConversion(), true
	case FloorUnitAPIMartCredits:
		return UnitConversion{InternalCreditUnits: 20, FloorUnits: 1}, true
	case FloorUnitRunwayCredits:
		return UnitConversion{InternalCreditUnits: 2, FloorUnits: 1}, true
	default:
		return UnitConversion{}, false
	}
}

func mustUnitConversionForFloorUnit(unit FloorUnit) UnitConversion {
	conversion, ok := UnitConversionForFloorUnit(unit)
	if !ok {
		panic("pricing catalog: unknown floor unit " + string(unit))
	}
	return conversion
}
