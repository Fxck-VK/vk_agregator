"use client";

import { useEffect, useState } from "react";

import { loadGenerationModelCatalog, type GenerationModel } from "@/features/models/generation-model-catalog";
import { getModelPresentation } from "@/features/models/ModelCard/model-card-content";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";

import styles from "./FeaturedModelShortcuts.module.css";

const featuredModelShortcutLimit = 4;

type LoadState = "loading" | "ready" | "failed";

type FeaturedModelShortcutsProps = {
  selectedModelId: string | null;
  onSelect: (model: GenerationModel) => void;
  onTextModelLoad?: (model: GenerationModel | null) => void;
  disabled?: boolean;
};

function hasCategory(model: GenerationModel, category: string) {
  return model.categories?.includes(category) ?? false;
}

function isImageModel(model: GenerationModel) {
  return hasCategory(model, "images");
}

function isTextModel(model: GenerationModel) {
  return hasCategory(model, "text");
}

export function FeaturedModelShortcuts({ selectedModelId, onSelect, onTextModelLoad, disabled = false }: FeaturedModelShortcutsProps) {
  const [chatModel, setChatModel] = useState<GenerationModel | null>(null);
  const [models, setModels] = useState<GenerationModel[]>([]);
  const [loadState, setLoadState] = useState<LoadState>("loading");

  useEffect(() => {
    let active = true;

    void loadGenerationModelCatalog()
      .then((catalog) => {
        if (!active) return;
        const nextChatModel =
          catalog.items.find((model) => model.id === catalog.default_model_id && isTextModel(model))
          ?? catalog.items.find(isTextModel)
          ?? null;
        setChatModel(nextChatModel);
        onTextModelLoad?.(nextChatModel);
        setModels(catalog.items.filter(isImageModel).slice(0, featuredModelShortcutLimit));
        setLoadState("ready");
      })
      .catch(() => {
        if (!active) return;
        onTextModelLoad?.(null);
        setLoadState("failed");
      });

    return () => {
      active = false;
    };
  }, [onTextModelLoad]);

  return <>
    {loadState === "ready" && chatModel !== null ? (
      <button
        aria-label={`Выбрать модель: ${chatModel.name}`}
        aria-pressed={selectedModelId === chatModel.id}
        className={styles.shortcut}
        disabled={disabled}
        onClick={() => onSelect(chatModel)}
        type="button"
      >
        <ModelIcon className={styles.icon} src={getModelPresentation(chatModel).artworkSrc} />
        <span>{chatModel.name}</span>
      </button>
    ) : null}
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
