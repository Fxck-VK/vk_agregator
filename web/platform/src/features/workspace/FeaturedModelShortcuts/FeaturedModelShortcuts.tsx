"use client";

import { useEffect, useState } from "react";

import { loadImageModelCatalog } from "@/features/models/image-model-catalog-cache";
import { getModelPresentation } from "@/features/models/ModelCard/model-card-content";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import type { ImageModel } from "@/lib/web-api/contracts";

import styles from "./FeaturedModelShortcuts.module.css";

const featuredModelShortcutLimit = 4;

type LoadState = "loading" | "ready" | "failed";

type FeaturedModelShortcutsProps = {
  selectedModelId: string | null;
  onSelect: (model: ImageModel | null) => void;
  disabled?: boolean;
};

export function FeaturedModelShortcuts({ selectedModelId, onSelect, disabled = false }: FeaturedModelShortcutsProps) {
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

  return <>
    <button
      aria-label="Выбрать модель: NeiroHub Chat"
      aria-pressed={selectedModelId === null}
      className={styles.shortcut}
      disabled={disabled}
      onClick={() => onSelect(null)}
      type="button"
    >
      <ModelIcon className={styles.icon} />
      <span>NeiroHub Chat</span>
    </button>
    {loadState === "loading" ? Array.from({ length: featuredModelShortcutLimit }, (_, index) => (
      <span
        aria-hidden="true"
        className={styles.skeleton}
        data-testid="featured-model-shortcut-skeleton"
        key={index}
      />
    )) : models.map((model) => {
    const presentation = getModelPresentation(model);

    return (
      <button
        aria-label={`Выбрать модель: ${model.name}`}
        aria-pressed={selectedModelId === model.id}
        className={styles.shortcut}
        data-testid="featured-model-shortcut"
        disabled={disabled}
        key={model.id}
        onClick={() => onSelect(model)}
        type="button"
      >
        <ModelIcon className={styles.icon} src={presentation.artworkSrc} />
        <span>{model.name}</span>
      </button>
    );
  })}
  </>;
}
