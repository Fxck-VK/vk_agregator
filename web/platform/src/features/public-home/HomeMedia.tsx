"use client";

import { useId, useSyncExternalStore } from "react";
import { homeMediaBriefsEn, homePresentationEn, type HomeMediaBriefKey } from "@/i18n/public/home-media-en";
import { usePublicCopy } from "@/i18n/public/use-public-copy";
import styles from "./PublicHome.module.css";

const reducedQuery = "(prefers-reduced-motion: reduce)";
const subscribe = (listener: () => void) => { const media = window.matchMedia?.(reducedQuery); media?.addEventListener("change", listener); return () => media?.removeEventListener("change", listener); };
export function useLandingReducedMotion() { return useSyncExternalStore(subscribe, () => window.matchMedia?.(reducedQuery).matches ?? false, () => false); }

export function HomeMediaPrompt({ brief, className = "" }: { brief: HomeMediaBriefKey; className?: string }) {
  const content = usePublicCopy(homeMediaBriefsEn[brief]);
  const labels = usePublicCopy(homePresentationEn);
  const captionId = useId();
  return <figure className={`${styles.mediaPrompt} ${className}`} data-media-brief={brief} aria-labelledby={captionId}>
    <figcaption id={captionId}><span className={styles.promptLabel}>{labels.promptLabel}</span><strong className={styles.promptTitle}>{content.title}</strong></figcaption>
    <p>{content.prompt}</p><span className={styles.promptFormat}>{labels.formatLabel} {content.format}</span>
  </figure>;
}
