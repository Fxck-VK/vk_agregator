"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";

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
  const msg = useMessages();
  const t = useDictionary();
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
        label={t.imageGeneration.promptLabel}
        mediaLabel={msg("imageGenerationComposer.uploadMedia")}
        mediaLibraryEnabled={access === "authenticated"}
        generatedMediaHref={access === "guest" ? "/login" : undefined}
        uploadedMediaHref={access === "guest" ? "/login" : undefined}
        note={priceNote ?? (price === null
          ? t.imageGeneration.priceUnavailable
          : <CreditAmount prefix={`${t.imageGeneration.priceLabel}:`} value={price} />)}
        onChange={(event) => onPromptChange(event.target.value)}
        onSend={onSubmit}
        placeholder={t.imageGeneration.promptPlaceholder}
        submitLabel={isSubmitting ? t.imageGeneration.preparing : t.imageGeneration.generate}
        value={prompt}
        variant="hero"
        wrapLeadingControls
      />
      {errorMessage === null ? null : (
        <StateNotice inline kind="error">{errorMessage}</StateNotice>
      )}
    </form>
  );
}
