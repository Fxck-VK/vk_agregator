"use client";

import { useRef, useState } from "react";

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
  const [hasStarted, setHasStarted] = useState(false);
  const videoRef = useRef<HTMLVideoElement>(null);

  const startPlayback = () => {
    const video = videoRef.current;
    if (!video) {
      return;
    }

    setHasStarted(true);
    video.play()?.catch(() => setHasStarted(false));
  };

  if (source && !hasPlaybackError) {
    return (
      <div className={styles.frame}>
        <video
          aria-label={title}
          className={styles.video}
          controls={hasStarted}
          onError={() => setHasPlaybackError(true)}
          onPlay={() => setHasStarted(true)}
          poster={poster}
          preload="none"
          ref={videoRef}
        >
          <source src={source.src} type={source.type} />
          Ваш браузер не поддерживает воспроизведение видео.
        </video>
        {!hasStarted ? (
          <div className={styles.posterOverlay}>
            <button
              aria-label={`Воспроизвести: ${title}`}
              className={styles.playButton}
              onClick={startPlayback}
              type="button"
            >
              <span aria-hidden="true">▶</span>
            </button>
          </div>
        ) : null}
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
