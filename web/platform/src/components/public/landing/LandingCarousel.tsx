"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { LandingArrow } from "./LandingPrimitives";
import styles from "./LandingChrome.module.css";

export function LandingCarousel({ label, previous, next, children, auto = false, paused = false, className }: { label: string; previous: string; next: string; children: ReactNode; auto?: boolean; paused?: boolean; className?: string }) {
  const viewport = useRef<HTMLDivElement>(null);
  const interacting = useRef(false);
  const move = (direction: number) => {
    const node = viewport.current;
    node?.scrollBy({ left: direction * node.clientWidth * .8, behavior: window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" });
  };
  useEffect(() => {
    if (!auto || paused || window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;
    let visible = false;
    const observer = typeof IntersectionObserver === "undefined" ? null : new IntersectionObserver(entries => { visible = entries.some(entry => entry.isIntersecting); });
    if (viewport.current) observer?.observe(viewport.current);
    const timer = window.setInterval(() => {
      const node = viewport.current;
      if (!node || document.hidden || interacting.current || !visible) return;
      if (node.scrollLeft + node.clientWidth >= node.scrollWidth - 4) node.scrollTo({ left: 0, behavior: "smooth" });
      else node.scrollBy({ left: 220, behavior: "smooth" });
    }, 4000);
    return () => { window.clearInterval(timer); observer?.disconnect(); };
  }, [auto, paused]);
  return <div className={styles.carousel} role="region" aria-label={label} onMouseEnter={() => { interacting.current = true; }} onMouseLeave={() => { interacting.current = false; }} onFocusCapture={() => { interacting.current = true; }} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) interacting.current = false; }}>
    <div className={styles.carouselControls}><button type="button" onClick={() => move(-1)} aria-label={`${previous}: ${label}`}><LandingArrow direction="left" /></button><button type="button" onClick={() => move(1)} aria-label={`${next}: ${label}`}><LandingArrow /></button></div>
    <div ref={viewport} className={`${styles.carouselViewport} ${className ?? ""}`}>{children}</div>
  </div>;
}
