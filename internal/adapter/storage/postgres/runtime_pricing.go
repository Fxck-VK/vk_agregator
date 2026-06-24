package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"vk-ai-aggregator/internal/domain"
	"vk-ai-aggregator/internal/service/pricingcatalog"
)

// RuntimePricingRepository reads DB-backed generation pricing.
type RuntimePricingRepository struct {
	db Querier
}

// NewRuntimePricingRepository builds a RuntimePricingRepository over the given
// querier. It is read-only; runtime price writes are intentionally absent.
func NewRuntimePricingRepository(db Querier) *RuntimePricingRepository {
	return &RuntimePricingRepository{db: db}
}

var _ pricingcatalog.RuntimePricingRepository = (*RuntimePricingRepository)(nil)

// ListActivePrices lists enabled prices for the single active runtime version.
func (r *RuntimePricingRepository) ListActivePrices(ctx context.Context) (pricingcatalog.RuntimePriceSet, error) {
	version, err := r.activeVersion(ctx)
	if err != nil {
		return pricingcatalog.RuntimePriceSet{}, err
	}
	const q = `
SELECT operation, modality, image_model_id, video_route_alias, quality, resolution,
       duration_sec, floor_amount, floor_unit, multiplier_numerator,
       multiplier_denominator, internal_credit_cap, floor_amount_cap, enabled
FROM runtime_generation_prices
WHERE catalog_version_id = $1 AND enabled = true
ORDER BY operation, modality, image_model_id, video_route_alias, quality, resolution, duration_sec`
	rows, err := r.db.Query(ctx, q, version.ID)
	if err != nil {
		return pricingcatalog.RuntimePriceSet{}, fmt.Errorf("postgres runtime pricing list: %w", err)
	}
	defer rows.Close()

	prices := make([]pricingcatalog.ProductPrice, 0)
	for rows.Next() {
		price, err := scanRuntimeProductPrice(rows, version)
		if err != nil {
			return pricingcatalog.RuntimePriceSet{}, err
		}
		prices = append(prices, price)
	}
	if err := rows.Err(); err != nil {
		return pricingcatalog.RuntimePriceSet{}, fmt.Errorf("postgres runtime pricing list rows: %w", err)
	}
	set := pricingcatalog.RuntimePriceSet{Version: version, Prices: prices}
	if _, err := set.ValidatedPrices(); err != nil {
		return pricingcatalog.RuntimePriceSet{}, err
	}
	return set, nil
}

// GetActivePrice fetches one enabled runtime price by public product key.
func (r *RuntimePricingRepository) GetActivePrice(ctx context.Context, key pricingcatalog.ProductKey) (pricingcatalog.ProductPrice, error) {
	key = key.Normalize()
	if !key.Valid() {
		return pricingcatalog.ProductPrice{}, pricingcatalog.ErrInvalidProductKey
	}
	version, err := r.activeVersion(ctx)
	if err != nil {
		return pricingcatalog.ProductPrice{}, err
	}
	const q = `
SELECT operation, modality, image_model_id, video_route_alias, quality, resolution,
       duration_sec, floor_amount, floor_unit, multiplier_numerator,
       multiplier_denominator, internal_credit_cap, floor_amount_cap, enabled
FROM runtime_generation_prices
WHERE catalog_version_id = $1
  AND enabled = true
  AND operation = $2
  AND modality = $3
  AND image_model_id = $4
  AND video_route_alias = $5
  AND quality = $6
  AND resolution = $7
  AND duration_sec = $8`
	price, err := scanRuntimeProductPrice(r.db.QueryRow(
		ctx,
		q,
		version.ID,
		string(key.Operation),
		string(key.Modality),
		key.ImageModelID,
		string(key.VideoRouteAlias),
		key.Quality,
		key.Resolution,
		key.DurationSec,
	), version)
	if errors.Is(err, pgx.ErrNoRows) {
		return pricingcatalog.ProductPrice{}, pricingcatalog.ErrPriceNotFound
	}
	if err != nil {
		return pricingcatalog.ProductPrice{}, err
	}
	if _, err := price.InternalCredits(); err != nil {
		return pricingcatalog.ProductPrice{}, fmt.Errorf("%w: %w", pricingcatalog.ErrInvalidRuntimePrice, err)
	}
	return price, nil
}

func (r *RuntimePricingRepository) activeVersion(ctx context.Context) (pricingcatalog.RuntimeCatalogVersion, error) {
	const q = `
SELECT id::text, price_version, status, effective_from, effective_until
FROM runtime_pricing_catalog_versions
WHERE status = 'active'
ORDER BY effective_from DESC, price_version DESC`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return pricingcatalog.RuntimeCatalogVersion{}, fmt.Errorf("postgres runtime pricing active version: %w", err)
	}
	defer rows.Close()

	versions := make([]pricingcatalog.RuntimeCatalogVersion, 0, 1)
	for rows.Next() {
		var version pricingcatalog.RuntimeCatalogVersion
		var effectiveUntil *time.Time
		if err := rows.Scan(&version.ID, &version.PriceVersion, &version.Status, &version.EffectiveFrom, &effectiveUntil); err != nil {
			return pricingcatalog.RuntimeCatalogVersion{}, err
		}
		if effectiveUntil != nil {
			until := *effectiveUntil
			version.EffectiveUntil = &until
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return pricingcatalog.RuntimeCatalogVersion{}, fmt.Errorf("postgres runtime pricing active version rows: %w", err)
	}
	return pricingcatalog.SelectActiveRuntimeVersion(versions, time.Now().UTC())
}

func scanRuntimeProductPrice(row rowScanner, version pricingcatalog.RuntimeCatalogVersion) (pricingcatalog.ProductPrice, error) {
	var (
		operation             string
		modality              string
		imageModelID          string
		videoRouteAlias       string
		quality               string
		resolution            string
		durationSec           int
		floorAmount           int64
		floorUnit             string
		multiplierNumerator   int64
		multiplierDenominator int64
		internalCreditCap     int64
		floorAmountCap        int64
		enabled               bool
	)
	if err := row.Scan(
		&operation,
		&modality,
		&imageModelID,
		&videoRouteAlias,
		&quality,
		&resolution,
		&durationSec,
		&floorAmount,
		&floorUnit,
		&multiplierNumerator,
		&multiplierDenominator,
		&internalCreditCap,
		&floorAmountCap,
		&enabled,
	); err != nil {
		return pricingcatalog.ProductPrice{}, err
	}

	unit := pricingcatalog.FloorUnit(floorUnit)
	conversion, ok := pricingcatalog.UnitConversionForFloorUnit(unit)
	if !ok {
		return pricingcatalog.ProductPrice{}, fmt.Errorf("%w: unknown floor unit %q", pricingcatalog.ErrInvalidRuntimePrice, floorUnit)
	}
	price := pricingcatalog.ProductPrice{
		Key: pricingcatalog.ProductKey{
			Operation:       domain.OperationType(operation),
			Modality:        domain.Modality(modality),
			ImageModelID:    imageModelID,
			VideoRouteAlias: domain.VideoRouteAlias(videoRouteAlias),
			Quality:         quality,
			Resolution:      resolution,
			DurationSec:     durationSec,
		},
		Version: version.PriceVersion,
		Source:  pricingcatalog.RuntimeDBSource,
		Floor: pricingcatalog.PriceFloor{
			Amount: floorAmount,
			Unit:   unit,
		},
		Multiplier: pricingcatalog.Multiplier{
			Numerator:   multiplierNumerator,
			Denominator: multiplierDenominator,
		},
		UnitConversion: conversion,
		Caps: pricingcatalog.SafetyCaps{
			InternalCreditCap: internalCreditCap,
			FloorAmountCap:    floorAmountCap,
		},
		Enabled: enabled,
	}
	if _, err := price.InternalCredits(); err != nil {
		return pricingcatalog.ProductPrice{}, fmt.Errorf("%w: %w", pricingcatalog.ErrInvalidRuntimePrice, err)
	}
	return price, nil
}
