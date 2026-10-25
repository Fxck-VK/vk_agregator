"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { LandingArrow } from "./LandingPrimitives";
import styles from "./LandingChrome.module.css";

export function LandingCarousel({ label, previous, next, children, auto = false, paused = false, className, bounded = false, controls = "above" }: { label: string; previous: string; next: string; children: ReactNode; auto?: boolean; paused?: boolean; className?: string; bounded?: boolean; controls?: "above"|"overlay" }) {
  const viewport = useRef<HTMLDivElement>(null);
  const interacting = useRef(false);
  const [edges,setEdges] = useState({ start:true,end:false });
  useEffect(() => {
    const node = viewport.current;
    if (!bounded || !node) return;
    const update = () => {
      const start = node.scrollLeft <= 1;
      const end = node.scrollLeft + node.clientWidth >= node.scrollWidth - 2;
      setEdges(current => current.start === start && current.end === end ? current : { start,end });
    };
    update(); node.addEventListener("scroll",update,{ passive:true }); window.addEventListener("resize",update);
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(update);
    observer?.observe(node);
    return () => { node.removeEventListener("scroll",update); window.removeEventListener("resize",update); observer?.disconnect(); };
  },[bounded,children]);
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
  return <div className={styles.carousel} role="region" aria-label={label} data-landing-carousel data-controls={controls} onMouseEnter={() => { interacting.current = true; }} onMouseLeave={() => { interacting.current = false; }} onFocusCapture={() => { interacting.current = true; }} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) interacting.current = false; }}>
    <div className={styles.carouselControls}><button type="button" disabled={bounded && edges.start} onClick={() => move(-1)} aria-label={`${previous}: ${label}`}><LandingArrow direction="left" /></button><button type="button" disabled={bounded && edges.end} onClick={() => move(1)} aria-label={`${next}: ${label}`}><LandingArrow /></button></div>
    <div ref={viewport} data-landing-carousel-viewport className={`${styles.carouselViewport} ${className ?? ""}`}>{children}</div>
  </div>;
}
