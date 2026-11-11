"use client";

import { FAQ } from "@/components/ui/FAQ/FAQ";
import { SubscriptionPlanCards } from "@/components/layout/WorkspaceHeader/SubscriptionPlanCards";
import { useRouter } from "@/i18n/navigation";
import { LandingArrow, LandingLink, LandingSection } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { HomePicture } from "./HomeMedia";
import { OriginalsCarousel } from "./OriginalsCarousel";
import styles from "./PublicHome.module.css";

export function HomeOriginals({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="originals" title={copy.originals.title} description={copy.originals.description} align="start" className={styles.originalsSection} action={<LandingLink href="/app/inspiration">{copy.originals.action}<LandingArrow /></LandingLink>}>
    <OriginalsCarousel copy={copy} />
  </LandingSection>;
}

export function HomeTeams({ copy }: { copy: HomeEnglishCopy }) {
  return <section id="teams" className={styles.teams} aria-labelledby="teams-heading"><div className={styles.teamsInner}>
    <h2 id="teams-heading">{copy.teams.title}</h2><p className={styles.teamsIntro}>{copy.teams.description}</p>
    <div className={styles.teamOptions}><div><h3>{copy.teams.personalText}</h3><LandingLink href="/app" tone="light">{copy.start}<LandingArrow /></LandingLink></div></div>
    <div className={styles.teamBenefits}>{copy.teams.benefits.map((text, index) => <div key={text}><span aria-hidden="true">{["✳", "◈", "✦"][index]}</span><p>{text}</p></div>)}</div>
  </div></section>;
}

export function HomePlans({ copy }: { copy: HomeEnglishCopy }) {
  const router = useRouter();
  return <LandingSection id="plans" title={copy.plans.title} description={copy.plans.description}>
    <div className={styles.subscriptionPlans} data-color-scheme="dark">
      <SubscriptionPlanCards onActivate={() => router.push("/app")} />
    </div>
  </LandingSection>;
}

export function HomeQuestions({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="questions" title={copy.faq.title} className={styles.questions} action={<a className={styles.support} href="mailto:support@epifera.com">{copy.faq.support}<LandingArrow /></a>}><FAQ items={copy.faq.items} name="public-home-questions" /></LandingSection>;
}

export function HomeClosing({ copy }: { copy: HomeEnglishCopy }) {
  return <section className={styles.closing} aria-labelledby="closing-heading"><div className={styles.closingArtwork}><HomePicture photo="closing" sizes="100vw" /></div><div><h2 id="closing-heading">{copy.closing}</h2><LandingLink href="/app" tone="light">{copy.start}<LandingArrow /></LandingLink></div><a href="#" className={styles.backToTop} aria-label={copy.controls.backToTop} onClick={event => {
    event.preventDefault();
    window.scrollTo({ top: 0, behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth" });
  }}>↑</a></section>;
}
