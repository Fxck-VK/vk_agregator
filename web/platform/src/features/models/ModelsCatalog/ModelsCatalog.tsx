"use client";

import { useDeferredValue, useEffect, useMemo, useState } from "react";

import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { loadImageModelCatalog } from "../image-model-catalog-cache";
import { ModelCard } from "../ModelCard/ModelCard";
import {
  getModelCatalogCategoryTabId,
  ModelCatalogToolbar,
  type ModelCatalogCategory,
} from "../ModelCatalogToolbar/ModelCatalogToolbar";
import { filterAndSortImageModels } from "./model-filters";
import styles from "./ModelsCatalog.module.css";

type CatalogStatus = "loading" | "ready" | "failure";

const modelsCatalogPanelId = "models-catalog-panel";

export function ModelsCatalog() {
  const [status, setStatus] = useState<CatalogStatus>("loading");
  const [models, setModels] = useState<ImageModel[]>([]);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<ModelCatalogCategory["id"]>("popular");
  const deferredQuery = useDeferredValue(query);

  useEffect(() => {
    let active = true;

    const loadModels = async () => {
      try {
        const catalog = await loadImageModelCatalog();
        if (!active) {
          return;
        }
        setModels(catalog.items);
        setStatus("ready");
      } catch {
        if (active) {
          setStatus("failure");
        }
      }
    };

    void loadModels();
    return () => {
      active = false;
    };
  }, []);

  const filteredModels = useMemo(
    () => filterAndSortImageModels(models, { query: deferredQuery, referenceOnly: false, quality: null }, "catalog"),
    [deferredQuery, models],
  );
  const selectedCategory = ru.modelsCatalog.categories.find((item) => item.id === category) ?? ru.modelsCatalog.categories[0];
  const showImageModels = category === "popular" || category === "images";

  return (
    <section aria-labelledby="models-catalog-title" className={styles.catalog}>
      <header className={styles.header}>
        <h1 id="models-catalog-title">{ru.modelsCatalog.title}</h1>
        <p>{ru.modelsCatalog.description}</p>
      </header>

      {status === "loading" ? <p role="status">{ru.modelsCatalog.loading}</p> : null}
      {status === "failure" ? (
        <p className={styles.error} role="alert">
          {ru.modelsCatalog.loadFailure}
        </p>
      ) : null}

      {status === "ready" ? (
        <>
          <ModelCatalogToolbar
            categories={ru.modelsCatalog.categories}
            category={category}
            onCategoryChange={setCategory}
            onQueryChange={setQuery}
            query={query}
            tabPanelId={modelsCatalogPanelId}
          />

          <div
            aria-labelledby={getModelCatalogCategoryTabId(category)}
            className={styles.section}
            id={modelsCatalogPanelId}
            role="tabpanel"
          >
            <h2 className={styles.sectionTitle}>{selectedCategory.label}</h2>

            {showImageModels && filteredModels.length === 0 ? (
              <p className={styles.emptyState}>{ru.modelsCatalog.empty}</p>
            ) : null}

            {showImageModels && filteredModels.length > 0 ? (
              <div className={styles.grid}>
                {filteredModels.map((model) => (
                  <ModelCard key={model.id} model={model} />
                ))}
              </div>
            ) : null}

            {!showImageModels ? (
              <p className={styles.emptyState}>{ru.modelsCatalog.categoryComingSoon(selectedCategory.label)}</p>
            ) : null}
          </div>
        </>
      ) : null}
    </section>
  );
}
