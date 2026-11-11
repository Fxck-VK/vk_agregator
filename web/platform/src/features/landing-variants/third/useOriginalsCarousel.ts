"use client";

import { useCallback, useEffect, useRef, useState, useSyncExternalStore, type PointerEvent } from 'react';

const motionQuery = '(prefers-reduced-motion: reduce)';
const readMotion = () => window.matchMedia?.(motionQuery).matches ?? false;
const readHidden = () => document.hidden;
const serverFalse = () => false;
function subscribeMotion(update: () => void) {
  const media = window.matchMedia?.(motionQuery);
  media?.addEventListener('change', update);
  return () => media?.removeEventListener('change', update);
}
function subscribeHidden(update: () => void) {
  document.addEventListener('visibilitychange', update);
  return () => document.removeEventListener('visibilitychange', update);
}

type Destination = { index: number; direction?: number };
type Motion = { position: number; animate: boolean; moving: boolean; queued: Destination | null };
function moveTo(current: Motion, destination: Destination, count: number, reduced: boolean): Motion {
  const selected = ((current.position % count) + count) % count;
  const forward = (destination.index - selected + count) % count;
  const backward = forward === 0 ? 0 : forward - count;
  const distance = destination.direction === 1 ? forward : destination.direction === -1 ? backward
    : forward <= Math.abs(backward) ? forward : backward;
  return { position: reduced ? count + destination.index : current.position + distance,
    animate: !reduced, moving: !reduced && distance !== 0, queued: null };
}

export function useOriginalsCarousel(count: number) {
  const root = useRef<HTMLDivElement>(null);
  const [motion, setMotion] = useState<Motion>({ position: count, animate: true, moving: false, queued: null });
  const [hovered, setHovered] = useState(false), [focused, setFocused] = useState(false);
  const [visible, setVisible] = useState(false), [drag, setDrag] = useState(0);
  const hidden = useSyncExternalStore(subscribeHidden, readHidden, serverFalse);
  const reduced = useSyncExternalStore(subscribeMotion, readMotion, serverFalse);
  const [touching, setTouching] = useState(false);
  const gesture = useRef({ down: false, x: 0, y: 0, dx: 0, horizontal: false });
  const dragged = useRef(false);
  const selected = ((motion.position % count) + count) % count;
  const finish = useCallback(() => {
    setMotion(current => {
      const position = count + ((current.position % count) + count) % count;
      return { position, animate: position === current.position, moving: false, queued: current.queued };
    });
  }, [count]);
  const go = (index: number) => setMotion(current => current.moving || !current.animate && !reduced
    ? { ...current, queued: { index } } : moveTo(current, { index }, count, reduced));
  const step = useCallback((direction: number) => {
    setMotion(current => {
      const base = current.queued?.index ?? ((current.position % count) + count) % count;
      const destination = { index: (base + direction + count) % count, direction };
      return current.moving || !current.animate && !reduced
        ? { ...current, queued: destination } : moveTo(current, destination, count, reduced);
    });
  }, [count, reduced]);

  useEffect(() => {
    const observer = typeof IntersectionObserver === 'undefined' ? null
      : new IntersectionObserver(entries => setVisible(entries.some(entry => entry.isIntersecting)), { threshold: .15 });
    if (root.current) observer?.observe(root.current);
    const frame = !observer ? window.requestAnimationFrame(() => setVisible(true)) : 0;
    return () => {
      observer?.disconnect(); window.cancelAnimationFrame(frame);
    };
  }, []);
  useEffect(() => {
    if (motion.animate || reduced) return;
    let second = 0;
    const first = window.requestAnimationFrame(() => {
      second = window.requestAnimationFrame(() => setMotion(current => ({ ...current, animate: true })));
    });
    return () => { cancelAnimationFrame(first); cancelAnimationFrame(second); };
  }, [motion.animate, reduced]);
  useEffect(() => {
    if (motion.moving || !motion.queued || !motion.animate && !reduced) return;
    const frame = window.requestAnimationFrame(() => setMotion(current =>
      current.moving || !current.queued || !current.animate && !reduced ? current
        : moveTo(current, current.queued, count, reduced)));
    return () => window.cancelAnimationFrame(frame);
  }, [motion.moving, motion.queued, motion.animate, count, reduced]);
  useEffect(() => {
    if (!motion.moving) return;
    const timer = window.setTimeout(finish, reduced ? 0 : 720);
    return () => window.clearTimeout(timer);
  }, [motion.moving, motion.position, finish, reduced]);
  useEffect(() => {
    if (!visible || hidden || reduced || hovered || focused || touching || motion.moving || motion.queued) return;
    const timer = window.setTimeout(() => step(1), 6000);
    return () => window.clearTimeout(timer);
  }, [visible, hidden, reduced, hovered, focused, touching, motion.moving, motion.position, motion.queued, step]);

  const pointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || motion.moving) return;
    gesture.current = { down: true, x: event.clientX, y: event.clientY, dx: 0, horizontal: false };
    dragged.current = false; setTouching(true);
  };
  const pointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const current = gesture.current;
    if (!current.down) return;
    const dx = event.clientX - current.x, dy = event.clientY - current.y;
    if (!current.horizontal && Math.abs(dx) > 8 && Math.abs(dx) > Math.abs(dy)) {
      current.horizontal = true; dragged.current = true;
      event.currentTarget.setPointerCapture?.(event.pointerId);
    }
    if (current.horizontal) { current.dx = dx; setDrag(Math.max(-240, Math.min(240, dx))); }
  };
  const pointerEnd = (cancel = false) => {
    const current = gesture.current;
    if (current.down && !cancel && current.horizontal && Math.abs(current.dx) > 40) step(current.dx < 0 ? 1 : -1);
    current.down = false; current.horizontal = false; current.dx = 0;
    setDrag(0); setTouching(false);
  };
  return { root, selected, position: motion.position, animate: motion.animate && !reduced,
    moving: motion.moving, drag, dragged, finish, go, step, setHovered, setFocused,
    pointerDown, pointerMove, pointerEnd };
}
