"use client";

import { type KeyboardEvent, useEffect, useId, useRef, useState, useSyncExternalStore } from "react";
import { assetPaths } from "@/assets/asset-paths";
import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./LoginShowcase.module.css";

const slideDuration = 6000;
const motionQuery = "(prefers-reduced-motion: reduce)";
const wideQuery = "(min-width: 64.01rem)";
const serverFalse = () => false;
function subscribeEnvironment(callback: () => void) {
  const queries = typeof window.matchMedia === "function" ? [window.matchMedia(motionQuery), window.matchMedia(wideQuery)] : [];
  queries.forEach(query => query.addEventListener("change", callback));
  document.addEventListener("visibilitychange", callback);
  return () => {
    queries.forEach(query => query.removeEventListener("change", callback));
    document.removeEventListener("visibilitychange", callback);
  };
}
function canAnimate() {
  return typeof window.matchMedia === "function" && window.matchMedia(wideQuery).matches && !window.matchMedia(motionQuery).matches && document.visibilityState === "visible";
}

export function LoginShowcase() {
  const { auth: t } = useDictionary();
  const [active, setActive] = useState(0);
  const environmentAllowsAnimation = useSyncExternalStore(subscribeEnvironment, canAnimate, serverFalse);
  const running = environmentAllowsAnimation;
  const video = useRef<HTMLVideoElement>(null);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const id = useId();
  const slides: { label: string; title: string; text: string; image: string; video?: string }[] = [
    { label: t.showcaseImages, title: t.showcaseImageTitle, text: t.showcaseImageText, image: assetPaths.images.auth.creativeImage },
    { label: t.showcaseVideo, title: t.showcaseVideoTitle, text: t.showcaseVideoText, image: assetPaths.images.auth.creativeImage, video: assetPaths.videos.auth.creativeVideo },
    { label: t.showcaseMusic, title: t.showcaseMusicTitle, text: t.showcaseMusicText, image: assetPaths.images.auth.creativeAudio },
  ];
  const count = slides.length;
  const current = slides[active];

  useEffect(() => {
    if (!running) return;
    const timer = window.setTimeout(() => setActive(value => (value + 1) % count), slideDuration);
    return () => window.clearTimeout(timer);
  }, [active, running, count]);
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    if (running) void element.play().catch(() => {});
    else element.pause();
    return () => element.pause();
  }, [active, running, environmentAllowsAnimation]);

  function navigate(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next = event.key === "ArrowRight" ? (index + 1) % count : event.key === "ArrowLeft" ? (index + count - 1) % count : event.key === "Home" ? 0 : event.key === "End" ? count - 1 : null;
    if (next === null) return;
    event.preventDefault(); setActive(next); tabs.current[next]?.focus();
  }

  return <section className={styles.showcase} data-testid="login-showcase" aria-label={t.showcaseLabel}>
    <div className={styles.panel} id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-tab-${active}`} tabIndex={0}>
      <div key={active} className={styles.visual} aria-hidden="true">
        {current.video && environmentAllowsAnimation
          ? <MediaVideo videoRef={video} src={current.video} poster={current.image} fit="cover" passive muted playsInline loop preload="auto" />
          : <MediaImage src={current.image} alt="" fit="cover" passive showLoading={false} loading="lazy" />}
      </div>
      <div className={styles.caption}><h2>{current.title}</h2><p>{current.text}</p></div>
    </div>
    <div className={styles.tabs} role="tablist" aria-label={t.showcaseLabel}>{slides.map((slide, index) => <button key={slide.label} ref={element => { tabs.current[index] = element; }} type="button" role="tab" id={`${id}-tab-${index}`} aria-controls={`${id}-panel`} aria-selected={active === index} tabIndex={active === index ? 0 : -1} onClick={() => setActive(index)} onKeyDown={event => navigate(event, index)}><span className={styles.track}><i key={`${active}-${running}`} data-active={active === index} data-running={running} /></span>{slide.label}</button>)}</div>
  </section>;
}
