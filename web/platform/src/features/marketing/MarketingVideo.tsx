"use client";

import { useEffect, useRef } from "react";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { useMarketingReducedMotion, useMarketingVisible } from "./useMarketingMotion";

export function MarketingVideo({ paused, kind = "image" }: { paused: boolean; kind?: "image" | "video" }) {
  const reduced = useMarketingReducedMotion();
  const video = useRef<HTMLVideoElement>(null);
  const visible = useMarketingVisible(video);
  const stopped = paused || reduced || !visible;
  useEffect(() => {
    if (!video.current?.getAttribute("src")) return;
    if (stopped) video.current.pause();
    else void video.current.play().catch(() => {});
  }, [stopped]);
  return <MediaVideo src={`/assets/videos/marketing/creative-suite-${kind}.webm`} poster={`/assets/images/marketing/${kind === "image" ? "product-shots" : "preview-1"}.webp`} videoRef={video} autoPlay={!stopped} onCanPlay={event => { if (stopped) event.currentTarget.pause(); else void event.currentTarget.play().catch(() => {}); }} muted playsInline loop controls={paused || reduced} fit="cover" loading="lazy" preload="none" passive width={1280} height={720} />;
}
