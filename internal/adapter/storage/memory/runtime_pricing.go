package memory

import (
	"context"
	"sync"
	"time"

	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// RuntimePricingRepo is an in-memory runtime pricing repository for tests.
type RuntimePricingRepo struct {
	mu       sync.Mutex
	versions []pricingcatalog.RuntimeCatalogVersion
	prices   []pricingcatalog.ProductPrice
	now      func() time.Time
}

func NewRuntimePricingRepo(versions []pricingcatalog.RuntimeCatalogVersion, prices []pricingcatalog.ProductPrice) *RuntimePricingRepo {
	return &RuntimePricingRepo{
		versions: append([]pricingcatalog.RuntimeCatalogVersion(nil), versions...),
		prices:   append([]pricingcatalog.ProductPrice(nil), prices...),
		now:      time.Now,
	}
}

var _ pricingcatalog.RuntimePricingRepository = (*RuntimePricingRepo)(nil)

func (r *RuntimePricingRepo) ListActivePrices(_ context.Context) (pricingcatalog.RuntimePriceSet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	version, err := pricingcatalog.SelectActiveRuntimeVersion(r.versions, r.now().UTC())
	if err != nil {
		return pricingcatalog.RuntimePriceSet{}, err
	}
	out := make([]pricingcatalog.ProductPrice, 0, len(r.prices))
	for _, price := range r.prices {
		if price.Version == version.PriceVersion && price.Enabled {
			out = append(out, price)
		}
	}
	set := pricingcatalog.RuntimePriceSet{Version: version, Prices: out}
	if _, err := set.ValidatedPrices(); err != nil {
		return pricingcatalog.RuntimePriceSet{}, err
	}
	return set, nil
}

func (r *RuntimePricingRepo) GetActivePrice(ctx context.Context, key pricingcatalog.ProductKey) (pricingcatalog.ProductPrice, error) {
	key = key.Normalize()
	if !key.Valid() {
		return pricingcatalog.ProductPrice{}, pricingcatalog.ErrInvalidProductKey
	}
	set, err := r.ListActivePrices(ctx)
	if err != nil {
		return pricingcatalog.ProductPrice{}, err
	}
	for _, price := range set.Prices {
		if price.Key.Normalize() == key {
			price.Source = pricingcatalog.RuntimeDBSource
			price.Version = set.Version.PriceVersion
			if _, err := price.InternalCredits(); err != nil {
				return pricingcatalog.ProductPrice{}, err
			}
			return price, nil
		}
	}
	return pricingcatalog.ProductPrice{}, pricingcatalog.ErrPriceNotFound
}

func (r *RuntimePricingRepo) SetNowForTest(now func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if now == nil {
		r.now = time.Now
		return
	}
	r.now = now
}
