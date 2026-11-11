"use client";

import type { CSSProperties } from 'react';
import { LandingArrow, LandingLink } from '@/components/public/landing/LandingPrimitives';
import type { HomeEnglishCopy } from '@/i18n/public/home-en';
import { useMessages } from '@/i18n/LocaleProvider';
import { HomePicture } from './HomeMedia';
import { originalPhotos } from './photos';
import { useOriginalsCarousel } from './useOriginalsCarousel';
import styles from './OriginalsCarousel.module.css';

export function OriginalsCarousel({ copy }: { copy: HomeEnglishCopy }) {
  const msg = useMessages();
  const slides = copy.originals.slides;
  const { root, selected, position, animate, moving, drag, dragged, finish, go, step,
    setHovered, setFocused, pointerDown, pointerMove, pointerEnd } = useOriginalsCarousel(slides.length);
  return <div ref={root} className={styles.carousel} role="region" aria-label={copy.originals.title}
    aria-roledescription={msg('OriginalsCarousel.carousel')} data-original-selected={selected} data-original-animate={animate} data-original-moving={moving}
    onPointerEnter={e => { if (e.pointerType !== 'touch') setHovered(true); }}
    onPointerLeave={e => { if (e.pointerType !== 'touch') setHovered(false); }}
    onFocusCapture={() => setFocused(true)}
    onBlurCapture={e => { if (!e.currentTarget.contains(e.relatedTarget)) setFocused(false); }}
    onKeyDown={e => {
      const target = e.key === 'ArrowRight' ? (selected + 1) % slides.length
        : e.key === 'ArrowLeft' ? (selected + slides.length - 1) % slides.length
        : e.key === 'Home' ? 0 : e.key === 'End' ? slides.length - 1 : null;
      if (target === null) return;
      e.preventDefault();
      if (e.key === 'ArrowRight') step(1);
      else if (e.key === 'ArrowLeft') step(-1);
      else go(target);
      e.currentTarget.querySelector<HTMLButtonElement>(`[data-original-dot="${target}"]`)?.focus({ preventScroll: true });
    }}>
    <div className={styles.viewport} data-original-viewport
      onPointerDown={pointerDown} onPointerMove={pointerMove}
      onPointerUp={() => pointerEnd()} onPointerCancel={() => pointerEnd(true)}
      onDragStart={e => e.preventDefault()}
      onClickCapture={e => {
        if (e.detail !== 0 && dragged.current) { e.preventDefault(); e.stopPropagation(); return; }
        const card = (e.target as HTMLElement).closest<HTMLElement>('[data-original-card]');
        if (card && card.dataset.active !== 'true') {
          e.preventDefault(); go(Number(card.dataset.originalCard) % slides.length);
        }
      }}>
      <div className={styles.track} data-original-track data-position={position}
        data-animate={animate && drag === 0}
        style={{ '--original-position': position, '--original-drag': `${drag}px` } as CSSProperties}
        onTransitionEnd={e => { if (e.target === e.currentTarget && e.propertyName === 'transform') finish(); }}>
        {Array.from({ length: slides.length * 3 }, (_, cardPosition) => {
          const index = cardPosition % slides.length, slide = slides[index], active = cardPosition === position;
          return <div className={styles.slide} key={cardPosition} data-original-card={cardPosition} data-active={active}
            role="group" aria-roledescription={msg('OriginalsCarousel.slide')} aria-label={`${index + 1} / ${slides.length}`} aria-hidden={!active}>
            <div className={styles.surface} inert={!active}>
              <HomePicture photo={originalPhotos[index]} sizes="(max-width: 767px) 82vw, 72vw" />
              <div className={styles.shade} />
              <div className={styles.copy}><h3>{slide.title}</h3><p>{slide.description}</p>
                <LandingLink href={slide.href} tone="light">{copy.explore}<LandingArrow /></LandingLink>
              </div>
            </div>
          </div>;
        })}
      </div>
    </div>
    <div className={styles.dots}>{slides.map((slide, index) => <button type="button" key={slide.title}
      data-original-dot={index} aria-label={`${copy.controls.showSlide} ${index + 1}`}
      aria-pressed={selected === index} onClick={() => go(index)} />)}</div>
  </div>;
}
