"use client";

import { ImageAspectRatioSelector } from "@/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector";
import { ImageQualitySelector } from "@/features/image-generation/ImageQualitySelector/ImageQualitySelector";
import { ImageOutputCountSelector } from "@/features/image-generation/ImageOutputCountSelector/ImageOutputCountSelector";
import { ImageTemplatePicker } from "@/features/image-generation/ImageTemplatePicker/ImageTemplatePicker";
import { ru } from "@/i18n/ru";

export type ImageGenerationControlsProps = {
  modelID?: string;
  qualityLabel?: string;
  showOutputCount?: boolean;
  allowedAspectRatios?: string[];
  aspectRatio: string;
  imageQuality: string;
  isSubmitting: boolean;
  maxOutputCount: number;
  onAspectRatioChange: (ratio: string) => void;
  onImageQualityChange: (quality: string) => void;
  onOutputCountChange: (count: number) => void;
  onPromptChange: (prompt: string) => void;
  outputCount: number;
  qualityOptions: string[];
};

export function ImageGenerationControls(props: Readonly<ImageGenerationControlsProps>) {
  return (
    <>
      <ImageTemplatePicker disabled={props.isSubmitting} onSelect={(template) => props.onPromptChange(template.prompt)} />
      <ImageAspectRatioSelector disabled={props.isSubmitting} onChange={props.onAspectRatioChange} value={props.aspectRatio} options={props.allowedAspectRatios} />
      {props.qualityOptions.length > 1 ? <ImageQualitySelector
        disabled={props.isSubmitting}
        label={props.qualityLabel ?? ru.imageGeneration.resolutionLabel}
        onChange={props.onImageQualityChange}
        options={props.qualityOptions}
        value={props.imageQuality}
      /> : null}
      {props.showOutputCount !== false ? <ImageOutputCountSelector
        disabled={props.isSubmitting}
        max={props.maxOutputCount}
        onChange={props.onOutputCountChange}
        value={props.outputCount}
      /> : null}
    </>
  );
}
