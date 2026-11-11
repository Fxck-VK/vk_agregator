"use client";

import { useEffect, useRef, useState, type MouseEvent, type PointerEvent } from 'react';
import { LandingToolIcon } from '@/components/public/landing/LandingPrimitives';
import Link from '@/i18n/Link';
import type { HomeEnglishCopy } from '@/i18n/public/home-en';
import { useModelMarquee } from './useModelMarquee';
import homeStyles from './PublicHome.module.css';
import styles from './ToolsMarquee.module.css';

type Tool = HomeEnglishCopy['tools']['items'][number];

export function ToolsMarquee({ copy, items }: { copy: HomeEnglishCopy; items: readonly Tool[] }) {
  const [focused, setFocused] = useState(false);
  const [touching, setTouching] = useState(false);
  const pointer = useRef({ x: 0, y: 0, down: false, dragged: false });
  const contentKey = items.map(item => item.href).join('|');
  const paused = focused || touching;
  const { viewport, original, copies, ready, reduced, normalize } = useModelMarquee(paused, contentKey);

  const markDragged = () => {
    pointer.current.dragged = true;
    const active = document.activeElement;
    if (active instanceof HTMLElement && viewport.current?.contains(active)) active.blur();
  };
  const pointerDown = (event: PointerEvent<HTMLDivElement>) => {
    pointer.current = { x: event.clientX, y: event.clientY, down: true, dragged: false };
    if (event.pointerType === 'touch') setTouching(true);
  };
  const pointerEnd = () => { pointer.current.down = false; setTouching(false); };
  const click = (event: MouseEvent<HTMLAnchorElement>) => {
    if (event.detail > 0 && pointer.current.dragged) event.preventDefault();
  };

  useEffect(() => {
    const outside = (event: globalThis.PointerEvent) => {
      if (!(event.target instanceof Node) || viewport.current?.contains(event.target)) return;
      const active = document.activeElement;
      if (active instanceof HTMLElement && viewport.current?.contains(active)) active.blur();
    };
    document.addEventListener('pointerdown', outside);
    return () => {
      document.removeEventListener('pointerdown', outside);
    };
  }, [viewport]);

  useEffect(() => {
    // Replacing a filter can remove a focused link without dispatching blur.
    if (!viewport.current?.contains(document.activeElement)) setFocused(false);
  }, [contentKey, viewport]);

  return <div ref={viewport} className={styles.viewport} role="region" aria-label={copy.tools.label}
    data-tool-marquee data-ready={ready} data-paused={paused || reduced}
    onPointerDown={pointerDown}
    onPointerMove={event => { if (pointer.current.down && Math.hypot(event.clientX - pointer.current.x, event.clientY - pointer.current.y) > 8) markDragged(); }}
    onPointerUp={pointerEnd} onPointerCancel={() => { markDragged(); pointerEnd(); }}
    onDragStart={event => { event.preventDefault(); markDragged(); }}
    onFocusCapture={event => {
      const link = (event.target as HTMLElement).closest<HTMLAnchorElement>('[data-tool-index]');
      if (!link) return;
      if (link.dataset.copy !== '1') {
        normalize();
        original.current?.querySelector<HTMLAnchorElement>(`[data-tool-index="${link.dataset.toolIndex}"]`)?.focus({ preventScroll: true });
      }
      setFocused(true);
    }}
    onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false); }}>
    <div className={styles.track}>
      {Array.from({ length: copies }, (_, group) => <div key={group} ref={group === 1 ? original : undefined} className={styles.group} aria-hidden={group !== 1 ? true : undefined} data-marquee-copy={group}>
        {items.map((tool, index) => <div className={styles.slot} key={tool.href}>
          <Link href={tool.href} className={`${homeStyles.toolCard} ${styles.card}`} tabIndex={group === 1 ? 0 : -1} data-tool-index={index} data-copy={group} onClick={click}>
            <span className={homeStyles.toolIcon}><LandingToolIcon name={tool.icon} /></span><h3>{tool.title}</h3><p>{tool.description}</p>
          </Link>
        </div>)}
        <div className={styles.slot}><Link href="/app/models" className={`${homeStyles.toolCard} ${homeStyles.allTools} ${styles.card}`} tabIndex={group === 1 ? 0 : -1} data-tool-index={items.length} data-copy={group} onClick={click}><span>{copy.tools.all}</span></Link></div>
      </div>)}
    </div>
  </div>;
}
