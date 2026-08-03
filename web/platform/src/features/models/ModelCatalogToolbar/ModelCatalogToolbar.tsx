"use client";

import { useRef } from "react";

import { ru } from "@/i18n/ru";

import styles from "./ModelCatalogToolbar.module.css";

export type ModelCatalogCategory = (typeof ru.modelsCatalog.categories)[number];

type ModelCatalogToolbarProps = {
  query: string;
  categories: readonly ModelCatalogCategory[];
  category: ModelCatalogCategory["id"];
  onQueryChange: (value: string) => void;
  onCategoryChange: (value: ModelCatalogCategory["id"]) => void;
};

export function ModelCatalogToolbar({
  categories,
  category,
  onCategoryChange,
  onQueryChange,
  query,
}: ModelCatalogToolbarProps) {
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const selectCategory = (index: number) => {
    const nextCategory = categories[index];
    if (!nextCategory) {
      return;
    }

    onCategoryChange(nextCategory.id);
    tabRefs.current[index]?.focus();
  };

  return (
    <div className={styles.toolbar}>
      <div className={styles.controls}>
        <div className={styles.searchField}>
          <span aria-hidden="true" className={styles.searchIcon}>
            ⌕
          </span>
          <input
            aria-label={ru.modelsCatalog.searchLabel}
            className={styles.search}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder={ru.modelsCatalog.searchPlaceholder}
            type="search"
            value={query}
          />
        </div>
        <div className={styles.categoryList} role="tablist">
          {categories.map((item, index) => (
            <button
              aria-selected={item.id === category}
              className={styles.category}
              key={item.id}
              onClick={() => onCategoryChange(item.id)}
              onKeyDown={(event) => {
                if (event.key === "ArrowLeft") {
                  event.preventDefault();
                  selectCategory((index - 1 + categories.length) % categories.length);
                }
                if (event.key === "ArrowRight") {
                  event.preventDefault();
                  selectCategory((index + 1) % categories.length);
                }
                if (event.key === "Home") {
                  event.preventDefault();
                  selectCategory(0);
                }
                if (event.key === "End") {
                  event.preventDefault();
                  selectCategory(categories.length - 1);
                }
              }}
              ref={(element) => {
                tabRefs.current[index] = element;
              }}
              role="tab"
              tabIndex={item.id === category ? 0 : -1}
              type="button"
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
