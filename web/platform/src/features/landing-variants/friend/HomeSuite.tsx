"use client";

import { useId, useRef, useState, type KeyboardEvent } from "react";
import { LandingArrow, LandingLink, LandingSection } from "@/components/public/landing/LandingPrimitives";
import { homeSuiteLaunchLabelsEn, type HomeEnglishCopy } from "./home-copy";
import { homePresentationEn, type HomeMediaBriefKey } from "./home-copy";
import { usePublicCopy } from "@/i18n/public/use-public-copy";
import { HomeMediaPrompt } from "./HomeMedia";
import styles from "./PublicHome.module.css";

export function keyboardTab(event: KeyboardEvent<HTMLButtonElement>, index: number, count: number): number | null {
  const next = event.key === "ArrowRight" || event.key === "ArrowDown" ? (index + 1) % count : event.key === "ArrowLeft" || event.key === "ArrowUp" ? (index + count - 1) % count : event.key === "Home" ? 0 : event.key === "End" ? count - 1 : null;
  if (next !== null) event.preventDefault();
  return next;
}

const categoryBriefs: readonly HomeMediaBriefKey[] = ["suiteImage", "suiteVideo", "suiteAudio", "suiteText", "suiteInspiration"];
const platformBriefs: readonly HomeMediaBriefKey[] = ["platformWorkspace", "platformMcp", "platformAgents", "platformPlugins", "platformInspiration"];

export function HomeSuite({ copy, onTour }: { copy: HomeEnglishCopy; onTour: () => void }) {
  const launchLabels = usePublicCopy(homeSuiteLaunchLabelsEn);
  const presentation = usePublicCopy(homePresentationEn);
  const [platform, setPlatform] = useState(0);
  const [category, setCategory] = useState(0);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const categories = useRef<(HTMLButtonElement | null)[]>([]);
  const id = useId();
  const selected = copy.suite.categories[category];
  const other = platform > 0 ? copy.suite.platforms[platform - 1] : null;
  const categoryKey = selected.icon as keyof typeof presentation.categoryHeadings;
  return <LandingSection id="creative-suite" title={copy.suite.title} description={copy.suite.description} className={styles.suiteSection} align="start">
    <div className={styles.platformTabs} role="tablist" aria-label={copy.suite.tabsLabel}>
      {copy.suite.tabs.map((label, index) => <button key={label} ref={node => { tabs.current[index] = node; }} type="button" role="tab" id={`${id}-platform-${index}`} aria-controls={`${id}-platform-panel`} aria-selected={platform === index} tabIndex={platform === index ? 0 : -1} onClick={() => setPlatform(index)} onKeyDown={event => { const next = keyboardTab(event, index, copy.suite.tabs.length); if (next !== null) { setPlatform(next); tabs.current[next]?.focus(); } }}>{label}</button>)}
    </div>
    <div className={styles.suitePanel} role="tabpanel" tabIndex={0} id={`${id}-platform-panel`} aria-labelledby={`${id}-platform-${platform}`}>
      {other ? <div className={styles.platformDetail}><div className={styles.platformCopy}>{"upcoming" in other && other.upcoming ? <span className={styles.upcoming}>{copy.controls.comingSoon}</span> : null}<h3>{other.title}</h3><p>{other.description}</p><LandingLink href={other.href}>{other.action}<LandingArrow /></LandingLink></div><HomeMediaPrompt brief={platformBriefs[platform - 1]} className={styles.platformArtwork} /></div> : <>
        <div className={styles.suiteCategories} role="tablist" aria-label={copy.suite.categoriesLabel}>{copy.suite.categories.map((item, index) => <button type="button" role="tab" ref={node => { categories.current[index] = node; }} id={`${id}-category-${index}`} aria-controls={`${id}-category-panel`} aria-selected={category === index} tabIndex={category === index ? 0 : -1} key={item.name} onClick={() => setCategory(index)} onKeyDown={event => { const next = keyboardTab(event, index, copy.suite.categories.length); if (next !== null) { setCategory(next); categories.current[next]?.focus(); } }}>{item.name}</button>)}</div>
        <div className={styles.suiteBody} id={`${id}-category-panel`} role="tabpanel" aria-labelledby={`${id}-category-${category}`} tabIndex={0}>
          <div className={styles.suiteIntro}>
            <h3>{presentation.categoryHeadings[categoryKey]}</h3><p>{selected.description}</p>
            <div className={styles.suiteActions}><LandingLink href={selected.href}>{launchLabels[selected.icon]}<LandingArrow /></LandingLink><button type="button" onClick={onTour}>{copy.suite.tour}</button></div>
            <p className={styles.suiteSupport}>{presentation.supportLine}</p>
          </div>
          <HomeMediaPrompt brief={categoryBriefs[category]} className={styles.suiteVisual} />
        </div>
      </>}
    </div>
  </LandingSection>;
}
