"use client";

import { useDeferredValue, useEffect, useMemo, useState } from "react";

import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { loadImageModelCatalog } from "../image-model-catalog-cache";
import { ModelCard } from "../ModelCard/ModelCard";
import { ModelCatalogToolbar } from "../ModelCatalogToolbar/ModelCatalogToolbar";
import { filterAndSortImageModels, imageModelQualities, type ImageModelSort } from "./model-filters";
import styles from "./ModelsCatalog.module.css";

type CatalogStatus = "loading" | "ready" | "failure";

export function ModelsCatalog() {
  const [status, setStatus] = useState<CatalogStatus>("loading");
  const [models, setModels] = useState<ImageModel[]>([]);
  const [query, setQuery] = useState("");
  const [referenceOnly, setReferenceOnly] = useState(false);
  const [quality, setQuality] = useState<string | null>(null);
  const [sort, setSort] = useState<ImageModelSort>("catalog");
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
    () => filterAndSortImageModels(models, { query: deferredQuery, referenceOnly, quality }, sort),
    [deferredQuery, models, quality, referenceOnly, sort],
  );
  const qualities = useMemo(() => imageModelQualities(models), [models]);

  const clearFilters = () => {
    setQuery("");
    setReferenceOnly(false);
    setQuality(null);
    setSort("catalog");
  };

  return (
    <section aria-labelledby="models-catalog-title" className={styles.catalog}>
      <header className={styles.header}>
        <p className={styles.eyebrow}>{ru.modelsCatalog.eyebrow}</p>
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
            onClear={clearFilters}
            onQualityChange={setQuality}
            onQueryChange={setQuery}
            onReferenceOnlyChange={setReferenceOnly}
            onSortChange={setSort}
            qualities={qualities}
            quality={quality}
            query={query}
            referenceOnly={referenceOnly}
            resultCount={filteredModels.length}
            sort={sort}
          />

          {filteredModels.length === 0 ? <p>{ru.modelsCatalog.empty}</p> : null}

          {filteredModels.length > 0 ? (
            <div className={styles.grid}>
              {filteredModels.map((model) => (
                <ModelCard key={model.id} model={model} />
              ))}
            </div>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
