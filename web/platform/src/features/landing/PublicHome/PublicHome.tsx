import Link from "next/link";

import { ru } from "@/i18n/ru";

import { CapabilitiesCarousel } from "../CapabilitiesCarousel/CapabilitiesCarousel";
import { FaqSection } from "../FaqSection/FaqSection";
import { HeroComposer } from "../HeroComposer/HeroComposer";
import { HowItWorks } from "../HowItWorks/HowItWorks";
import { NewsCarousel } from "../NewsCarousel/NewsCarousel";
import { PopularModels } from "../PopularModels/PopularModels";
import { PromptLibraryCta } from "../PromptLibraryCta/PromptLibraryCta";
import { PublicFooter } from "../PublicFooter/PublicFooter";
import { QuickTools } from "../QuickTools/QuickTools";
import { SocialCta } from "../SocialCta/SocialCta";
import { landingTools } from "../landing-content";
import styles from "./PublicHome.module.css";

export const publicHomeBlockOrder = [
  "hero",
  "quick-tools",
  "trust-strip",
  "news",
  "models",
  "how-it-works",
  "capabilities",
  "use-cases",
  "prompt-library",
  "faq",
  "social",
  "footer",
] as const;

export function PublicHome() {
  return (
    <div className={styles.home}>
      <section className={styles.hero} data-landing-block="hero">
        <p className={styles.eyebrow}>{ru.landing.heroEyebrow}</p>
        <h1>{ru.landing.heroTitle}</h1>
        <p className={styles.heroDescription}>{ru.landing.heroDescription}</p>
        <HeroComposer />
      </section>

      <section aria-labelledby="quick-tools-title" className={styles.section} data-landing-block="quick-tools">
        <h2 className={styles.srOnly} id="quick-tools-title">Быстрый выбор инструмента</h2>
        <QuickTools />
      </section>

      <section aria-label="Преимущества NeiroHub" className={styles.trustStrip} data-landing-block="trust-strip">
        <p><strong>Каталог нейросетей</strong><span>и инструментов для разных задач</span></p>
        <p><strong>1</strong><span>аккаунт для всех задач</span></p>
        <p><strong>Сразу</strong><span>видна стоимость запуска</span></p>
      </section>

      <section aria-labelledby="news-title" className={styles.section} data-landing-block="news">
        <NewsCarousel />
      </section>

      <section aria-labelledby="models-title" className={styles.section} data-landing-block="models">
        <PopularModels />
      </section>

      <section aria-labelledby="how-title" className={styles.section} data-landing-block="how-it-works">
        <HowItWorks />
      </section>

      <section aria-labelledby="capabilities-title" className={styles.section} data-landing-block="capabilities">
        <CapabilitiesCarousel />
      </section>

      <section aria-labelledby="use-cases-title" className={styles.section} data-landing-block="use-cases">
        <h2 id="use-cases-title">Нейросети для любой задачи</h2>
        <div className={styles.tagList}>
          {landingTools.slice(1, 6).map((tool) => <Link href={tool.href} key={tool.id}>{tool.name}</Link>)}
        </div>
      </section>

      <section className={styles.promptBanner} data-landing-block="prompt-library">
        <PromptLibraryCta />
      </section>

      <section aria-labelledby="faq-title" className={styles.section} data-landing-block="faq">
        <FaqSection />
      </section>

      <section className={styles.social} data-landing-block="social">
        <SocialCta />
      </section>

      <div className={styles.footer} data-landing-block="footer"><PublicFooter /></div>
    </div>
  );
}
