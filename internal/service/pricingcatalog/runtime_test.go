package pricingcatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"vk-ai-aggregator/internal/domain"
)

func TestRuntimePriceSetValidationFailsClosed(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	version := RuntimeCatalogVersion{
		ID:            "runtime-version",
		PriceVersion:  17,
		Status:        RuntimePriceVersionStatusActive,
		EffectiveFrom: now.Add(-time.Hour),
	}
	key := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: "nano_banana_2",
		Quality:      ImageQuality1K,
	}

	cases := []struct {
		name  string
		price ProductPrice
	}{
		{
			name: "unexpectedly disabled",
			price: ProductPrice{
				Key:            key,
				Floor:          PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
				Multiplier:     DefaultMultiplier(),
				UnitConversion: IdentityUnitConversion(),
				Enabled:        false,
			},
		},
		{
			name: "zero floor",
			price: ProductPrice{
				Key:            key,
				Floor:          PriceFloor{Unit: FloorUnitPoYoCredits},
				Multiplier:     DefaultMultiplier(),
				UnitConversion: IdentityUnitConversion(),
				Enabled:        true,
			},
		},
		{
			name: "unknown unit",
			price: ProductPrice{
				Key:            key,
				Floor:          PriceFloor{Amount: 1, Unit: "provider_credit_micros"},
				Multiplier:     DefaultMultiplier(),
				UnitConversion: IdentityUnitConversion(),
				Enabled:        true,
			},
		},
		{
			name: "zero multiplier",
			price: ProductPrice{
				Key:            key,
				Floor:          PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
				Multiplier:     Multiplier{Numerator: 0, Denominator: 1},
				UnitConversion: IdentityUnitConversion(),
				Enabled:        true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := RuntimePriceSet{Version: version, Prices: []ProductPrice{tc.price}}
			if _, err := set.Catalog(); !errors.Is(err, ErrInvalidRuntimePrice) {
				t.Fatalf("Catalog() error = %v, want ErrInvalidRuntimePrice", err)
			}
		})
	}

	emptySet := RuntimePriceSet{Version: version}
	if _, err := emptySet.Catalog(); !errors.Is(err, ErrInvalidRuntimePrice) {
		t.Fatalf("empty active runtime price set error = %v, want ErrInvalidRuntimePrice", err)
	}
}

func TestSelectActiveRuntimeVersionRejectsOverlap(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	versions := []RuntimeCatalogVersion{
		{ID: "v1", PriceVersion: 1, Status: RuntimePriceVersionStatusActive, EffectiveFrom: now.Add(-time.Hour)},
		{ID: "v2", PriceVersion: 2, Status: RuntimePriceVersionStatusActive, EffectiveFrom: now.Add(-time.Minute)},
	}

	_, err := SelectActiveRuntimeVersion(versions, now)
	if !errors.Is(err, ErrActiveVersionOverlap) {
		t.Fatalf("SelectActiveRuntimeVersion() error = %v, want ErrActiveVersionOverlap", err)
	}

	_, err = SelectActiveRuntimeVersion([]RuntimeCatalogVersion{
		{ID: "future", PriceVersion: 3, Status: RuntimePriceVersionStatusActive, EffectiveFrom: now.Add(time.Hour)},
	}, now)
	if !errors.Is(err, ErrNoActiveVersion) {
		t.Fatalf("future-only active version error = %v, want ErrNoActiveVersion", err)
	}
}

func TestResolveRuntimeCatalogRequiresExplicitStaticFallback(t *testing.T) {
	staticCatalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("static catalog: %v", err)
	}
	ctx := context.Background()

	_, _, err = ResolveRuntimeCatalog(ctx, nil, staticCatalog, RuntimeCatalogConfig{})
	if !errors.Is(err, ErrRuntimePricingOff) {
		t.Fatalf("ResolveRuntimeCatalog() without fallback error = %v, want ErrRuntimePricingOff", err)
	}

	catalog, selection, err := ResolveRuntimeCatalog(ctx, nil, staticCatalog, RuntimeCatalogConfig{StaticFallbackEnabled: true})
	if err != nil {
		t.Fatalf("ResolveRuntimeCatalog() static fallback: %v", err)
	}
	if catalog != staticCatalog || selection.Source != StaticSource || !selection.StaticFallback {
		t.Fatalf("unexpected static fallback selection: %+v", selection)
	}

	_, _, err = ResolveRuntimeCatalog(ctx, invalidRuntimeRepo{}, staticCatalog, RuntimeCatalogConfig{DBEnabled: true, StaticFallbackEnabled: true})
	if !errors.Is(err, ErrInvalidRuntimePrice) {
		t.Fatalf("DB-enabled invalid pricing error = %v, want ErrInvalidRuntimePrice", err)
	}
}

func TestRuntimeCatalogCacheReloadKeepsStablePointerAndFailsClosed(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	key := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: PublicImageNanoBanana2,
		Quality:      ImageQuality1K,
	}
	repo := &mutableRuntimeRepo{}
	repo.setSet(RuntimePriceSet{
		Version: RuntimeCatalogVersion{
			ID:            "v1",
			PriceVersion:  21,
			Status:        RuntimePriceVersionStatusActive,
			EffectiveFrom: now.Add(-time.Hour),
		},
		Prices: []ProductPrice{
			runtimeTestPrice(key, 5_000_000),
		},
	})
	cache := NewRuntimeCatalogCache(repo, nil, RuntimeCatalogConfig{DBEnabled: true})
	if err := cache.Load(context.Background()); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	catalog, selection, err := cache.Current()
	if err != nil {
		t.Fatalf("current after load: %v", err)
	}
	if selection.Source != RuntimeDBSource || selection.Version != 21 || selection.StaticFallback {
		t.Fatalf("unexpected selection: %+v", selection)
	}
	if selection.EffectiveFrom.IsZero() || selection.EffectiveUntil != nil {
		t.Fatalf("unexpected selection effective metadata: %+v", selection)
	}
	credits, err := catalog.CostEstimateCredits(key)
	if err != nil {
		t.Fatalf("initial estimate: %v", err)
	}
	if credits != 15 {
		t.Fatalf("initial credits = %d, want 15", credits)
	}

	repo.setSet(RuntimePriceSet{
		Version: RuntimeCatalogVersion{
			ID:            "v2",
			PriceVersion:  22,
			Status:        RuntimePriceVersionStatusActive,
			EffectiveFrom: now.Add(-time.Minute),
		},
		Prices: []ProductPrice{
			runtimeTestPrice(key, 8_000_000),
		},
	})
	if err := cache.Reload(context.Background()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	reloaded, selection, err := cache.Current()
	if err != nil {
		t.Fatalf("current after reload: %v", err)
	}
	if reloaded != catalog {
		t.Fatal("reload replaced catalog pointer; consumers must keep a stable pointer")
	}
	if selection.Version != 22 {
		t.Fatalf("selection version = %d, want 22", selection.Version)
	}
	credits, err = catalog.CostEstimateCredits(key)
	if err != nil {
		t.Fatalf("reloaded estimate: %v", err)
	}
	if credits != 24 {
		t.Fatalf("reloaded credits = %d, want 24", credits)
	}

	repo.setErr(ErrInvalidRuntimePrice)
	if err := cache.Reload(context.Background()); !errors.Is(err, ErrInvalidRuntimePrice) {
		t.Fatalf("invalid reload error = %v, want ErrInvalidRuntimePrice", err)
	}
	credits, err = catalog.CostEstimateCredits(key)
	if err != nil {
		t.Fatalf("estimate after failed reload: %v", err)
	}
	if credits != 24 {
		t.Fatalf("failed reload changed catalog credits = %d, want 24", credits)
	}
}

func TestRuntimeCatalogSnapshotsRemainImmutableAcrossDBPriceChanges(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	key := ProductKey{
		Operation:    domain.OperationImageGenerate,
		Modality:     domain.ModalityImage,
		ImageModelID: PublicImageNanoBanana2,
		Quality:      ImageQuality1K,
	}
	repo := &mutableRuntimeRepo{}
	repo.setSet(RuntimePriceSet{
		Version: RuntimeCatalogVersion{
			ID:            "db-v1",
			PriceVersion:  31,
			Status:        RuntimePriceVersionStatusActive,
			EffectiveFrom: now.Add(-time.Hour),
		},
		Prices: []ProductPrice{runtimeTestPrice(key, 5_000_000)},
	})
	cache := NewRuntimeCatalogCache(repo, nil, RuntimeCatalogConfig{DBEnabled: true})
	if err := cache.Load(context.Background()); err != nil {
		t.Fatalf("initial load: %v", err)
	}
	catalog, _, err := cache.Current()
	if err != nil {
		t.Fatalf("current catalog: %v", err)
	}
	dbSnapshotV1, err := catalog.Snapshot(key)
	if err != nil {
		t.Fatalf("db snapshot v1: %v", err)
	}
	dbSnapshotV1Raw, err := json.Marshal(dbSnapshotV1)
	if err != nil {
		t.Fatalf("marshal db snapshot v1: %v", err)
	}
	oldDBJob := domain.Job{PricingSnapshot: dbSnapshotV1Raw, CostReserved: 999}
	if got := oldDBJob.ChargeAmountCredits(); got != 15 {
		t.Fatalf("old DB-backed snapshot charge = %d, want 15", got)
	}

	staticCatalog, err := NewStaticCatalog()
	if err != nil {
		t.Fatalf("static catalog: %v", err)
	}
	staticSnapshot, err := staticCatalog.Snapshot(key)
	if err != nil {
		t.Fatalf("static snapshot: %v", err)
	}
	staticSnapshotRaw, err := json.Marshal(staticSnapshot)
	if err != nil {
		t.Fatalf("marshal static snapshot: %v", err)
	}
	staticJob := domain.Job{PricingSnapshot: staticSnapshotRaw, CostReserved: 999}
	if got := staticJob.ChargeAmountCredits(); got != staticSnapshot.InternalCredits {
		t.Fatalf("static snapshot charge = %d, want %d", got, staticSnapshot.InternalCredits)
	}

	legacyJob := domain.Job{CostReserved: 7}
	if credits, ok := legacyJob.PricingSnapshotCredits(); ok || credits != 0 {
		t.Fatalf("legacy no-snapshot credits = %d/%v, want 0/false", credits, ok)
	}
	if got := legacyJob.ChargeAmountCredits(); got != 7 {
		t.Fatalf("legacy no-snapshot charge = %d, want reserved 7", got)
	}

	repo.setSet(RuntimePriceSet{
		Version: RuntimeCatalogVersion{
			ID:            "db-v2",
			PriceVersion:  32,
			Status:        RuntimePriceVersionStatusActive,
			EffectiveFrom: now.Add(-time.Minute),
		},
		Prices: []ProductPrice{runtimeTestPrice(key, 8_000_000)},
	})
	if err := cache.Reload(context.Background()); err != nil {
		t.Fatalf("reload v2: %v", err)
	}
	if got := oldDBJob.ChargeAmountCredits(); got != 15 {
		t.Fatalf("old DB-backed snapshot changed after runtime price reload: got %d, want 15", got)
	}
	dbSnapshotV2, err := catalog.Snapshot(key)
	if err != nil {
		t.Fatalf("db snapshot v2: %v", err)
	}
	if dbSnapshotV2.InternalCredits != 24 || dbSnapshotV2.Version != 32 {
		t.Fatalf("new DB-backed snapshot = %+v, want version 32 and 24 credits", dbSnapshotV2)
	}
}

func TestUnitConversionForFloorUnitIsExact(t *testing.T) {
	cases := []struct {
		unit        FloorUnit
		amount      int64
		multiplier  Multiplier
		wantCredits int64
	}{
		{unit: FloorUnitUSDMicros, amount: 5_000, multiplier: Multiplier{Numerator: 1, Denominator: 1}, wantCredits: 1},
		{unit: FloorUnitPoYoCredits, amount: 1_000_000, multiplier: DefaultMultiplier(), wantCredits: 3},
		{unit: FloorUnitAPIMartCredits, amount: 1_000_000, multiplier: DefaultMultiplier(), wantCredits: 60},
		{unit: FloorUnitRunwayCredits, amount: 1_000_000, multiplier: DefaultMultiplier(), wantCredits: 6},
	}
	for _, tc := range cases {
		t.Run(string(tc.unit), func(t *testing.T) {
			conversion, ok := UnitConversionForFloorUnit(tc.unit)
			if !ok {
				t.Fatalf("UnitConversionForFloorUnit(%q) not found", tc.unit)
			}
			got, err := CalculateInternalCredits(PriceFloor{Amount: tc.amount, Unit: tc.unit}, conversion, tc.multiplier, SafetyCaps{})
			if err != nil {
				t.Fatalf("CalculateInternalCredits() error: %v", err)
			}
			if got != tc.wantCredits {
				t.Fatalf("credits = %d, want %d", got, tc.wantCredits)
			}
		})
	}
	if _, ok := UnitConversionForFloorUnit("credits"); ok {
		t.Fatal("generic credits unit must not be accepted")
	}
}

func TestRuntimeSnapshotDoesNotExposePrivateFields(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	set := RuntimePriceSet{
		Version: RuntimeCatalogVersion{
			ID:            "runtime-version",
			PriceVersion:  17,
			Status:        RuntimePriceVersionStatusActive,
			EffectiveFrom: now.Add(-time.Hour),
		},
		Prices: []ProductPrice{
			{
				Key: ProductKey{
					Operation:    domain.OperationImageGenerate,
					Modality:     domain.ModalityImage,
					ImageModelID: "nano_banana_2",
					Quality:      ImageQuality1K,
				},
				Floor:          PriceFloor{Amount: 5_000_000, Unit: FloorUnitPoYoCredits},
				Multiplier:     DefaultMultiplier(),
				UnitConversion: IdentityUnitConversion(),
				Enabled:        true,
			},
		},
	}
	catalog, err := set.Catalog()
	if err != nil {
		t.Fatalf("runtime catalog: %v", err)
	}
	snapshot, err := catalog.Snapshot(set.Prices[0].Key)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	raw := strings.ToLower(snapshot.Source + " " + snapshot.Key.ImageModelID)
	for _, forbidden := range []string{"provider", "prompt", "payload", "private_url"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("runtime snapshot leaked private marker %q: %+v", forbidden, snapshot)
		}
	}
}

type invalidRuntimeRepo struct{}

func (invalidRuntimeRepo) ListActivePrices(context.Context) (RuntimePriceSet, error) {
	return RuntimePriceSet{}, ErrInvalidRuntimePrice
}

func (invalidRuntimeRepo) GetActivePrice(context.Context, ProductKey) (ProductPrice, error) {
	return ProductPrice{}, ErrInvalidRuntimePrice
}

type mutableRuntimeRepo struct {
	mu  sync.Mutex
	set RuntimePriceSet
	err error
}

func (r *mutableRuntimeRepo) setSet(set RuntimePriceSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set = set
	r.err = nil
}

func (r *mutableRuntimeRepo) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *mutableRuntimeRepo) ListActivePrices(context.Context) (RuntimePriceSet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return RuntimePriceSet{}, r.err
	}
	return r.set, nil
}

func (r *mutableRuntimeRepo) GetActivePrice(_ context.Context, key ProductKey) (ProductPrice, error) {
	set, err := r.ListActivePrices(context.Background())
	if err != nil {
		return ProductPrice{}, err
	}
	key = key.Normalize()
	for _, price := range set.Prices {
		if price.Key.Normalize() == key {
			price.Version = set.Version.PriceVersion
			price.Source = RuntimeDBSource
			return price, nil
		}
	}
	return ProductPrice{}, ErrPriceNotFound
}

func runtimeTestPrice(key ProductKey, floorAmount int64) ProductPrice {
	return ProductPrice{
		Key:            key,
		Floor:          PriceFloor{Amount: floorAmount, Unit: FloorUnitPoYoCredits},
		Multiplier:     DefaultMultiplier(),
		UnitConversion: IdentityUnitConversion(),
		Caps: SafetyCaps{
			InternalCreditCap: 1000,
			FloorAmountCap:    floorAmount,
		},
		Enabled: true,
	}
}
