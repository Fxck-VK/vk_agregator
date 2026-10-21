"use client";

import Image from "next/image";
import { type KeyboardEvent, useEffect, useId, useRef, useState, useSyncExternalStore } from "react";
import { assetPaths } from "@/assets/asset-paths";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./LoginShowcase.module.css";

const slideDuration = 6500;
const motionQuery = "(prefers-reduced-motion: reduce)";
const wideQuery = "(min-width: 60.01rem)";
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
  const [paused, setPaused] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const environmentAllowsAnimation = useSyncExternalStore(subscribeEnvironment, canAnimate, serverFalse);
  const running = environmentAllowsAnimation && !paused && !hovered && !focused;
  const video = useRef<HTMLVideoElement>(null);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const id = useId();
  const slides = [
    { label: t.showcaseImages, title: t.showcaseImageTitle, text: t.showcaseImageText, tags: t.showcaseImageTags },
    { label: t.showcaseVideo, title: t.showcaseVideoTitle, text: t.showcaseVideoText, tags: t.showcaseVideoTags },
    { label: t.showcaseMusic, title: t.showcaseMusicTitle, text: t.showcaseMusicText, tags: t.showcaseMusicTags },
    { label: t.showcaseText, title: t.showcaseTextTitle, text: t.showcaseTextDescription, tags: t.showcaseTextTags },
  ];
  const current = slides[active];

  useEffect(() => {
    if (!running) return;
    const timer = window.setTimeout(() => setActive(value => (value + 1) % 4), slideDuration);
    return () => window.clearTimeout(timer);
  }, [active, running]);
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    if (running) void element.play().catch(() => {});
    else element.pause();
    return () => element.pause();
  }, [active, running]);

  function navigate(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const next = event.key === "ArrowRight" ? (index + 1) % 4 : event.key === "ArrowLeft" ? (index + 3) % 4 : event.key === "Home" ? 0 : event.key === "End" ? 3 : null;
    if (next === null) return;
    event.preventDefault(); setActive(next); tabs.current[next]?.focus();
  }

  return <section className={styles.showcase} data-testid="login-showcase" aria-label={t.showcaseLabel}
    onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}
    onFocusCapture={() => setFocused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false); }}>
    <div className={`${styles.panel} ${styles[`slide${active}`]}`} id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-tab-${active}`} tabIndex={0}>
      <div className={styles.visual} aria-hidden="true">
        {active === 0 ? <Image src={assetPaths.images.inspiration.desertAirplaneTraveler} alt="" fill sizes="(max-width: 60rem) 1px, 48vw" className={styles.photo} /> : null}
        {active === 1 ? <video ref={video} className={styles.video} src={environmentAllowsAnimation ? assetPaths.videos.inspiration.videoOne : undefined} poster={assetPaths.images.inspiration.videoOnePoster} muted playsInline loop preload="none" /> : null}
        {active === 2 ? <div className={styles.musicArt}><div className={styles.record}><span>♪</span></div><p>{t.showcaseMusicTrack}</p><div className={styles.wave}>{Array.from({ length: 35 }, (_, index) => <i key={index} style={{ height: `${16 + ((index * 37 + 17) % 65)}%` }} />)}</div></div> : null}
        {active === 3 ? <div className={styles.chatArt}><div className={styles.chatPrompt}>{t.showcasePrompt}</div><div className={styles.chatAnswer}><span className={styles.spark}>✦</span><p>{t.showcaseAnswer}</p><i /><i /><i /></div><div className={styles.chatInput}>{t.showcaseInput}<span>↑</span></div></div> : null}
      </div>
      <div className={styles.caption}><h2>{current.title}</h2><p>{current.text}</p><div className={styles.tags}>{current.tags.split("|").map(tag => <span key={tag}>{tag}</span>)}</div></div>
    </div>
    <button className={styles.pause} type="button" aria-label={paused ? t.showcasePlay : t.showcasePause} aria-pressed={paused} onClick={() => setPaused(value => !value)}><span aria-hidden="true">{paused ? "▶" : "Ⅱ"}</span></button>
    <div className={styles.tabs} role="tablist" aria-label={t.showcaseLabel}>{slides.map((slide, index) => <button key={slide.label} ref={element => { tabs.current[index] = element; }} type="button" role="tab" id={`${id}-tab-${index}`} aria-controls={`${id}-panel`} aria-selected={active === index} tabIndex={active === index ? 0 : -1} onClick={() => setActive(index)} onKeyDown={event => navigate(event, index)}><span className={styles.track}><i key={`${active}-${running}`} data-active={active === index} data-running={running} /></span>{slide.label}</button>)}</div>
  </section>;
}
