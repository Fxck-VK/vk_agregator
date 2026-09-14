"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { ModelCard } from "@/features/models/ModelCard/ModelCard";
import { loadGenerationModelCatalog, type GenerationModel } from "@/features/models/generation-model-catalog";

import styles from "./FeaturedModels.module.css";

const collapsedModelLimit = 4;
const expandedModelLimit = 6;

type LoadState = "loading" | "ready" | "failed";

function CatalogActionContent({ label }: { label: string }) {
  return (
    <>
      <Image
        alt=""
        className={styles.catalogActionBackground}
        fill
        sizes="12rem"
        src={assetPaths.images.workspace.allModelsButtonBackground}
      />
      <span className={styles.catalogActionLabel}>{label}</span>
    </>
  );
}

export function FeaturedModels() {
  const [models, setModels] = useState<GenerationModel[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    let active = true;

    void loadGenerationModelCatalog()
      .then((catalog) => {
        if (!active) return;
        setModels(catalog.items.slice(0, expandedModelLimit));
        setLoadState("ready");
      })
      .catch(() => {
        if (!active) return;
        setLoadState("failed");
      });

    return () => {
      active = false;
    };
  }, []);

  if (loadState === "loading") {
    return (
      <div aria-hidden="true" className={styles.grid}>
        {Array.from({ length: collapsedModelLimit }, (_, index) => (
          <div className={styles.skeletonCard} key={index} />
        ))}
      </div>
    );
  }

  if (loadState === "failed" || models.length === 0) {
    return <p className={styles.empty}>Каталог нейросетей временно недоступен.</p>;
  }

  const canExpand = models.length > collapsedModelLimit;
  const visibleModels = models.slice(0, expanded ? expandedModelLimit : collapsedModelLimit);

  return (
    <>
      <div className={styles.grid} id="featured-models-grid">
        {visibleModels.map((model, index) => {
          const isNewlyRevealed = expanded && index >= collapsedModelLimit;

          return (
            <ModelCard
              className={isNewlyRevealed ? styles.revealedCard : undefined}
              key={model.id}
              model={model}
              revealed={isNewlyRevealed}
              testId="featured-model-card"
            />
          );
        })}
      </div>

      <div className={styles.actions}>
        {canExpand && !expanded ? (
          <button
            aria-controls="featured-models-grid"
            aria-expanded={expanded}
            className={styles.catalogAction}
            onClick={() => setExpanded(true)}
            type="button"
          >
            <CatalogActionContent label="Показать ещё" />
          </button>
        ) : (
          <Link
            className={`${styles.catalogAction} ${expanded ? styles.revealedCatalogAction : ""}`}
            data-revealed={expanded || undefined}
            href="/app/models"
          >
            <CatalogActionContent label="Все нейросети" />
          </Link>
        )}
      </div>
    </>
  );
}
