"use client";

import { useId, useRef, useState, type KeyboardEvent } from "react";
import { LandingArrow, LandingLink, LandingSection, LandingToolIcon } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import Link from "@/i18n/Link";
import { HomeLoop, HomePicture } from "./HomeMedia";
import styles from "./PublicHome.module.css";

export function keyboardTab(event: KeyboardEvent<HTMLButtonElement>, index: number, count: number): number | null {
  const next = event.key === "ArrowRight" || event.key === "ArrowDown" ? (index + 1) % count : event.key === "ArrowLeft" || event.key === "ArrowUp" ? (index + count - 1) % count : event.key === "Home" ? 0 : event.key === "End" ? count - 1 : null;
  if (next !== null) event.preventDefault();
  return next;
}

export function HomeSuite({ copy, paused, onTour }: { copy: HomeEnglishCopy; paused: boolean; onTour: () => void }) {
  const [platform, setPlatform] = useState(0);
  const [category, setCategory] = useState(0);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const categories = useRef<(HTMLButtonElement | null)[]>([]);
  const id = useId();
  const selected = copy.suite.categories[category];
  const other = platform > 0 ? copy.suite.platforms[platform - 1] : null;
  return <LandingSection id="creative-suite" title={copy.suite.title} description={copy.suite.description} className={styles.suiteSection}>
    <div className={styles.platformTabs} role="tablist" aria-label={copy.suite.tabsLabel}>
      {copy.suite.tabs.map((label, index) => <button key={label} ref={node => { tabs.current[index] = node; }} type="button" role="tab" id={`${id}-platform-${index}`} aria-controls={`${id}-platform-panel`} aria-selected={platform === index} tabIndex={platform === index ? 0 : -1} onClick={() => setPlatform(index)} onKeyDown={event => { const next = keyboardTab(event, index, copy.suite.tabs.length); if (next !== null) { setPlatform(next); tabs.current[next]?.focus(); } }}>{label}</button>)}
    </div>
    <div className={styles.suitePanel} role="tabpanel" tabIndex={0} id={`${id}-platform-panel`} aria-labelledby={`${id}-platform-${platform}`}>
      {other ? <div className={styles.platformDetail}><div className={styles.platformArtwork}><HomePicture file={other.image} /></div><div className={styles.platformCopy}>{"upcoming" in other && other.upcoming ? <span className={styles.upcoming}>{copy.controls.comingSoon}</span> : null}<h3>{other.title}</h3><p>{other.description}</p><LandingLink href={other.href} tone="light">{other.action}<LandingArrow /></LandingLink></div></div> : <>
        <div className={styles.suiteVisual} id={`${id}-category-panel`} role="tabpanel" aria-labelledby={`${id}-category-${category}`} tabIndex={0}>
          {"video" in selected ? <HomeLoop key={selected.video} file={selected.video} poster={selected.image} paused={paused} /> : <HomePicture file={selected.image} sizes="(max-width: 767px) 92vw, 90vw" />}
          <Link href={selected.href} className={styles.suiteLaunch} aria-label={`${copy.explore} ${selected.name}`}><LandingArrow /></Link>
        </div>
        <div className={styles.suiteIntro}><div><h3>{copy.suite.heading}</h3><p>{copy.suite.intro}</p></div><button type="button" onClick={onTour}>{copy.suite.tour}<LandingArrow /></button></div>
        <div className={styles.suiteCategories} role="tablist" aria-label={copy.suite.categoriesLabel}>{copy.suite.categories.map((item, index) => <button type="button" role="tab" ref={node => { categories.current[index] = node; }} id={`${id}-category-${index}`} aria-controls={`${id}-category-panel`} aria-selected={category === index} tabIndex={category === index ? 0 : -1} key={item.name} onClick={() => setCategory(index)} onKeyDown={event => { const next = keyboardTab(event, index, copy.suite.categories.length); if (next !== null) { setCategory(next); categories.current[next]?.focus(); } }}><span><LandingToolIcon name={item.icon} />{item.name}</span><p>{item.description}</p><i aria-hidden="true" /></button>)}</div>
      </>}
    </div>
  </LandingSection>;
}
