"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';

/** Equivalent pixels in the middle copy, with one complete copy available to its left. */
export function normalizeMarqueeOffset(offset: number, cycle: number) {
  if (cycle <= 0) return offset;
  return cycle + ((offset - cycle) % cycle + cycle) % cycle;
}

export function useModelMarquee(paused: boolean, contentKey?: string, pixelsPerSecond = 41.8) {
  const viewport = useRef<HTMLDivElement>(null);
  const original = useRef<HTMLDivElement>(null);
  const cycle = useRef(0);
  const motionOffset = useRef(0);
  const [copies, setCopies] = useState(3);
  const [ready, setReady] = useState(false);
  const [visible, setVisible] = useState(true);
  const [reduced, setReduced] = useState(false);
  const [hidden, setHidden] = useState(false);

  const normalize = useCallback(() => {
    const node = viewport.current;
    if (!node || !cycle.current) return;
    const next = normalizeMarqueeOffset(node.scrollLeft, cycle.current);
    motionOffset.current = next;
    if (Math.abs(next - node.scrollLeft) > .5) node.scrollLeft = next;
  }, []);

  useLayoutEffect(() => {
    const node = viewport.current, group = original.current;
    if (!node || !group) return;
    cycle.current = 0;
    const measure = () => {
      const next = group.getBoundingClientRect().width;
      if (!next) return;
      const phase = cycle.current ? (node.scrollLeft - cycle.current) / cycle.current : 0;
      cycle.current = next;
      setCopies(Math.max(3, Math.ceil(node.clientWidth / next) + 3));
      motionOffset.current = normalizeMarqueeOffset(next * (1 + phase), next);
      node.scrollLeft = motionOffset.current;
      setReady(true);
    };
    measure();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    observer?.observe(node); observer?.observe(group);
    window.addEventListener('resize', measure);
    return () => { observer?.disconnect(); window.removeEventListener('resize', measure); };
  }, [contentKey]);

  // A short filtered group may need more copies than the initial render. Apply
  // its target after those copies exist, before the browser paints the strip.
  useLayoutEffect(() => {
    if (viewport.current) viewport.current.scrollLeft = motionOffset.current;
  }, [copies]);

  useEffect(() => {
    const node = viewport.current;
    if (!node) return;
    const observer = typeof IntersectionObserver === 'undefined' ? null : new IntersectionObserver(entries => {
      setVisible(entries.some(entry => entry.isIntersecting));
    }, { threshold: .1 });
    if (observer) observer.observe(node);
    return () => observer?.disconnect();
  }, []);

  useEffect(() => {
    const media = window.matchMedia?.('(prefers-reduced-motion: reduce)');
    const updateMotion = () => setReduced(media?.matches ?? false);
    const updateVisibility = () => setHidden(document.hidden);
    updateMotion(); updateVisibility();
    media?.addEventListener('change', updateMotion);
    document.addEventListener('visibilitychange', updateVisibility);
    return () => { media?.removeEventListener('change', updateMotion); document.removeEventListener('visibilitychange', updateVisibility); };
  }, []);

  useEffect(() => {
    const node = viewport.current;
    if (!node || !ready || paused || reduced || hidden || !visible) return;
    motionOffset.current = node.scrollLeft;
    let frame = 0, last = 0;
    const tick = (now: number) => {
      if (last) {
        motionOffset.current = normalizeMarqueeOffset(motionOffset.current + Math.min(now - last, 64) * pixelsPerSecond / 1000, cycle.current);
        node.scrollLeft = motionOffset.current;
      }
      last = now;
      frame = window.requestAnimationFrame(tick);
    };
    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [ready, paused, reduced, hidden, visible, pixelsPerSecond]);

  return { viewport, original, copies, ready, reduced, normalize };
}
