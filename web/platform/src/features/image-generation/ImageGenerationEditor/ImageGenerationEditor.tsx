"use client";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import styles from "./ImageGenerationEditor.module.css";

type ImageGenerationEditorProps = {
  canSubmit: boolean;
  errorMessage: string | null;
  imageQuality: string;
  isSubmitting: boolean;
  modelID: string;
  models: ImageModel[];
  onImageQualityChange: (quality: string) => void;
  onModelChange: (modelID: string) => void;
  onPromptChange: (prompt: string) => void;
  onSubmit: () => void;
  price: number | null;
  prompt: string;
};

export function ImageGenerationEditor({
  canSubmit,
  errorMessage,
  imageQuality,
  isSubmitting,
  modelID,
  models,
  onImageQualityChange,
  onModelChange,
  onPromptChange,
  onSubmit,
  price,
  prompt,
}: Readonly<ImageGenerationEditorProps>) {
  const selectedModel = models.find((model) => model.id === modelID) ?? null;
  const qualities = selectedModel?.quality_options ?? [];

  return (
    <form
      className={styles.editor}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <label>
        <span>{ru.imageGeneration.modelLabel}</span>
        <select disabled={isSubmitting} onChange={(event) => onModelChange(event.target.value)} value={modelID}>
          {models.map((model) => (
            <option key={model.id} value={model.id}>
              {model.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        <span>{ru.imageGeneration.qualityLabel}</span>
        <select
          disabled={isSubmitting || qualities.length === 0}
          onChange={(event) => onImageQualityChange(event.target.value)}
          value={imageQuality}
        >
          {qualities.map((quality) => (
            <option key={quality} value={quality}>
              {quality}
            </option>
          ))}
        </select>
      </label>
      <label className={styles.promptField}>
        <span>{ru.imageGeneration.promptLabel}</span>
        <textarea
          disabled={isSubmitting}
          onChange={(event) => onPromptChange(event.target.value)}
          placeholder={ru.imageGeneration.promptPlaceholder}
          required
          rows={6}
          value={prompt}
        />
      </label>
      {price === null ? (
        <p className={styles.priceUnavailable} role="status">{ru.imageGeneration.priceUnavailable}</p>
      ) : (
        <p className={styles.price} role="status">
          {ru.imageGeneration.priceLabel}: {formatStars(price)}
        </p>
      )}
      <Button disabled={!canSubmit || isSubmitting} type="submit">
        {isSubmitting ? ru.imageGeneration.preparing : generateLabel(price)}
      </Button>
      {errorMessage !== null ? (
        <p className={styles.error} role="alert">
          {errorMessage}
        </p>
      ) : null}
    </form>
  );
}

function formatStars(value: number): string {
  return `${value} \u2605`;
}

function generateLabel(price: number | null): string {
  return price === null ? ru.imageGeneration.generate : `${ru.imageGeneration.generate} · ${formatStars(price)}`;
}
