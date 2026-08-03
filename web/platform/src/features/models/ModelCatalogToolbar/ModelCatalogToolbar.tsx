"use client";

import { ru } from "@/i18n/ru";

import type { ImageModelSort } from "../ModelsCatalog/model-filters";

import styles from "./ModelCatalogToolbar.module.css";

type ModelCatalogToolbarProps = {
  query: string;
  referenceOnly: boolean;
  quality: string | null;
  qualities: string[];
  sort: ImageModelSort;
  resultCount: number;
  onQueryChange: (value: string) => void;
  onReferenceOnlyChange: (value: boolean) => void;
  onQualityChange: (value: string | null) => void;
  onSortChange: (value: ImageModelSort) => void;
  onClear: () => void;
};

export function ModelCatalogToolbar({
  onClear,
  onQualityChange,
  onQueryChange,
  onReferenceOnlyChange,
  onSortChange,
  qualities,
  quality,
  query,
  referenceOnly,
  resultCount,
  sort,
}: ModelCatalogToolbarProps) {
  const hasActiveFilters = query.trim() !== "" || referenceOnly || quality !== null || sort !== "catalog";

  return (
    <div className={styles.toolbar}>
      <div className={styles.controls}>
        <input
          aria-label={ru.modelsCatalog.searchLabel}
          className={styles.search}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={ru.modelsCatalog.searchPlaceholder}
          type="search"
          value={query}
        />
        <label className={styles.checkbox}>
          <input
            checked={referenceOnly}
            onChange={(event) => onReferenceOnlyChange(event.target.checked)}
            type="checkbox"
          />
          {ru.modelsCatalog.referenceFilterLabel}
        </label>
        <label className={styles.selectLabel}>
          <span>{ru.modelsCatalog.qualityFilterLabel}</span>
          <select
            aria-label={ru.modelsCatalog.qualityFilterLabel}
            onChange={(event) => onQualityChange(event.target.value || null)}
            value={quality ?? ""}
          >
            <option value="">{ru.modelsCatalog.allQualitiesLabel}</option>
            {qualities.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </label>
        <label className={styles.selectLabel}>
          <span>{ru.modelsCatalog.sortLabel}</span>
          <select
            aria-label={ru.modelsCatalog.sortLabel}
            onChange={(event) => onSortChange(event.target.value as ImageModelSort)}
            value={sort}
          >
            <option value="catalog">{ru.modelsCatalog.catalogSortLabel}</option>
            <option value="name">{ru.modelsCatalog.nameSortLabel}</option>
          </select>
        </label>
      </div>
      <div className={styles.summary}>
        <p aria-live="polite" className={styles.resultCount}>
          {ru.modelsCatalog.resultCount(resultCount)}
        </p>
        <button disabled={!hasActiveFilters} onClick={onClear} type="button">
          {ru.modelsCatalog.clearFiltersLabel}
        </button>
      </div>
    </div>
  );
}
