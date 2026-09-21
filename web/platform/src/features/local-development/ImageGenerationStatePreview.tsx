"use client";

import { useState } from "react";
import { AttachmentPreviewDialog } from "@/components/media/AttachmentPreviewDialog/AttachmentPreviewDialog";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { ImageAspectRatioSelector } from "@/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector";
import { ImageOutputCountSelector } from "@/features/image-generation/ImageOutputCountSelector/ImageOutputCountSelector";
import { ImageGenerationGrid, ImageGenerationPlaceholders } from "@/features/image-generation/ImageGenerationGrid/ImageGenerationGrid";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import type { ImageJobResult } from "@/lib/web-api/contracts";
import styles from "./AsyncStatePreview.module.css";

export function ImageGenerationStatePreview({ state, result, onRetry }: Readonly<{
  state: string; result: ImageJobResult; onRetry: () => void;
}>) {
  const t = useDictionary();
  const msg = useMessages();
  const [count, setCount] = useState(4);
  const [aspectRatio, setAspectRatio] = useState("1:1");
  const [preview, setPreview] = useState<{ index: number; trigger: HTMLButtonElement } | null>(null);
  const items = Array.from({ length: count }, (_, index) => ({
    id: `example-${index}`,
    src: `/web/v1/image-artifacts/${result.artifacts[0].id}`,
    alt: `${t.files.generatedImageAlt} ${index + 1}`,
  }));

  return <section id="image-generation" aria-labelledby="image-generation-preview-title">
    <h2 id="image-generation-preview-title">{msg("localDevelopment.imageBatch")}</h2>
    <p className={styles.description}>{msg("localDevelopment.imageBatchDescription")}</p>
    <div className={styles.controls}>
      <ImageOutputCountSelector max={15} value={count} onChange={value => { setCount(value); setPreview(null); }} />
      <ImageAspectRatioSelector disabled={false} value={aspectRatio} onChange={setAspectRatio} />
    </div>
    {state === "loading" ? <ImageGenerationPlaceholders count={count} aspectRatio={aspectRatio} />
      : state === "error" ? <StateNotice kind="error" action={{ label: t.files.retry, onClick: onRetry }}>{t.imageGeneration.resultFailure}</StateNotice>
      : <ImageGenerationGrid count={count} aspectRatio={aspectRatio}>
        {items.map((item, index) => <div key={item.id} className={styles.batchImage}>
          <MediaImage src={item.src} alt={item.alt} fit="cover" action={{ label: item.alt, onClick: event => setPreview({ index, trigger: event.currentTarget }) }} />
        </div>)}
      </ImageGenerationGrid>}
    {preview ? <AttachmentPreviewDialog items={items} selectedIndex={preview.index} onSelect={index => setPreview({ ...preview, index })} onClose={() => setPreview(null)} returnFocusTo={preview.trigger} /> : null}
  </section>;
}
