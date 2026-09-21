"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { useState } from "react";
import type { GenerationOptions } from "./generation-options-contract";
export type { GenerationOptions } from "./generation-options-contract";
import { ImageGenerationControls } from "@/features/image-generation/ImageGenerationComposer/ImageGenerationControls";
import { ImageAspectRatioSelector } from "@/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector";
import { ImageQualitySelector } from "@/features/image-generation/ImageQualitySelector/ImageQualitySelector";
import type { GenerationModel } from "./generation-model-catalog";

function videoAspectRatios(model: Extract<GenerationModel, { category: "video" }>, options: GenerationOptions) {
 return model.allowed_aspect_ratios.filter(ratio => !model.variants || model.variants.some(variant =>
  variant.resolution === options.resolution && variant.duration_sec === options.duration_sec && variant.aspect_ratio === ratio));
}

export function resolveGenerationOptions(model: GenerationModel | undefined, saved: GenerationOptions = {}) {
 if (model?.category === "images") {
  const image_quality = model.quality_options.includes(saved.image_quality ?? "") ? saved.image_quality! : model.default_quality;
  const ratios = (model.allowed_aspect_ratios ?? []).filter(ratio => !model.price_by_variant || model.price_by_variant[`${image_quality}:${ratio}`] > 0);
  const aspect_ratio = ratios.includes(saved.aspect_ratio ?? "") ? saved.aspect_ratio! : ratios.includes(model.default_aspect_ratio ?? "") ? model.default_aspect_ratio! : ratios[0];
  const output_count = Math.min(Math.max(saved.output_count ?? 1,1),model.max_output_count ?? 1);
  const unitPrice = model.price_by_variant ? model.price_by_variant[`${image_quality}:${aspect_ratio}`] : model.price_by_quality?.[image_quality];
  return {options:{image_quality,aspect_ratio,output_count},cost:unitPrice === undefined ? undefined : unitPrice*output_count};
 }
 if (model?.category === "video") {
  const resolution = model.allowed_resolutions.includes(saved.resolution ?? "") ? saved.resolution! : model.default_resolution;
  const durations = model.allowed_durations_sec.filter(duration => model.price_by_option[`${resolution}:${duration}`] > 0 && (!model.variants || model.variants.some(variant => variant.resolution === resolution && variant.duration_sec === duration)));
  const duration_sec = durations.includes(saved.duration_sec ?? 0) ? saved.duration_sec! : durations.includes(model.default_duration_sec) ? model.default_duration_sec : durations[0];
  const ratios = videoAspectRatios(model, { resolution, duration_sec });
  const aspect_ratio = ratios.includes(saved.aspect_ratio ?? "") ? saved.aspect_ratio! : ratios.includes(model.default_aspect_ratio) ? model.default_aspect_ratio : ratios[0];
  return {options:{resolution,duration_sec,aspect_ratio},cost:model.price_by_option[`${resolution}:${duration_sec}`]};
 }
 return {options:{},cost: model?.category === "text" ? model.estimate_credits : undefined};
}

export function useGenerationControls(model: GenerationModel | undefined, disabled: boolean, onPromptChange: (prompt:string)=>void) {
  const msg = useMessages();
 const [byModel,setByModel] = useState<Record<string,GenerationOptions>>({});
 const {options,cost} = resolveGenerationOptions(model,model ? byModel[model.id] : undefined);
 const change = (patch:GenerationOptions) => {
  if (model) setByModel(current=>({...current,[model.id]:{...options,...patch}}));
 };
 let controls = null;
 if (model?.category === "images") controls = <ImageGenerationControls
  modelID={model.id} qualityLabel={model.quality_label} showOutputCount={model.show_output_count} allowedAspectRatios={model.allowed_aspect_ratios?.filter(ratio=>!model.price_by_variant || model.price_by_variant[`${options.image_quality}:${ratio}`]>0)} aspectRatio={options.aspect_ratio!}
  imageQuality={options.image_quality!} qualityOptions={model.quality_options} outputCount={options.output_count!}
  maxOutputCount={model.max_output_count ?? 1} isSubmitting={disabled}
  onAspectRatioChange={aspect_ratio=>change({aspect_ratio})} onImageQualityChange={image_quality=>change({image_quality})}
  onOutputCountChange={output_count=>change({output_count})} onPromptChange={onPromptChange}
 />;
 if (model?.category === "video") controls = <>
  <ImageAspectRatioSelector disabled={disabled} value={options.aspect_ratio!} options={videoAspectRatios(model, options)} onChange={aspect_ratio=>change({aspect_ratio})}/>
  <ImageQualitySelector disabled={disabled} label={msg("generationOptions.resolution")} value={options.resolution!} options={model.allowed_resolutions} onChange={resolution=>change({resolution})}/>
  <ImageQualitySelector disabled={disabled} label={msg("generationOptions.duration")} value={msg("generationOptions.valueS", { value1: options.duration_sec ?? "—" })} options={model.allowed_durations_sec.filter(value=>model.price_by_option[`${options.resolution}:${value}`] > 0 && (!model.variants || model.variants.some(variant=>variant.resolution===options.resolution && variant.duration_sec===value))).map(value=>msg("generationOptions.valueS", { value1: value }))} onChange={value=>change({duration_sec:parseInt(value,10)})}/>
 </>;
 return {options,controls,cost,canSubmit:model !== undefined && (model.category === "text" || (cost ?? 0)>0)};
}
