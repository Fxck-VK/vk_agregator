"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import type { ImageModel } from "@/lib/web-api/contracts";

import styles from "./FeaturedModels.module.css";

const featuredModelLimit = 4;

type LoadState = "loading" | "ready" | "failed";

function getMinimumPrice(model: ImageModel): number | null {
  const prices = Object.values(model.price_by_quality ?? {});
  return prices.length > 0 ? Math.min(...prices) : null;
}

function getModelDescription(model: ImageModel): string {
  return model.supports_reference_image
    ? "Создание и редактирование изображений по запросу и референсам"
    : "Генерация изображений по текстовому запросу";
}

export function FeaturedModels() {
  const [models, setModels] = useState<ImageModel[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");

  useEffect(() => {
    let active = true;

    void loadImageModelCatalog()
      .then((catalog) => {
        if (!active) return;
        setModels(catalog.items.slice(0, featuredModelLimit));
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
        {Array.from({ length: featuredModelLimit }, (_, index) => (
          <div className={`${styles.card} ${styles.skeleton}`} key={index} />
        ))}
      </div>
    );
  }

  if (loadState === "failed" || models.length === 0) {
    return <p className={styles.empty}>Каталог нейросетей временно недоступен.</p>;
  }

  return (
    <div className={styles.grid}>
      {models.map((model) => {
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
              {minimumPrice !== null ? <CreditAmount className={styles.price} prefix="от" value={minimumPrice} /> : null}
            </span>
            <span className={styles.copy}>
              <strong>{model.name}</strong>
              <span>{getModelDescription(model)}</span>
            </span>
            <span className={styles.facts}>
              <span>{model.quality_options.join(" · ")}</span>
              <span>{model.supports_reference_image ? "Поддерживает референсы" : "По текстовому запросу"}</span>
            </span>
          </Link>
        );
      })}
    </div>
  );
}
