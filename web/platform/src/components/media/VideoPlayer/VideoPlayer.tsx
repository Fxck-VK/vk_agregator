"use client";

import { MediaState } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";


import { type CSSProperties, useEffect, useRef, useState } from "react";

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
  const msg = useMessages();
  const [hasPlaybackError, setHasPlaybackError] = useState(false);
  const [isWaiting, setIsWaiting] = useState(false);
  const [hasStarted, setHasStarted] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const spacePauseActiveRef = useRef(false);
  const videoRef = useRef<HTMLVideoElement>(null);
  const posterOverlayStyle = {
    "--video-player-poster": poster ? `url("${poster}")` : "none",
  } as CSSProperties;

  useEffect(() => {
    const blockSpaceEvent = (event: KeyboardEvent) => {
      event.preventDefault();
      event.stopPropagation();
    };

    const pauseWithSpace = (event: KeyboardEvent) => {
      if (event.code !== "Space") {
        return;
      }

      if (spacePauseActiveRef.current) {
        blockSpaceEvent(event);
        return;
      }

      const target = event.target;
      const video = videoRef.current;
      if (
        !video
        || video.paused
        || video.ended
        || event.defaultPrevented
        || (target instanceof HTMLElement && (
          target.isContentEditable
          || target.closest("input, textarea, select, button, a, [role='button'], [contenteditable='true']")
        ))
      ) {
        return;
      }

      spacePauseActiveRef.current = true;
      blockSpaceEvent(event);
      video.pause();
    };

    const finishSpacePause = (event: KeyboardEvent) => {
      if (event.code !== "Space" || !spacePauseActiveRef.current) {
        return;
      }

      blockSpaceEvent(event);
      spacePauseActiveRef.current = false;
    };

    const resetSpacePause = () => {
      spacePauseActiveRef.current = false;
    };

    window.addEventListener("keydown", pauseWithSpace, true);
    window.addEventListener("keyup", finishSpacePause, true);
    window.addEventListener("blur", resetSpacePause);
    return () => {
      window.removeEventListener("keydown", pauseWithSpace, true);
      window.removeEventListener("keyup", finishSpacePause, true);
      window.removeEventListener("blur", resetSpacePause);
    };
  }, []);

  const startPlayback = () => {
    const video = videoRef.current;
    if (!video) {
      return;
    }

    const isResuming = hasStarted;
    setHasStarted(true);
    setIsPaused(false);
    video.play()?.catch(() => {
      setHasStarted(isResuming);
      setIsPaused(isResuming);
    });
  };

  const restoreAfterSeeking = () => {
    const video = videoRef.current;
    if (!video) {
      return;
    }

    setIsPaused(hasStarted && (video.paused || video.ended));
  };

  if (source && !hasPlaybackError) {
    return (
      <div className={styles.frame}>
        <video
          aria-label={title}
          className={styles.video}
          controls={hasStarted}
          onWaiting={() => setIsWaiting(true)}
          onPlaying={() => setIsWaiting(false)}
          onCanPlay={() => setIsWaiting(false)}
          onEnded={() => {
            setIsPaused(true);
          }}
          onError={() => {
            setHasPlaybackError(true);
          }}
          onPause={() => {
            if (hasStarted) {
              setIsPaused(true);
            }
          }}
          onPlay={() => {
            setHasStarted(true);
            setIsPaused(false);
          }}
          onSeeked={restoreAfterSeeking}
          poster={poster}
          preload="none"
          ref={videoRef}
        >
          <source src={source.src} type={source.type} />
          {msg("videoPlayer.yourBrowserDoesNotSupportVideoPlayback")}</video>
        {hasStarted && isWaiting ? <div className={styles.waiting}><MediaState label={title} /></div> : null}
        {!hasStarted ? (
          <button
            aria-label={msg("videoPlayer.playValue", { value1: title })}
            className={styles.posterOverlay}
            data-testid="video-poster-overlay"
            onClick={startPlayback}
            style={posterOverlayStyle}
            type="button"
          >
            <span aria-hidden="true" className={styles.playButton}>
              <span aria-hidden="true">▶</span>
            </span>
          </button>
        ) : null}
        {hasStarted && isPaused ? (
          <div className={styles.pauseOverlay} data-testid="video-pause-overlay">
            <button
              aria-label={msg("videoPlayer.resumeValue", { value1: title })}
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

  if (hasPlaybackError) return <div className={styles.frame}><MediaState state="error" label={msg("videoPlayer.videoIsTemporarilyUnavailable")} retry={{ label: msg("chatComposer.uploadFailedRetry"), onClick: () => { setHasPlaybackError(false); setHasStarted(false); setIsPaused(false); setIsWaiting(false); } }} /></div>;

  const placeholder = hasPlaybackError ? msg("videoPlayer.videoIsTemporarilyUnavailable") : msg("videoPlayer.videoIsComingSoon");

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
