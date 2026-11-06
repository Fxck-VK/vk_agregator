"use client";

import { useId, useSyncExternalStore } from "react";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { homeMediaBriefsEn, homePresentationEn, type HomeMediaBriefKey } from "./home-copy";
import { usePublicCopy } from "@/i18n/public/use-public-copy";
import { homeMediaAssets } from "./HomeMedia.assets";
import styles from "./PublicHome.module.css";

const reducedQuery = "(prefers-reduced-motion: reduce)";
const subscribe = (listener: () => void) => { const media = window.matchMedia?.(reducedQuery); media?.addEventListener("change", listener); return () => media?.removeEventListener("change", listener); };
export function useLandingReducedMotion() { return useSyncExternalStore(subscribe, () => window.matchMedia?.(reducedQuery).matches ?? false, () => false); }

export function HomeMediaPrompt({ brief, className = "" }: { brief: HomeMediaBriefKey; className?: string }) {
  const content = usePublicCopy(homeMediaBriefsEn[brief]);
  const labels = usePublicCopy(homePresentationEn);
  const captionId = useId();
  const artwork = homeMediaAssets[brief];
  if (artwork) {
    const loading = brief === "hero" ? "eager" : "lazy";
    return <figure
      className={`${styles.mediaPrompt} ${styles.mediaArtwork} ${className}`}
      data-media-brief={brief} data-brief={brief} aria-labelledby={captionId}
      style={{ aspectRatio: `${artwork.width} / ${artwork.height}` }}
    >
      <figcaption id={captionId} className={styles.artworkCaption}>{content.title}</figcaption>
      <MediaImage src={artwork.src} alt={content.title} width={artwork.width} height={artwork.height}
        srcSet={artwork.srcSet} sizes={artwork.sizes}
        fit={brief === "hero" ? "cover" : "contain"} passive loading={loading} fetchPriority={brief === "hero" ? "high" : "auto"} />
    </figure>;
  }
  return <figure className={`${styles.mediaPrompt} ${className}`} data-media-brief={brief} aria-labelledby={captionId}>
    <figcaption id={captionId}><span className={styles.promptLabel}>{labels.promptLabel}</span><strong className={styles.promptTitle}>{content.title}</strong></figcaption>
    <p>{content.prompt}</p><span className={styles.promptFormat}>{labels.formatLabel} {content.format}</span>
  </figure>;
}
