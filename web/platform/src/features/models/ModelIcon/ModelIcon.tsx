"use client";

import Image from "next/image";
import { useState } from "react";

import { assetPaths } from "@/assets/asset-paths";

import styles from "./ModelIcon.module.css";

type ModelIconProps = {
  className?: string;
  src?: string | null;
};

function DefaultModelArtwork({ classNames }: Readonly<{ classNames: string }>) {
  return (
    <span
      aria-hidden="true"
      className={`${classNames} ${styles.fallback}`}
      data-testid="model-icon-fallback"
      style={{ backgroundImage: `url("${assetPaths.images.models.fallback}")` }}
    />
  );
}

export function ModelIcon({ className, src }: Readonly<ModelIconProps>) {
  const classNames = [styles.icon, className].filter(Boolean).join(" ");
  const [failedSource, setFailedSource] = useState<string | null>(null);

  if (!src || failedSource === src) {
    return <DefaultModelArtwork classNames={classNames} />;
  }

  return (
    <Image
      alt=""
      aria-hidden="true"
      className={classNames}
      data-testid="model-icon"
      height={245}
      onError={() => setFailedSource(src)}
      src={src}
      unoptimized
      width={205}
    />
  );
}
