"use client";

/* eslint-disable @next/next/no-img-element -- Posters are pre-sized static WebP files; keep their load/error lifecycle local to the video. */
import { useEffect, useImperativeHandle, useRef, useState, type ComponentPropsWithoutRef, type Ref } from "react";
import { MediaState } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./MediaVideo.module.css";

type Props = ComponentPropsWithoutRef<"video"> & {
  src: string; passive?: boolean; videoRef?: Ref<HTMLVideoElement>;
  fit?: "contain" | "cover"; loading?: "lazy" | "eager";
};
export function MediaVideo(props: Props) { return <VideoAttempt key={`${props.src}:${props.poster ?? ""}`} {...props} />; }

function VideoAttempt({ passive = false, videoRef, fit = "contain", loading = "eager", poster, src, children,
  onError, onWaiting, onStalled, onCanPlay, onLoadedData, onLoadedMetadata, onPlaying, ...props }: Props) {
  const t = useDictionary();
  const ref = useRef<HTMLVideoElement | null>(null);
  useImperativeHandle(videoRef, () => ref.current!, []);
  const [visible, setVisible] = useState(loading !== "lazy");
  const [posterState, setPosterState] = useState<"loading" | "ready" | "error">("loading");
  const [hasFrame, setHasFrame] = useState(false);
  const [state, setState] = useState<"loading" | "ready" | "error">(props.preload === "none" ? "ready" : "loading");
  useEffect(() => {
    if (loading !== "lazy") return;
    if (typeof IntersectionObserver === "undefined") {
      // Older browsers load normally; keep hydration identical to the server.
      const timer = window.setTimeout(() => setVisible(true), 0);
      return () => window.clearTimeout(timer);
    }
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); }
    });
    if (ref.current) observer.observe(ref.current);
    return () => observer.disconnect();
  }, [loading]);
  const active = loading !== "lazy" || visible;
  const showPoster = !!poster && posterState !== "error" && (!hasFrame || state === "error");
  const readyPoster = showPoster && posterState === "ready";
  const frameReady = () => { setHasFrame(true); setState("ready"); };
  const showState = state !== "ready" && (state === "error" || !readyPoster);
  return <span className={styles.root} data-ui="media-video" data-fit={fit} data-state={state}
    style={props.width && props.height ? { aspectRatio: `${props.width} / ${props.height}` } : undefined}>
    <video {...props} ref={ref} src={active ? src : undefined}
      preload={!active || (poster && posterState === "loading") ? "none" : props.preload}
      onLoadedMetadata={onLoadedMetadata}
      onLoadedData={event => { frameReady(); onLoadedData?.(event); }}
      onPlaying={event => { frameReady(); onPlaying?.(event); }}
      onCanPlay={event => { frameReady(); onCanPlay?.(event); }}
      onWaiting={event => { setState("loading"); onWaiting?.(event); }}
      onStalled={event => { if (!event.currentTarget.paused) setState("loading"); onStalled?.(event); }}
      onError={event => { setState("error"); onError?.(event); }}>{active ? children : null}</video>
    {poster ? <img src={poster} alt="" aria-hidden="true" className={styles.poster} data-ui="video-poster"
      hidden={!showPoster} loading={loading} decoding="async" width={props.width} height={props.height}
      ref={image => { if (image?.complete && image.naturalWidth > 0) setPosterState("ready"); }}
      onLoad={() => setPosterState("ready")} onError={() => setPosterState("error")} /> : null}
    {showState ? <span className={styles.overlay} data-over-poster={readyPoster || hasFrame || undefined} data-passive={passive || state === "loading" || undefined}>
      <MediaState compact={passive} state={state} label={state === "error" ? t.files.previewFailure : t.files.previewLoading}
        retry={state === "error" && !passive ? { label: t.files.previewRetry, onClick: () => { setHasFrame(false); setState("loading"); ref.current?.load(); } } : undefined} />
    </span> : null}
  </span>;
}
