"use client";

import { useDeferredValue, useEffect, useMemo, useState } from "react";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { ru } from "@/i18n/ru";

import { loadGenerationModelCatalog, type GenerationModel } from "../generation-model-catalog";
import { ModelCard } from "../ModelCard/ModelCard";
import {
  getModelCatalogCategoryTabId,
  ModelCatalogToolbar,
  type ModelCatalogCategory,
} from "../ModelCatalogToolbar/ModelCatalogToolbar";
import { filterAndSortCatalogModels } from "./model-filters";
import styles from "./ModelsCatalog.module.css";

type CatalogStatus = "loading" | "ready" | "failure";

const modelsCatalogPanelId = "models-catalog-panel";

export function ModelsCatalog() {
  const [status, setStatus] = useState<CatalogStatus>("loading");
  const [models, setModels] = useState<GenerationModel[]>([]);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<ModelCatalogCategory["id"]>("popular");
  const deferredQuery = useDeferredValue(query);

  useEffect(() => {
    let active = true;

    const loadModels = async () => {
      try {
        const catalog = await loadGenerationModelCatalog();
        if (!active) {
          return;
        }
        setModels(catalog.items);
        setStatus(catalog.items.length === 0 && Object.keys(catalog.categoryErrors).length > 0 ? "failure" : "ready");
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
    () => filterAndSortCatalogModels(models, { category, query: deferredQuery }, "catalog"),
    [category, deferredQuery, models],
  );
  const selectedCategory = ru.modelsCatalog.categories.find((item) => item.id === category) ?? ru.modelsCatalog.categories[0];

  return (
    <WorkspacePageFrame>
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

              {filteredModels.length === 0 ? (
                <p className={styles.emptyState}>{ru.modelsCatalog.empty}</p>
              ) : null}

              {filteredModels.length > 0 ? (
                <div className={styles.grid}>
                  {filteredModels.map((model) => (
                    <ModelCard
                      key={model.id}
                      model={model}
                    />
                  ))}
                </div>
              ) : null}
            </div>
          </>
        ) : null}
      </section>
    </WorkspacePageFrame>
  );
}
