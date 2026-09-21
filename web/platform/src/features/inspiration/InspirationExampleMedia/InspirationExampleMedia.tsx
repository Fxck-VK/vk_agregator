"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import type { ComponentPropsWithoutRef, Ref } from "react";

import type { InspirationExample } from "../inspiration-examples";

export const INSPIRATION_IMAGE_SIZES = "(max-width: 48rem) 100vw, 48rem";

type InspirationVideoProps = Pick<
  ComponentPropsWithoutRef<"video">,
  "controls" | "loop" | "muted" | "playsInline" | "preload"
>;

type InspirationExampleMediaProps = {
  className?: string;
  example: InspirationExample;
  priority?: boolean;
  fit?: "contain" | "cover";
  passive?: boolean;
  videoProps?: InspirationVideoProps;
  videoRef?: Ref<HTMLVideoElement>;
};

export function InspirationExampleMedia({
  className,
  example,
  priority = false,
  fit = "cover",
  passive = true,
  videoProps,
  videoRef,
}: Readonly<InspirationExampleMediaProps>) {
  const msg = useMessages();
  if (example.mediaType === "video" && !(videoProps?.preload === "none" && example.posterPath)) {
    return (
      <MediaVideo fit={fit} passive={passive} src={example.mediaPath} poster={example.posterPath}
        loading={passive ? "lazy" : "eager"}
        {...videoProps}
        aria-label={example.mediaAlt}
        className={className}
        height={example.mediaHeight}
        videoRef={videoRef}
        width={example.mediaWidth}
      >
        {msg("inspirationExampleMedia.yourBrowserDoesNotSupportVideoPlayback")}</MediaVideo>
    );
  }

  return (
    <MediaImage optimized fit={fit} passive={passive}
      alt={example.mediaAlt}
      className={className}
      height={example.mediaHeight}
      priority={priority}
      sizes={INSPIRATION_IMAGE_SIZES}
      src={example.mediaType === "video" ? example.posterPath! : example.mediaPath}
      width={example.mediaWidth}
    />
  );
}
