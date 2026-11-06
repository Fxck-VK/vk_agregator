"use client";

import { useState } from "react";
import { FAQ } from "@/components/ui/FAQ/FAQ";
import { LandingArrow, LandingLink, LandingSection } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { HomePicture } from "./HomeMedia";
import styles from "./PublicHome.module.css";

export function HomeOriginals({ copy }: { copy: HomeEnglishCopy }) {
  const [selected, setSelected] = useState(0);
  const slide = copy.originals.slides[selected];
  const move = (direction: number) => setSelected((selected + direction + copy.originals.slides.length) % copy.originals.slides.length);
  return <LandingSection id="originals" title={copy.originals.title} description={copy.originals.description} align="start" action={<LandingLink href="/app/inspiration">{copy.originals.action}<LandingArrow /></LandingLink>}>
    <div className={styles.originalsFrame} role="region" aria-label={copy.originals.title}>
      <HomePicture key={slide.image} file={slide.image} sizes="95vw" />
      <div className={styles.originalsCopy} aria-live="polite"><h3>{slide.title}</h3><p>{slide.description}</p><LandingLink href={slide.href} tone="light">{copy.explore}<LandingArrow /></LandingLink></div>
      <div className={styles.slideArrows}><button type="button" aria-label={`${copy.controls.previous}: ${copy.originals.title}`} onClick={() => move(-1)}><LandingArrow direction="left" /></button><button type="button" aria-label={`${copy.controls.next}: ${copy.originals.title}`} onClick={() => move(1)}><LandingArrow /></button></div>
      <div className={styles.slideDots}>{copy.originals.slides.map((item, index) => <button type="button" key={item.title} aria-label={`${copy.controls.showSlide} ${index + 1}`} aria-pressed={selected === index} onClick={() => setSelected(index)} />)}</div>
    </div>
  </LandingSection>;
}

export function HomeTeams({ copy }: { copy: HomeEnglishCopy }) {
  return <section id="teams" className={styles.teams} aria-labelledby="teams-heading"><div className={styles.teamsInner}><h2 id="teams-heading">{copy.teams.title}</h2><p className={styles.teamsIntro}>{copy.teams.description}</p><div className={styles.teamOptions}><div><span>{copy.teams.personal}</span><h3>{copy.teams.personalText}</h3><LandingLink href="/app" tone="light">{copy.start}<LandingArrow /></LandingLink></div><div><span>{copy.teams.business}</span><h3>{copy.teams.businessText}</h3><LandingLink href="mailto:support@epifera.com" tone="outline">{copy.teams.contact}<LandingArrow /></LandingLink></div></div><div className={styles.teamBenefits}>{copy.teams.benefits.map((text, index) => <div key={text}><span aria-hidden="true">{["✳", "◈", "✦"][index]}</span><p>{text}</p></div>)}</div></div></section>;
}

export function HomePlans({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="plans" title={copy.plans.title} description={copy.plans.description}>
    <div className={styles.planCards}>{copy.plans.cards.map((card, index) => <article key={card.name} className={styles.planCard} data-featured={index === 2 || undefined}><span className={styles.planBadge}>{card.badge}</span><h3>{card.name}</h3><p>{card.description}</p><LandingLink href={card.href} tone={index === 2 ? "light" : "dark"}>{card.action}</LandingLink><ul>{card.features.map(feature => <li key={feature}><span aria-hidden="true">✓</span>{feature}</li>)}</ul></article>)}</div>
  </LandingSection>;
}

export function HomeQuestions({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="questions" title={copy.faq.title} className={styles.questions} action={<a className={styles.support} href="mailto:support@epifera.com">{copy.faq.support}<LandingArrow /></a>}><FAQ items={copy.faq.items} name="public-home-questions" /></LandingSection>;
}

export function HomeClosing({ copy }: { copy: HomeEnglishCopy }) {
  return <section className={styles.closing} aria-labelledby="closing-heading"><div className={styles.closingArtwork}><HomePicture file="hero.webp" sizes="100vw" /></div><div><h2 id="closing-heading">{copy.closing}</h2><LandingLink href="/app" tone="light">{copy.start}<LandingArrow /></LandingLink></div><a href="#" className={styles.backToTop} aria-label={copy.controls.backToTop}>↑</a></section>;
}
