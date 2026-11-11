"use client";

import { LandingArrow, LandingLink, LandingSection } from "@/components/public/landing/LandingPrimitives";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { HomePicture } from "./HomeMedia";
import { possibilityPhotos } from "./photos";
import { ModelsMarquee } from "./ModelsMarquee";
import { ToolsMarquee } from "./ToolsMarquee";
import styles from "./PublicHome.module.css";

export function HomeModels({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="models" title={copy.models.title} description={copy.models.description} className={styles.modelsSection}>
    <ModelsMarquee copy={copy} />
  </LandingSection>;
}

export { HomeWorkflow } from './HomeWorkflow';

export function HomeTools({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="tools" title={copy.tools.title} description={copy.tools.description} align="start" className={styles.toolsSection}>
    <div className={styles.toolsPanel}>
      <ToolsMarquee copy={copy} items={copy.tools.items} />
    </div>
  </LandingSection>;
}

export function HomePossibilities({ copy }: { copy: HomeEnglishCopy }) {
  return <LandingSection id="possibilities" title={copy.possibilities.title} description={copy.possibilities.description} align="start" className={styles.possibilitiesSection} action={<LandingLink href="/app">{copy.start}<LandingArrow /></LandingLink>}>
    <div className={styles.possibilityViewport}>
    <MasonryGrid className={styles.possibilityGrid} aria-labelledby="possibilities-heading">
      {copy.possibilities.cards.map((card, index) => <li key={card.title}>
        <article className={styles.possibilityCard}>
          <div className={styles.possibilityImage}>
            <HomePicture photo={possibilityPhotos[index]} sizes="(max-width: 1050px) 45vw, 25vw" />
            <div className={styles.possibilityCopy}><h3>{card.title}</h3><p>{card.description}</p></div>
          </div>
        </article>
      </li>)}
    </MasonryGrid>
    </div>
  </LandingSection>;
}
