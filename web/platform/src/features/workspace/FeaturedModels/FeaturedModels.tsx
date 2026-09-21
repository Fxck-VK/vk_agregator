"use client";

import { Skeleton, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";


import Image from "next/image";
import Link from "@/i18n/Link";
import { useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { ModelCard } from "@/features/models/ModelCard/ModelCard";
import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";

import styles from "./FeaturedModels.module.css";

const collapsedModelLimit = 4;
const expandedModelLimit = 6;



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
  const msg = useMessages();
  const t = useDictionary();
  const { catalog, status, retry } = useGenerationCatalog();
  const models = catalog?.items.slice(0, expandedModelLimit) ?? [];
  const loadState = status === "failure" ? "failed" : status;
  const [expanded, setExpanded] = useState(false);

  if (loadState === "loading") {
    return (
      <div aria-hidden="true" className={styles.grid}>
        {Array.from({ length: collapsedModelLimit }, (_, index) => (
          <Skeleton className={styles.skeletonCard} key={index} />
        ))}
      </div>
    );
  }

  if (loadState === "failed" || models.length === 0) {
    return <StateNotice kind="error" action={{ label: t.files.retry, onClick: retry }}>{msg("featuredModels.theAiModelCatalogIsTemporarilyUnavailable")}</StateNotice>;
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
            <CatalogActionContent label={msg("featuredModels.showMore")} />
          </button>
        ) : (
          <Link
            className={`${styles.catalogAction} ${expanded ? styles.revealedCatalogAction : ""}`}
            data-revealed={expanded || undefined}
            href="/app/models"
          >
            <CatalogActionContent label={msg("featuredModels.allAiModels")} />
          </Link>
        )}
      </div>
    </>
  );
}
