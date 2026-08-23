"use client";

import { useState } from "react";

import styles from "./VideoPlayer.module.css";

export type VideoSource = {
  src: string;
  type: "application/vnd.apple.mpegurl" | "video/mp4";
};

type VideoPlayerProps = {
  poster?: string;
  source?: VideoSource;
  title: string;
};

export function VideoPlayer({ poster, source, title }: Readonly<VideoPlayerProps>) {
  const [hasPlaybackError, setHasPlaybackError] = useState(false);

  if (source && !hasPlaybackError) {
    return (
      <div className={styles.frame}>
        <video
          aria-label={title}
          className={styles.video}
          controls
          onError={() => setHasPlaybackError(true)}
          poster={poster}
          preload="none"
        >
          <source src={source.src} type={source.type} />
          Ваш браузер не поддерживает воспроизведение видео.
        </video>
      </div>
    );
  }

  const placeholder = hasPlaybackError ? "Видео временно недоступно" : "Видео скоро появится";

  return (
    <div
      aria-label={hasPlaybackError ? undefined : title}
      className={styles.frame}
      role={hasPlaybackError ? "status" : "img"}
    >
      <span aria-hidden="true" className={styles.playIcon}>▶</span>
      <span className={styles.placeholder}>{placeholder}</span>
    </div>
  );
}
