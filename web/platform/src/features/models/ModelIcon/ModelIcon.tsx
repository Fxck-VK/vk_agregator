"use client";

import Image from "next/image";
import { useState } from "react";

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
    >
      <svg fill="none" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg">
        <path
          d="M23 5v8M32 5v8M41 5v8M23 51v8M32 51v8M41 51v8M5 23h8M5 32h8M5 41h8M51 23h8M51 32h8M51 41h8"
          stroke="currentColor"
          strokeLinecap="round"
          strokeWidth="6"
        />
        <rect fill="currentColor" height="40" rx="10" width="40" x="12" y="12" />
        <circle className={styles.faceCutout} cx="25" cy="29" r="3" />
        <circle className={styles.faceCutout} cx="39" cy="29" r="3" />
        <path
          className={styles.faceStroke}
          d="M23 39c2.5 3 5.5 4.5 9 4.5s6.5-1.5 9-4.5"
          fill="none"
          strokeLinecap="round"
          strokeWidth="3.5"
        />
      </svg>
    </span>
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
