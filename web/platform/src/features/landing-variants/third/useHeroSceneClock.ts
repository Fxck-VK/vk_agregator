"use client";

import { useEffect, useRef, useState, type RefObject } from 'react';

/** One bounded clock per visible concept; manual selection also restarts its delay. */
export function useHeroSceneClock(target: RefObject<HTMLElement | null>, count: number, duration: number) {
  const [scene, setScene] = useState(0);
  const [running, setRunning] = useState(false);
  const restart = useRef(() => {});
  useEffect(() => {
    const node = target.current;
    if (!node) return;
    let inView = typeof IntersectionObserver === 'undefined';
    let timer = 0, disposed = false;
    const media = window.matchMedia?.('(prefers-reduced-motion: reduce)');
    const tick = () => {
      if (disposed) return;
      setScene(current => (current + 1) % count);
      timer = window.setTimeout(tick, duration);
    };
    const update = () => {
      if (disposed) return;
      window.clearTimeout(timer);
      const active = inView && !document.hidden && !media?.matches;
      setRunning(active);
      if (active) timer = window.setTimeout(tick, duration);
    };
    const observer = typeof IntersectionObserver === 'undefined' ? null : new IntersectionObserver(entries => {
      inView = entries.some(entry => entry.isIntersecting);
      update();
    }, { threshold: .1 });
    observer?.observe(node);
    media?.addEventListener('change', update);
    document.addEventListener('visibilitychange', update);
    restart.current = update;
    update();
    return () => {
      disposed = true; window.clearTimeout(timer); observer?.disconnect();
      media?.removeEventListener('change', update);
      document.removeEventListener('visibilitychange', update);
      restart.current = () => {};
    };
  }, [target, count, duration]);
  return { scene, running, select: (next: number) => {
    if (!Number.isInteger(next) || next < 0 || next >= count) return;
    setScene(next); restart.current();
  } };
}
