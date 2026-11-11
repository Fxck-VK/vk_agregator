"use client";

import { useEffect, useRef, useState, type MouseEvent, type PointerEvent } from 'react';
import Link from '@/i18n/Link';
import { ModelIcon } from '@/features/models/ModelIcon/ModelIcon';
import { modelIconRegistry } from '@/features/models/model-icon-registry';
import { modelEntryHref } from '@/features/models/model-entry';
import type { HomeEnglishCopy } from '@/i18n/public/home-en';
import { HomePicture } from './HomeMedia';
import { modelPhotos } from './photos';
import { useModelMarquee } from './useModelMarquee';
import styles from './ModelsMarquee.module.css';

export function ModelsMarquee({ copy }: { copy: HomeEnglishCopy }) {
  const [hovered, setHovered] = useState<string | null>(null);
  const [focused, setFocused] = useState<string | null>(null);
  const [touchPreview, setTouchPreview] = useState<string | null>(null);
  const [touching, setTouching] = useState(false);
  const hoverPoint = useRef<{ x: number; y: number } | null>(null);
  const pointer = useRef({ type: 'mouse', x: 0, y: 0, down: false, dragged: false });
  const paused = focused !== null || touchPreview !== null || touching;
  const { viewport, original, copies, ready, reduced, normalize } = useModelMarquee(paused);
  const preview = hovered ?? touchPreview ?? focused;

  const markDragged = () => {
    pointer.current.dragged = true; setTouchPreview(null);
    const active = document.activeElement;
    if (active instanceof HTMLElement && viewport.current?.contains(active)) active.blur();
  };
  const pointerDown = (event: PointerEvent<HTMLDivElement>) => {
    pointer.current = { type: event.pointerType, x: event.clientX, y: event.clientY, down: true, dragged: false };
    if (event.pointerType === 'touch') setTouching(true);
  };
  const pointerMove = (event: PointerEvent<HTMLDivElement>) => {
    if (pointer.current.down && Math.hypot(event.clientX - pointer.current.x, event.clientY - pointer.current.y) > 8) markDragged();
  };
  const pointerEnd = () => {
    pointer.current.down = false;
    setTouching(false);
  };
  const click = (event: MouseEvent<HTMLAnchorElement>, id: string) => {
    if (event.detail === 0) return;
    if (pointer.current.dragged) { event.preventDefault(); return; }
    if (pointer.current.type !== 'touch') return;
    if (touchPreview !== id) { event.preventDefault(); setTouchPreview(id); }
  };

  useEffect(() => {
    let frame = 0;
    const clearHover = () => {
      hoverPoint.current = null;
      setHovered(null);
    };
    const reconcileHover = () => {
      frame = 0;
      const point = hoverPoint.current, node = viewport.current;
      if (!point || !node) return;
      const hit = document.elementFromPoint(point.x, point.y);
      const inside = hit !== null && node.contains(hit);
      setHovered(inside ? hit.closest<HTMLElement>('[data-model-id]')?.dataset.modelId ?? null : null);
    };
    const schedule = () => {
      if (hoverPoint.current && !frame) frame = window.requestAnimationFrame(reconcileHover);
    };
    const remember = (event: globalThis.PointerEvent) => {
      if (event.pointerType === 'touch') { clearHover(); return; }
      hoverPoint.current = { x: event.clientX, y: event.clientY };
    };
    const leaveWindow = (event: globalThis.PointerEvent) => {
      if (event.relatedTarget === null) clearHover();
    };
    // Scrolling can move a card away from a stationary pointer without delivering
    // another boundary event. Recheck the topmost element after layout updates.
    window.addEventListener('pointermove', remember, { passive: true });
    window.addEventListener('pointerdown', remember, { passive: true });
    window.addEventListener('pointerout', leaveWindow);
    window.addEventListener('scroll', schedule, { capture: true, passive: true });
    window.addEventListener('resize', schedule);
    window.addEventListener('blur', clearHover);
    return () => {
      window.removeEventListener('pointermove', remember);
      window.removeEventListener('pointerdown', remember);
      window.removeEventListener('pointerout', leaveWindow);
      window.removeEventListener('scroll', schedule, true);
      window.removeEventListener('resize', schedule);
      window.removeEventListener('blur', clearHover);
      window.cancelAnimationFrame(frame);
    };
  }, [viewport]);

  useEffect(() => {
    const outside = (event: globalThis.PointerEvent) => {
      if (event.target instanceof Node && !viewport.current?.contains(event.target)) {
        setTouchPreview(null);
        const active = document.activeElement;
        if (active instanceof HTMLElement && viewport.current?.contains(active)) active.blur();
      }
    };
    document.addEventListener('pointerdown', outside);
    return () => { document.removeEventListener('pointerdown', outside); };
  }, [viewport]);

  return <div ref={viewport} className={styles.viewport} role="region" aria-label={copy.models.label}
    data-model-marquee data-ready={ready} data-paused={paused || reduced}
    onPointerEnter={event => { if (event.pointerType !== 'touch') hoverPoint.current = { x: event.clientX, y: event.clientY }; }}
    onPointerLeave={event => { if (event.pointerType !== 'touch') setHovered(null); }}
    onPointerDown={pointerDown} onPointerMove={pointerMove} onPointerUp={pointerEnd}
    onPointerCancel={() => { markDragged(); pointerEnd(); }}
    onDragStart={event => { event.preventDefault(); markDragged(); }}
    onFocusCapture={event => {
      const link = (event.target as HTMLElement).closest<HTMLAnchorElement>('[data-model-index]');
      if (!link) return;
      if (link.dataset.copy !== '1') {
        normalize();
        original.current?.querySelector<HTMLAnchorElement>(`[data-model-index="${link.dataset.modelIndex}"]`)?.focus({ preventScroll: true });
      }
      setFocused(link.dataset.modelId ?? null);
    }}
    onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(null); }}>
    <div className={styles.track}>
      {Array.from({ length: copies }, (_, group) => <div key={group} ref={group === 1 ? original : undefined} className={styles.group} aria-hidden={group !== 1 ? true : undefined} data-marquee-copy={group}>
        {copy.models.items.map((model, index) => <Link key={model.id} href={modelEntryHref(model.id)} prefetch={false} className={styles.card} tabIndex={group === 1 ? 0 : -1}
          data-model-index={index} data-model-id={model.id} data-copy={group} data-preview={preview === model.id}
          onPointerEnter={event => { if (event.pointerType !== 'touch') setHovered(model.id); }}
          onPointerLeave={event => { if (event.pointerType !== 'touch') setHovered(null); }}
          onClick={event => click(event, model.id)}>
          <div className={styles.photo}><HomePicture photo={modelPhotos[model.id]} sizes="(max-width: 767px) 260px, 356px" /></div>
          <span className={styles.shade} aria-hidden="true" />
          <span className={styles.label}><ModelIcon src={modelIconRegistry[model.id]} className={styles.logo} /><span>{model.name}</span></span>
        </Link>)}
      </div>)}
    </div>
  </div>;
}
