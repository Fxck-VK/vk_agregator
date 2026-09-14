"use client";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { ru } from "@/i18n/ru";

import { ImageGenerationControls, type ImageGenerationControlsProps } from "./ImageGenerationControls";

import styles from "./ImageGenerationComposer.module.css";

type ImageGenerationComposerProps = ImageGenerationControlsProps & {
  access?: "authenticated" | "guest";
  canSubmit: boolean;
  errorMessage: string | null;
  onSubmit: () => void;
  price: number | null;
  priceNote?: string;
  prompt: string;
};

export function ImageGenerationComposer({
  access = "authenticated",
  modelID,
  qualityLabel,
  showOutputCount,
  allowedAspectRatios,
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
  priceNote,
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
          <ImageGenerationControls
            modelID={modelID}
            qualityLabel={qualityLabel}
            showOutputCount={showOutputCount}
            allowedAspectRatios={allowedAspectRatios}
            aspectRatio={aspectRatio}
            imageQuality={imageQuality}
            isSubmitting={isSubmitting}
            maxOutputCount={maxOutputCount}
            onAspectRatioChange={onAspectRatioChange}
            onImageQualityChange={onImageQualityChange}
            onOutputCountChange={onOutputCountChange}
            onPromptChange={onPromptChange}
            outputCount={outputCount}
            qualityOptions={qualityOptions}
          />
        )}
        canSubmit={canSubmit}
        disabled={isSubmitting}
        label={ru.imageGeneration.promptLabel}
        mediaLabel="Загрузить медиа"
        mediaLibraryEnabled={access === "authenticated"}
        generatedMediaHref={access === "guest" ? "/login" : undefined}
        uploadedMediaHref={access === "guest" ? "/login" : undefined}
        note={priceNote ?? (price === null
          ? ru.imageGeneration.priceUnavailable
          : <CreditAmount prefix={`${ru.imageGeneration.priceLabel}:`} value={price} />)}
        onChange={(event) => onPromptChange(event.target.value)}
        onSend={onSubmit}
        placeholder={ru.imageGeneration.promptPlaceholder}
        submitLabel={isSubmitting ? ru.imageGeneration.preparing : ru.imageGeneration.generate}
        value={prompt}
        variant="hero"
        wrapLeadingControls
      />
      {errorMessage === null ? null : (
        <p className={styles.error} role="alert">{errorMessage}</p>
      )}
    </form>
  );
}
