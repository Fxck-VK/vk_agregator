"use client";

import { useId, useRef, useState } from "react";
import { LandingCarousel } from "@/components/public/landing/LandingCarousel";
import { LandingArrow, LandingLink, LandingSection, LandingToolIcon } from "@/components/public/landing/LandingPrimitives";
import { ModelIcon } from "@/features/models/ModelIcon/ModelIcon";
import { modelIconRegistry } from "@/features/models/model-icon-registry";
import Link from "@/i18n/Link";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { HomePicture } from "./HomeMedia";
import { keyboardTab } from "./HomeSuite";
import styles from "./PublicHome.module.css";

export function HomeModels({ copy, paused }: { copy: HomeEnglishCopy; paused: boolean }) {
  return <LandingSection id="models" title={copy.models.title} description={copy.models.description} className={styles.modelsSection}>
    <LandingCarousel label={copy.models.label} previous={copy.controls.previous} next={copy.controls.next} auto paused={paused} className={styles.modelCards}>{copy.models.items.map(model => <Link href="/app/models" key={model.name} className={styles.modelCard}><div className={styles.modelArtwork}><HomePicture file={model.image} sizes="240px" /><ModelIcon src={modelIconRegistry[model.id]} className={styles.modelLogo} /></div><span>{model.name}<LandingArrow /></span></Link>)}</LandingCarousel>
  </LandingSection>;
}

export function HomeWorkflow({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="workflow" title={copy.workflow.title} description={copy.workflow.description} align="start" action={<LandingLink href="/app">{copy.start}<LandingArrow /></LandingLink>}>
    <div className={styles.workflowGrid}>{copy.workflow.cards.map((card, index) => <Link href={card.href} key={card.title} className={styles.workflowCard} data-card={index}><div><h3>{card.title}</h3><p>{card.description}</p></div><div className={styles.workflowArtwork}><HomePicture file={card.image} /></div><span className={styles.cardArrow}><LandingArrow /></span></Link>)}</div>
  </LandingSection>;
}

export function HomeTools({ copy }: { copy: HomeEnglishCopy }) {
  const [filter, setFilter] = useState(0);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const id = useId();
  const items = copy.tools.items.filter(tool => filter === 0 || tool.category === copy.tools.filters[filter]);
  return <LandingSection id="tools" title={copy.tools.title} description={copy.tools.description} align="start">
    <div role="tablist" aria-label={copy.tools.filtersLabel} className={styles.filterTabs}>{copy.tools.filters.map((label, index) => <button type="button" role="tab" id={`${id}-filter-${index}`} aria-controls={`${id}-tool-panel`} aria-selected={filter === index} tabIndex={filter === index ? 0 : -1} ref={node => { tabs.current[index] = node; }} key={label} onClick={() => setFilter(index)} onKeyDown={event => { const next = keyboardTab(event, index, copy.tools.filters.length); if (next !== null) { setFilter(next); tabs.current[next]?.focus(); } }}>{label}</button>)}</div>
    <div role="tabpanel" aria-labelledby={`${id}-filter-${filter}`} id={`${id}-tool-panel`} tabIndex={0}>
      <LandingCarousel key={filter} label={copy.tools.label} previous={copy.controls.previous} next={copy.controls.next} className={styles.toolCards}>{items.map(tool => <Link href={tool.href} className={styles.toolCard} key={tool.title}><span className={styles.toolIcon}><LandingToolIcon name={tool.icon} /></span><h3>{tool.title}</h3><p>{tool.description}</p><LandingArrow /></Link>)}<Link href="/app/models" className={`${styles.toolCard} ${styles.allTools}`}><span>{copy.tools.all}</span><LandingArrow /></Link></LandingCarousel>
    </div>
  </LandingSection>;
}

export function HomePossibilities({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="possibilities" title={copy.possibilities.title} description={copy.possibilities.description} align="start" action={<LandingLink href="/app">{copy.start}<LandingArrow /></LandingLink>}>
    <div className={styles.possibilityGrid}>{copy.possibilities.cards.map(card => <Link href={card.href} key={card.title} className={styles.possibilityCard}><div className={styles.possibilityImage}><HomePicture file={card.image} sizes="(max-width: 767px) 90vw, 24vw" /><span><LandingArrow /></span></div><h3>{card.title}</h3><p>{card.description}</p></Link>)}</div>
  </LandingSection>;
}
