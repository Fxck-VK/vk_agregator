"use client";

import { LandingArrow, LandingLink } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import Link from "@/i18n/Link";
import { HomeMediaPrompt } from "./HomeMedia";
import styles from "./PublicHome.module.css";

export function HomeHero({ copy, paused, onPause, onIntro }: { copy: HomeEnglishCopy; paused: boolean; onPause: () => void; onIntro: () => void }) {
  return <section className={styles.hero} aria-labelledby="home-heading">
    <div className={styles.heroInner}>
      <div className={styles.heroCopy}>
        <Link href="#creative-suite" className={styles.eyebrow}>{copy.hero.eyebrow}<LandingArrow /></Link>
        <h1 id="home-heading">{copy.hero.title}</h1><p>{copy.hero.description}</p>
        <div className={styles.heroActions}><LandingLink href="/app">{copy.start}<LandingArrow /></LandingLink><button type="button" className={styles.introButton} onClick={onIntro}>{copy.why}<LandingArrow /></button></div>
      </div>
      <HomeMediaPrompt brief="hero" className={styles.heroArtwork} />
    </div>
    <div className={styles.partnerStrip}><p>{copy.hero.modelCaption}</p><div>{copy.hero.partners.map(name => <span key={name}>{name}</span>)}</div>
      <button type="button" className={styles.pauseButton} aria-label={paused ? copy.controls.play : copy.controls.pause} aria-pressed={paused} onClick={onPause}>{paused ? "▶" : "Ⅱ"}</button>
    </div>
  </section>;
}
