"use client";

import { SearchIcon } from "@/components/icons/SearchIcon";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { ru } from "@/i18n/ru";

import styles from "./ModelCatalogToolbar.module.css";

export type ModelCatalogCategory = (typeof ru.modelsCatalog.categories)[number];

type ModelCatalogToolbarProps = {
  query: string;
  categories: readonly ModelCatalogCategory[];
  category: ModelCatalogCategory["id"];
  tabPanelId: string;
  onQueryChange: (value: string) => void;
  onCategoryChange: (value: ModelCatalogCategory["id"]) => void;
};

export function getModelCatalogCategoryTabId(category: ModelCatalogCategory["id"]) {
  return `model-category-tab-${category}`;
}

export function ModelCatalogToolbar({
  categories,
  category,
  onCategoryChange,
  onQueryChange,
  query,
  tabPanelId,
}: ModelCatalogToolbarProps) {
  return (
    <div className={styles.toolbar}>
      <div className={styles.controls}>
        <InputSurface className={styles.searchField}>
          <SearchIcon className={styles.searchIcon} />
          <input
            aria-label={ru.modelsCatalog.searchLabel}
            className={styles.search}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder={ru.modelsCatalog.searchPlaceholder}
            type="search"
            value={query}
          />
        </InputSurface>
        <ModeSwitchPanel
          activeID={category}
          ariaLabel={ru.modelsCatalog.categoryTabsLabel}
          className={styles.categoryPanel}
          items={categories.map((item) => ({
            ariaControls: tabPanelId,
            elementID: getModelCatalogCategoryTabId(item.id),
            ...item,
          }))}
          onChange={onCategoryChange}
          semantics="tabs"
        />
      </div>
    </div>
  );
}
