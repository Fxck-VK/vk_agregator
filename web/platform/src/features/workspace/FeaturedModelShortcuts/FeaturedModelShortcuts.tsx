"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import styles from "./FeaturedModelShortcuts.module.css";

const featuredModelShortcutLimit = 4;

type LoadState = "loading" | "ready" | "failed";

type FeaturedModelShortcutsProps = {
  artworkByModelId?: Readonly<Record<string, string>>;
};

export function FeaturedModelShortcuts({ artworkByModelId = {} }: FeaturedModelShortcutsProps) {
  const [models, setModels] = useState<ImageModel[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");

  useEffect(() => {
    let active = true;

    void loadImageModelCatalog()
      .then((catalogue) => {
        if (!active) return;
        setModels(catalogue.items.slice(0, featuredModelShortcutLimit));
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
    return Array.from({ length: featuredModelShortcutLimit }, (_, index) => (
      <span
        aria-hidden="true"
        className={styles.skeleton}
        data-testid="featured-model-shortcut-skeleton"
        key={index}
      />
    ));
  }

  if (loadState === "failed" || models.length === 0) {
    return null;
  }

  return models.map((model) => (
    <Link
      aria-label={`${ru.modelsCatalog.openGeneratorLabel}: ${model.name}`}
      className={styles.shortcut}
      data-testid="featured-model-shortcut"
      href={`/app/image?model=${encodeURIComponent(model.id)}`}
      key={model.id}
      prefetch={false}
    >
      <ModelIcon className={styles.icon} src={artworkByModelId[model.id]} />
      <span>{model.name}</span>
    </Link>
  ));
}
