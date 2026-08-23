"use client";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { ImageAspectRatioSelector } from "@/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector";
import { ru } from "@/i18n/ru";

import styles from "./ImageGenerationComposer.module.css";

type ImageGenerationComposerProps = {
  aspectRatio: string;
  canSubmit: boolean;
  errorMessage: string | null;
  imageQuality: string;
  isSubmitting: boolean;
  onAspectRatioChange: (ratio: string) => void;
  onImageQualityChange: (quality: string) => void;
  onPromptChange: (prompt: string) => void;
  onSubmit: () => void;
  price: number | null;
  prompt: string;
  qualityOptions: string[];
};

export function ImageGenerationComposer({
  aspectRatio,
  canSubmit,
  errorMessage,
  imageQuality,
  isSubmitting,
  onAspectRatioChange,
  onImageQualityChange,
  onPromptChange,
  onSubmit,
  price,
  prompt,
  qualityOptions,
}: Readonly<ImageGenerationComposerProps>) {
  return (
    <form
      className={styles.root}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <ChatComposer
        additionalControls={(
          <>
            <ImageAspectRatioSelector disabled={isSubmitting} onChange={onAspectRatioChange} value={aspectRatio} />
            <label className={styles.qualityControl}>
              <span className={styles.visuallyHidden}>{ru.imageGeneration.qualityLabel}</span>
              <span aria-hidden="true" className={styles.qualityIcon}>⌁</span>
              <select
                aria-label={ru.imageGeneration.qualityLabel}
                disabled={isSubmitting || qualityOptions.length === 0}
                onChange={(event) => onImageQualityChange(event.target.value)}
                value={imageQuality}
              >
                {qualityOptions.map((quality) => (
                  <option key={quality} value={quality}>{quality}</option>
                ))}
              </select>
            </label>
          </>
        )}
        canSubmit={canSubmit}
        disabled={isSubmitting}
        label={ru.imageGeneration.promptLabel}
        mediaLabel="Загрузить медиа"
        note={price === null
          ? ru.imageGeneration.priceUnavailable
          : `${ru.imageGeneration.priceLabel}: ${formatStars(price)}`}
        onChange={(event) => onPromptChange(event.target.value)}
        onSend={onSubmit}
        placeholder={ru.imageGeneration.promptPlaceholder}
        submitLabel={isSubmitting ? ru.imageGeneration.preparing : ru.imageGeneration.generate}
        value={prompt}
        variant="hero"
      />
      {errorMessage === null ? null : (
        <p className={styles.error} role="alert">{errorMessage}</p>
      )}
    </form>
  );
}

function formatStars(value: number): string {
  return `${value} \u2605`;
}
