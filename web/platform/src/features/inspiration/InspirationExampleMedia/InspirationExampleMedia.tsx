"use client";

import Image from "next/image";
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
  videoProps?: InspirationVideoProps;
  videoRef?: Ref<HTMLVideoElement>;
};

export function InspirationExampleMedia({
  className,
  example,
  priority = false,
  videoProps,
  videoRef,
}: Readonly<InspirationExampleMediaProps>) {
  if (example.mediaType === "video") {
    return (
      <video
        {...videoProps}
        aria-label={example.mediaAlt}
        className={className}
        height={example.mediaHeight}
        ref={videoRef}
        width={example.mediaWidth}
      >
        <source src={example.mediaPath} type="video/mp4" />
        Ваш браузер не поддерживает воспроизведение видео.
      </video>
    );
  }

  return (
    <Image
      alt={example.mediaAlt}
      className={className}
      height={example.mediaHeight}
      priority={priority}
      sizes={INSPIRATION_IMAGE_SIZES}
      src={example.mediaPath}
      width={example.mediaWidth}
    />
  );
}
