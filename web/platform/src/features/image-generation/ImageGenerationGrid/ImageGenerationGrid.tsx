"use client";

import type { CSSProperties, ReactNode } from "react";
import { MediaState } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";
import styles from "./ImageGenerationGrid.module.css";

export type PendingImagePreview = { count: number; aspectRatio: string };

export function imageAspectRatio(value: string) {
  const parts = /^(\d+(?:\.\d+)?):(\d+(?:\.\d+)?)$/.exec(value);
  const ratio = parts ? Number(parts[1]) / Number(parts[2]) : 1;
  return Number.isFinite(ratio) && ratio > 0 ? ratio : 1;
}

export function ImageGenerationGrid({ count, aspectRatio = "1:1", children }: Readonly<{
  count: number; aspectRatio?: string; children: ReactNode;
}>) {
  return <div className={styles.grid} data-ui="image-generation-grid" data-count={count}
    style={{ "--generation-aspect-ratio": imageAspectRatio(aspectRatio) } as CSSProperties}>
    {children}
  </div>;
}

export function ImageGenerationPlaceholders({ count, aspectRatio }: Readonly<PendingImagePreview>) {
  const msg = useMessages();
  return <ImageGenerationGrid count={count} aspectRatio={aspectRatio}>
    {Array.from({ length: count }, (_, index) => <div className={styles.placeholder} key={index}>
      <MediaState label={msg("imageGeneration.pendingImage", { value1: index + 1, value2: count })} />
    </div>)}
  </ImageGenerationGrid>;
}
