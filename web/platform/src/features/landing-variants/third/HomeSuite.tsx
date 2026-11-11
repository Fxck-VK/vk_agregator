"use client";

import { useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { LandingArrow, LandingLink, LandingSection, LandingToolIcon } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import Link from "@/i18n/Link";
import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { HomePicture } from "./HomeMedia";
import { platformPhotos, suitePhotos, suitePhotoSizes } from "./photos";
import styles from "./HomeSuite.module.css";

function photoGeometry(photo: keyof typeof suitePhotoSizes): CSSProperties {
  const { width, height } = suitePhotoSizes[photo];
  return { '--suite-photo-ratio': width / height } as CSSProperties;
}

export function keyboardTab(event: KeyboardEvent<HTMLButtonElement>, index: number, count: number): number | null {
  const next = event.key === "ArrowRight" || event.key === "ArrowDown" ? (index + 1) % count : event.key === "ArrowLeft" || event.key === "ArrowUp" ? (index + count - 1) % count : event.key === "Home" ? 0 : event.key === "End" ? count - 1 : null;
  if (next !== null) event.preventDefault();
  return next;
}

export function HomeSuite({ copy, onTour }: { copy: HomeEnglishCopy; onTour: () => void }) {
  const [platform, setPlatform] = useState(0);
  const [categoryIndex, setCategory] = useState(0);
  const categories = useRef<(HTMLButtonElement | null)[]>([]);
  const categoryList = useRef<HTMLDivElement>(null);
  const id = useId();
  const visibleTabs = copy.suite.tabs.map((label, index) => ({ label, index })).filter(({ index }) => {
    const item = copy.suite.platforms[index - 1];
    return !item || !("upcoming" in item && item.upcoming);
  });
  const activePlatform = visibleTabs.some(tab => tab.index === platform) ? platform : 0;
  const visibleCategories = copy.suite.categories.map((item, index) => ({ ...item, photo: suitePhotos[index] })).filter(item => item.href !== "/app/inspiration");
  const category = visibleCategories[categoryIndex] ? categoryIndex : 0;
  const selected = visibleCategories[category];
  useLayoutEffect(() => {
    const node = categoryList.current;
    if (!node) return;
    let disposed = false, frame = 0, measuredWidth = -1;
    const measure = () => {
      if (disposed) return;
      measuredWidth = node.clientWidth;
      const buttons = Array.from(node.querySelectorAll('button'));
      const labelsHeight = buttons.reduce((height, button) => {
        const css = getComputedStyle(button);
        return height + button.firstElementChild!.getBoundingClientRect().height + (parseFloat(css.paddingTop) || 0) + (parseFloat(css.paddingBottom) || 0);
      }, 0);
      const descriptionHeight = Math.max(0, ...buttons.map(button => button.querySelector('[data-suite-description]')?.firstElementChild?.scrollHeight ?? 0));
      // Reserve the longest description so switching a short caption does not
      // resize or move the photo in the wide layout.
      node.style.minBlockSize = `${Math.ceil(labelsHeight + descriptionHeight)}px`;
    };
    measure();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => {
      if (node.clientWidth === measuredWidth) return;
      cancelAnimationFrame(frame); frame = requestAnimationFrame(measure);
    });
    observer?.observe(node);
    window.addEventListener('resize', measure);
    document.fonts?.ready.then(measure);
    return () => { disposed = true; observer?.disconnect(); window.removeEventListener('resize', measure); cancelAnimationFrame(frame); };
  }, [activePlatform, copy.suite.categories]);
  return <LandingSection id="creative-suite" title={copy.suite.title} description={copy.suite.description} className={styles.suiteSection}>
    <div className={styles.suiteViewport} data-viewport-block="suite">
    <ModeSwitchPanel className={styles.platformTabs} fullWidth semantics="tabs" ariaLabel={copy.suite.tabsLabel} activeID={String(activePlatform)} onChange={value => setPlatform(Number(value))} items={visibleTabs.map(({ label, index }) => ({ id: String(index), label, elementID: `${id}-platform-${index}`, ariaControls: `${id}-platform-panel` }))} />
    <div className={styles.suitePanel} role="tabpanel" tabIndex={0} id={`${id}-platform-panel`} aria-labelledby={`${id}-platform-${activePlatform}`}>
      <div className={`${styles.suiteContent} ${styles.suiteSurface}`} data-suite-content data-suite-surface="0" data-active={activePlatform === 0} aria-hidden={activePlatform !== 0} inert={activePlatform !== 0}>
        <div className={styles.suiteSidebar} data-suite-sidebar>
          <div className={styles.suiteIntro}><h3>{copy.suite.heading}</h3><p>{copy.suite.intro}</p><button type="button" onClick={onTour}>{copy.suite.tabs[1]}<LandingArrow /></button></div>
          <div ref={categoryList} className={styles.suiteCategories} role="tablist" aria-orientation="vertical" aria-label={copy.suite.categoriesLabel}>
            {visibleCategories.map((item, index) => <button type="button" role="tab" ref={node => { categories.current[index] = node; }} id={`${id}-category-${index}`} aria-controls={`${id}-category-panel`} aria-label={item.name} aria-describedby={category === index ? `${id}-category-description-${index}` : undefined} aria-selected={category === index} tabIndex={category === index ? 0 : -1} key={item.name} onClick={() => setCategory(index)} onKeyDown={event => { const next = keyboardTab(event, index, visibleCategories.length); if (next !== null) { setCategory(next); categories.current[next]?.focus({ preventScroll: true }); } }}>
              <span className={styles.categoryLabel}><LandingToolIcon name={item.icon} />{item.name}</span>
              <span className={styles.categoryDescription} data-suite-description aria-hidden={category !== index}><span><span className={styles.descriptionText} id={`${id}-category-description-${index}`}>{item.description}</span><i aria-hidden="true" /></span></span>
            </button>)}
          </div>
        </div>
        <div className={styles.suiteVisual} data-suite-visual data-suite-media-slot style={photoGeometry(selected.photo)} id={`${id}-category-panel`} role="tabpanel" aria-labelledby={`${id}-category-${category}`} tabIndex={0}>
          {visibleCategories.map(({ photo }, index) => <div key={photo} className={`${styles.photoFrame} ${styles.suiteFrame}`} style={photoGeometry(photo)} data-suite-photo-frame data-suite-frame={index} data-active={category === index} aria-hidden={category !== index}><HomePicture photo={photo} original /></div>)}
          <Link href={selected.href} className={styles.suiteLaunch} aria-label={`${copy.explore} ${selected.name}`} />
        </div>
      </div>
      {visibleTabs.filter(tab => tab.index > 0).map(({ index }) => {
        const item = copy.suite.platforms[index - 1];
        const active = activePlatform === index;
        return <div key={index} className={`${styles.platformDetail} ${styles.suiteSurface}`} data-suite-platform data-suite-surface={index} data-active={active} aria-hidden={!active} inert={!active}>
          <div className={styles.platformCopy}><h3>{item.title}</h3><p>{item.description}</p><LandingLink href={item.href} tone="light">{item.action}<LandingArrow /></LandingLink></div>
          <div className={styles.platformArtwork} data-suite-media-slot style={photoGeometry(platformPhotos[index - 1])}><div className={styles.photoFrame} data-suite-photo-frame><HomePicture photo={platformPhotos[index - 1]} original /></div></div>
        </div>;
      })}
    </div>
    </div>
  </LandingSection>;
}
