"use client";

import Image from "next/image";
import { useEffect, useRef, useSyncExternalStore } from "react";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import styles from "./PublicHome.module.css";

export const homeMediaPath = "/assets/images/marketing/";
const reducedQuery = "(prefers-reduced-motion: reduce)";
const subscribe = (listener: () => void) => { const media = window.matchMedia?.(reducedQuery); media?.addEventListener("change", listener); return () => media?.removeEventListener("change", listener); };
export function useLandingReducedMotion() { return useSyncExternalStore(subscribe, () => window.matchMedia?.(reducedQuery).matches ?? false, () => false); }

export function HomePicture({ file, alt = "", sizes = "(max-width: 767px) 92vw, 45vw", preload = false, className }: { file: string; alt?: string; sizes?: string; preload?: boolean; className?: string }) {
  return <Image src={`${homeMediaPath}${file}`} alt={alt} fill sizes={sizes} preload={preload} className={className} />;
}

export function HomeLoop({ file, poster, paused }: { file: string; poster: string; paused: boolean }) {
  const video = useRef<HTMLVideoElement>(null);
  const reduced = useLandingReducedMotion();
  useEffect(() => {
    if (paused || reduced) video.current?.pause();
    else if (video.current?.readyState && !document.hidden) void video.current.play().catch(() => undefined);
  }, [paused, reduced]);
  return <div className={styles.loop}><MediaVideo src={`/assets/videos/marketing/${file}`} poster={`${homeMediaPath}${poster}`} videoRef={video} loading="lazy" fit="cover" autoPlay={!paused && !reduced} muted playsInline loop preload="none" /></div>;
}
