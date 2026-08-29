"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import type { ImageModel } from "@/lib/web-api/contracts";

import styles from "./FeaturedModels.module.css";

const collapsedModelLimit = 4;
const expandedModelLimit = 6;

const featuredModelDescriptions: Readonly<Record<string, string>> = {
  "Nano Banana 2": "Быстрая генерация и редактирование изображений для повседневных задач",
  "Nano Banana Pro": "Детализированные изображения для сложных творческих и рабочих задач",
  "GPT Image 2": "Точное создание изображений по описанию с хорошей передачей текста",
  "Seedream 4.5": "Фотореалистичные изображения с высокой детализацией и выразительным стилем",
};

type LoadState = "loading" | "ready" | "failed";

function getMinimumPrice(model: ImageModel): number | null {
  const prices = Object.values(model.price_by_quality ?? {});
  return prices.length > 0 ? Math.min(...prices) : null;
}

function getModelDescription(model: ImageModel): string {
  return featuredModelDescriptions[model.name] ?? `${model.name} для создания изображений по вашему описанию`;
}

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
  const [models, setModels] = useState<ImageModel[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    let active = true;

    void loadImageModelCatalog()
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
          <div className={`${styles.card} ${styles.skeleton}`} key={index} />
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
        {visibleModels.map((model) => {
          const minimumPrice = getMinimumPrice(model);

          return (
            <Link
              className={styles.card}
              data-testid="featured-model-card"
              href={`/app/image?model=${encodeURIComponent(model.id)}`}
              key={model.id}
              prefetch={false}
            >
              <span className={styles.cardTop}>
                <ModelIcon />
                {minimumPrice !== null ? <CreditAmount className={styles.price} value={minimumPrice} /> : null}
              </span>
              <span className={styles.copy}>
                <strong>{model.name}</strong>
                <span>{getModelDescription(model)}</span>
              </span>
            </Link>
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
          <Link className={styles.catalogAction} href="/app/models">
            <CatalogActionContent label="Все нейросети" />
          </Link>
        )}
      </div>
    </>
  );
}
