"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";

import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { loadImageModelCatalog } from "../image-model-catalog-cache";
import { filterImageModels, imageModelQualities } from "./model-filters";
import styles from "./ModelsCatalog.module.css";

type CatalogStatus = "loading" | "ready" | "failure";

export function ModelsCatalog() {
  const [status, setStatus] = useState<CatalogStatus>("loading");
  const [models, setModels] = useState<ImageModel[]>([]);
  const [query, setQuery] = useState("");
  const [referenceOnly, setReferenceOnly] = useState(false);
  const [quality, setQuality] = useState<string | null>(null);

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
    () => filterImageModels(models, { query, referenceOnly, quality }),
    [models, quality, query, referenceOnly],
  );
  const qualities = useMemo(() => imageModelQualities(models), [models]);
  const hasActiveFilters = query.trim() !== "" || referenceOnly || quality !== null;

  const clearFilters = () => {
    setQuery("");
    setReferenceOnly(false);
    setQuality(null);
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
          <div className={styles.filters}>
            <input
              aria-label={ru.modelsCatalog.searchLabel}
              onChange={(event) => setQuery(event.target.value)}
              type="search"
              value={query}
            />
            <label className={styles.checkbox}>
              <input
                checked={referenceOnly}
                onChange={(event) => setReferenceOnly(event.target.checked)}
                type="checkbox"
              />
              {ru.modelsCatalog.referenceFilterLabel}
            </label>
            <select
              aria-label={ru.modelsCatalog.qualityFilterLabel}
              onChange={(event) => setQuality(event.target.value || null)}
              value={quality ?? ""}
            >
              <option value="">{ru.modelsCatalog.allQualitiesLabel}</option>
              {qualities.map((value) => (
                <option key={value} value={value}>
                  {value}
                </option>
              ))}
            </select>
            {hasActiveFilters ? (
              <button className={styles.clearFilters} onClick={clearFilters} type="button">
                {ru.modelsCatalog.clearFiltersLabel}
              </button>
            ) : null}
          </div>

          {filteredModels.length === 0 ? <p>{ru.modelsCatalog.empty}</p> : null}

          {filteredModels.length > 0 ? (
            <div className={styles.grid}>
              {filteredModels.map((model) => (
                <article className={styles.card} key={model.id}>
                  <h2>{model.name}</h2>
                  <p className={styles.type}>{ru.modelsCatalog.imageTypeLabel}</p>
                  <ul aria-label={ru.modelsCatalog.qualityFilterLabel} className={styles.qualities}>
                    {model.quality_options.map((value) => (
                      <li key={value}>{value}</li>
                    ))}
                  </ul>
                  <p className={styles.reference}>
                    {model.supports_reference_image
                      ? ru.modelsCatalog.referenceSupportedLabel
                      : ru.modelsCatalog.referenceUnsupportedLabel}
                  </p>
                  <Link
                    aria-label={`${ru.modelsCatalog.openGeneratorLabel}: ${model.name}`}
                    href={`/app/image?model=${encodeURIComponent(model.id)}`}
                  >
                    {ru.modelsCatalog.openGeneratorLabel}
                  </Link>
                </article>
              ))}
            </div>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
