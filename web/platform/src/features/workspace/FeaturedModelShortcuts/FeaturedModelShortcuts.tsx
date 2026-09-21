"use client";

import { Skeleton, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";


import { useEffect } from "react";

import { type GenerationModel } from "@/features/models/generation-model-catalog";
import { getModelPresentation } from "@/features/models/ModelCard/model-card-content";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";

import { useGenerationCatalog } from "@/features/models/GenerationCatalogProvider";

import styles from "./FeaturedModelShortcuts.module.css";

const featuredModelShortcutLimit = 4;



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
  const msg = useMessages();
  const { catalog, status } = useGenerationCatalog();
  const chatModel = catalog?.items.find(model => model.id === catalog.default_model_id && isTextModel(model))
    ?? catalog?.items.find(isTextModel) ?? null;
  const models = catalog?.items.filter(isImageModel).slice(0, featuredModelShortcutLimit) ?? [];
  const loadState = status === "failure" ? "failed" : status;
  useEffect(() => { onTextModelLoad?.(chatModel); }, [chatModel, onTextModelLoad]);

  return <>
    {loadState === "failed" ? <StateNotice inline kind="error">{msg("featuredModels.theAiModelCatalogIsTemporarilyUnavailable")}</StateNotice> : null}
    {loadState === "ready" && chatModel !== null ? (
      <button
        aria-label={msg("featuredModelShortcuts.selectModelValue", { value1: chatModel.name })}
        aria-pressed={selectedModelId === chatModel.id}
        className={styles.shortcut}
        disabled={disabled}
        onClick={() => onSelect(chatModel)}
        type="button"
      >
        <ModelIcon className={styles.icon} src={getModelPresentation(chatModel, msg).artworkSrc} />
        <span>{chatModel.name}</span>
      </button>
    ) : null}
    {loadState === "loading" ? Array.from({ length: featuredModelShortcutLimit }, (_, index) => (
      <Skeleton
        className={styles.skeleton}
        data-testid="featured-model-shortcut-skeleton"
        key={index}
      />
    )) : models.map((model) => {
    const presentation = getModelPresentation(model, msg);

    return (
      <button
        aria-label={msg("featuredModelShortcuts.selectModelValue", { value1: model.name })}
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
