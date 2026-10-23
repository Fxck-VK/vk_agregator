"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import { useMarketingReducedMotion, useMarketingVisible } from "./useMarketingMotion";
import styles from "./MarketingHome.module.css";

export function MarketingCarousel({ label, className, children, auto = false, paused = false }: { label: string; className: string; children: ReactNode; auto?: boolean; paused?: boolean }) {
  const { marketing: t } = useDictionary();
  const track = useRef<HTMLDivElement>(null);
  const reduced = useMarketingReducedMotion();
  const visible = useMarketingVisible(track);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [localPaused, setLocalPaused] = useState(false);
  const [edges, setEdges] = useState({ start: true, end: true });
  const move = (direction: number) => {
    const element = track.current;
    if (element) element.scrollBy({ left: direction * element.clientWidth * .8, behavior: reduced ? "instant" : "smooth" });
  };
  useEffect(() => {
    const element = track.current;
    if (!element) return;
    const update = () => setEdges({ start: element.scrollLeft < 2, end: element.scrollLeft + element.clientWidth >= element.scrollWidth - 2 });
    const initial = window.requestAnimationFrame(update); element.addEventListener("scroll", update, { passive: true });
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(update);
    observer?.observe(element);
    return () => { window.cancelAnimationFrame(initial); element.removeEventListener("scroll", update); observer?.disconnect(); };
  }, [children]);
  useEffect(() => {
    if (!auto || paused || localPaused || reduced || hovered || focused || !visible) return;
    const timer = window.setInterval(() => {
      const element = track.current;
      if (!element || element.scrollWidth <= element.clientWidth) return;
      const end = element.scrollLeft + element.clientWidth >= element.scrollWidth - 2;
      element.scrollTo({ left: end ? 0 : element.scrollLeft + (element.firstElementChild?.getBoundingClientRect().width ?? 230) + 16, behavior: "smooth" });
    }, 4500);
    return () => window.clearInterval(timer);
  }, [auto, paused, localPaused, reduced, hovered, focused, visible]);
  return <div className={styles.carousel} onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)} onFocusCapture={() => setFocused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false); }}>
    <div className={styles.carouselControls} aria-label={label}>
      {auto ? <button type="button" aria-label={`${localPaused ? t.play : t.pause}: ${label}`} aria-pressed={localPaused} onClick={() => setLocalPaused(!localPaused)}>{localPaused ? "▶" : "Ⅱ"}</button> : null}
      <button type="button" aria-label={`${t.carouselPrevious}: ${label}`} disabled={edges.start} onClick={() => move(-1)}>←</button>
      <button type="button" aria-label={`${t.carouselNext}: ${label}`} disabled={edges.end} onClick={() => move(1)}>→</button>
    </div>
    <div ref={track} className={className} role="region" aria-label={label} tabIndex={0} onKeyDown={event => {
      if (event.key === "ArrowRight" || event.key === "ArrowLeft") { event.preventDefault(); move(event.key === "ArrowRight" ? 1 : -1); }
    }}>{children}</div>
  </div>;
}
