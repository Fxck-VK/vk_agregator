"use client";

import { LandingLink } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { HomePicture } from "./HomeMedia";
import { HeroTextAnimation } from "./HeroTextAnimation";
import styles from "./PublicHome.module.css";

export function HomeHero({ copy, onIntro }: { copy: HomeEnglishCopy; onIntro: () => void }) {
  return <section className={styles.hero} aria-labelledby="home-heading">
    <HomePicture photo="hero" sizes="(max-width: 767px) 180vh, 100vw" preload className={styles.heroImage} />
    <div className={styles.heroShade} />
    <div className={styles.heroInner}>
      <div className={styles.heroCopy}>
        <h1 id="home-heading">{copy.hero.title}</h1><p>{copy.hero.description}</p>
        <div className={styles.heroActions}><LandingLink href="/app" tone="light">{copy.start}</LandingLink><button type="button" className={styles.introButton} onClick={onIntro}><span aria-hidden="true">▶</span>{copy.why}</button></div>
      </div>
      <HeroTextAnimation phrases={copy.hero.capabilities} />
    </div>
    <div className={styles.partnerStrip}><div>{copy.hero.partners.map(name => <span key={name}>{name}</span>)}</div></div>
  </section>;
}
