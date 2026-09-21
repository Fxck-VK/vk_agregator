"use client";

import { StateNotice, Skeleton } from "@/components/ui/AsyncState/AsyncState";
import { LoadFeedback } from "@/components/ui/AsyncState/LoadFeedback";
import { useDictionary } from "@/i18n/LocaleProvider";


import { useDeferredValue, useMemo, useState } from "react";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";

import { useGenerationCatalog } from "../GenerationCatalogProvider";
import { ModelCard } from "../ModelCard/ModelCard";
import {
  getModelCatalogCategoryTabId,
  ModelCatalogToolbar,
  type ModelCatalogCategory,
} from "../ModelCatalogToolbar/ModelCatalogToolbar";
import { filterAndSortCatalogModels } from "./model-filters";
import styles from "./ModelsCatalog.module.css";



const modelsCatalogPanelId = "models-catalog-panel";

export function ModelsCatalog() {
  const t = useDictionary();
  const { catalog, status, pending, failed, retry } = useGenerationCatalog();
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<ModelCatalogCategory["id"]>("popular");
  const deferredQuery = useDeferredValue(query);

  const filteredModels = useMemo(
    () => filterAndSortCatalogModels(catalog?.items ?? [], { category, query: deferredQuery }, "catalog"),
    [category, deferredQuery, catalog],
  );
  const selectedCategory = t.modelsCatalog.categories.find((item) => item.id === category) ?? t.modelsCatalog.categories[0];

  return (
    <WorkspacePageFrame>
      <section aria-labelledby="models-catalog-title" className={styles.catalog}>
        <header className={styles.header}>
          <h1 id="models-catalog-title">{t.modelsCatalog.title}</h1>
          <p>{t.modelsCatalog.description}</p>
        </header>
        <LoadFeedback pending={pending} failed={failed && catalog !== null} hasData={catalog !== null} onRetry={retry} />

        {status === "loading" ? <div role="status" aria-label={t.modelsCatalog.loading}><div className={styles.grid} aria-hidden="true">{Array.from({ length: 6 }, (_, index) => <Skeleton key={index} className={styles.loadingCard} />)}</div></div> : null}
        {status === "failure" ? (
          <StateNotice kind="error" action={{ label: t.files.retry, onClick: retry }}>
            {t.modelsCatalog.loadFailure}
          </StateNotice>
        ) : null}

        {status === "ready" ? (
          <>
            <ModelCatalogToolbar
              categories={t.modelsCatalog.categories}
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
                <StateNotice>{t.modelsCatalog.empty}</StateNotice>
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
