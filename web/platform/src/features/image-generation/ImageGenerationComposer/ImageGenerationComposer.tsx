"use client";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ImageAspectRatioSelector } from "@/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector";
import { ImageQualitySelector } from "@/features/image-generation/ImageQualitySelector/ImageQualitySelector";
import { ImageOutputCountSelector } from "@/features/image-generation/ImageOutputCountSelector/ImageOutputCountSelector";
import { ImageTemplatePicker } from "@/features/image-generation/ImageTemplatePicker/ImageTemplatePicker";
import { ru } from "@/i18n/ru";

import styles from "./ImageGenerationComposer.module.css";

type ImageGenerationComposerProps = {
  aspectRatio: string;
  canSubmit: boolean;
  errorMessage: string | null;
  imageQuality: string;
  isSubmitting: boolean;
  maxOutputCount: number;
  onAspectRatioChange: (ratio: string) => void;
  onImageQualityChange: (quality: string) => void;
  onOutputCountChange: (count: number) => void;
  onPromptChange: (prompt: string) => void;
  onSubmit: () => void;
  price: number | null;
  outputCount: number;
  prompt: string;
  qualityOptions: string[];
};

export function ImageGenerationComposer({
  aspectRatio,
  canSubmit,
  errorMessage,
  imageQuality,
  isSubmitting,
  maxOutputCount,
  onAspectRatioChange,
  onImageQualityChange,
  onOutputCountChange,
  onPromptChange,
  onSubmit,
  price,
  outputCount,
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
        leadingControls={(
          <>
            <ImageTemplatePicker
              disabled={isSubmitting}
              onSelect={(template) => onPromptChange(template.prompt)}
            />
            <ImageAspectRatioSelector disabled={isSubmitting} onChange={onAspectRatioChange} value={aspectRatio} />
            <ImageQualitySelector
              disabled={isSubmitting}
              label={ru.imageGeneration.resolutionLabel}
              onChange={onImageQualityChange}
              options={qualityOptions}
              value={imageQuality}
            />
            <ImageOutputCountSelector
              disabled={isSubmitting}
              max={maxOutputCount}
              onChange={onOutputCountChange}
              value={outputCount}
            />
          </>
        )}
        canSubmit={canSubmit}
        disabled={isSubmitting}
        label={ru.imageGeneration.promptLabel}
        mediaLabel="Загрузить медиа"
        note={price === null
          ? ru.imageGeneration.priceUnavailable
          : <CreditAmount prefix={`${ru.imageGeneration.priceLabel}:`} value={price} />}
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
