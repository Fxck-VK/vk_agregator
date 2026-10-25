"use client";

import { useState } from "react";
import { LandingCarousel } from "@/components/public/landing/LandingCarousel";
import { aiToolsEn as copy, type OverviewAction } from "@/i18n/public/ai-tools-en";
import { aiToolsReferenceEn as reference } from "@/i18n/public/ai-tools-reference-en";
import Link from "@/i18n/Link";
import { OverviewActionLink, OverviewHeading, OverviewPicture } from "./OverviewPrimitives";
import styles from "./ToolsOverview.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText,translatePublicMessage} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



export function OverviewPlugins({ onPreview }: { onPreview: (item: OverviewAction) => void }) {
  const publicCopyLocale=useLocale();

  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="plugins" className={styles.carouselSection}>
    <OverviewHeading title={copy.plugins.title} description={copy.plugins.description} />
    <LandingCarousel label={copy.plugins.title} previous={copy.controls.previous} next={copy.controls.next} className={styles.pluginTrack} bounded>
      {copy.plugins.items.map(plugin => <button type="button" className={styles.pluginCard} key={plugin.name} onClick={() => onPreview({ title:plugin.name, description:translatePublicMessage('A planned {plugin} integration for Epifera.',publicCopyLocale,{plugin:plugin.name}), planned:true })}>
        <span className={styles.editorMark} aria-hidden="true">{plugin.mark}</span><span className={styles.pluginTitle}>{plugin.name}</span><span className={styles.pluginDescription}>{translatePublicText("Explore ",publicCopyLocale)}{plugin.name} {translatePublicText(" with Epifera",publicCopyLocale)}</span><span className={styles.pluginStatus}>{copy.controls.comingSoon}</span>
      </button>)}
    </LandingCarousel>
  </section>;
}

export function OverviewTeams() {
  const publicCopyLocale=useLocale();

  const {reference,copy} = usePublicCopy(publicCopySourcesForLocale);

  const [audience,setAudience] = useState(0);
  const cards = reference.outcomes.map((image,index) => ({ ...image, ...copy.teams.cards[index] }));
  const ordered = [...cards.slice(audience),...cards.slice(0,audience)];
  return <section id="teams" className={styles.teamsSection}>
    <OverviewHeading title={copy.teams.title} description={copy.teams.description} />
    <div className={styles.audienceFilters} role="group" aria-label={translatePublicText("Creative audiences",publicCopyLocale)}>{copy.teams.audiences.map((item,index) => <button type="button" key={item} aria-pressed={audience === index} onClick={() => setAudience(index)}>{item}</button>)}</div>
    <LandingCarousel key={audience} label={copy.teams.title} previous={copy.controls.previous} next={copy.controls.next} className={styles.scenarioTrack} bounded>
      {ordered.map(card => <Link href={card.href} key={card.file} className={styles.scenarioCard} data-audience-card><div className={styles.scenarioImage}><OverviewPicture image={card} /></div><h3>{card.title}</h3><p>{card.description}</p></Link>)}
    </LandingCarousel>
  </section>;
}

export function OverviewDocumentation({ onPreview }: { onPreview: (item: OverviewAction) => void }) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="docs" className={styles.docsSection}>
    <OverviewHeading title={copy.docs.title} description={copy.docs.description} />
    <ul className={styles.docsGrid}>{copy.docs.items.map(item => <li key={item.title}><OverviewActionLink item={item} onPreview={onPreview} className={styles.docCard}><h3>{item.title}</h3><p>{item.description}</p>{"planned" in item && item.planned ? <small>{copy.controls.comingSoon}</small> : null}<span aria-hidden="true">↗</span></OverviewActionLink></li>)}</ul>
    <Link href="/app/models" className={styles.viewMore}>{copy.docs.more}<span aria-hidden="true">→</span></Link>
  </section>;
}

export function OverviewShowcase({ onPreview }: { onPreview: (item: OverviewAction) => void }) {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="explore" className={styles.carouselSection}>
    <OverviewHeading title={copy.showcase.title} description={copy.showcase.description} />
    <LandingCarousel label={copy.showcase.title} previous={copy.controls.previous} next={copy.controls.next} className={styles.showcaseTrack} bounded>
      {reference.showcase.map((image,index) => {
        const item: OverviewAction = { title:image.title, ...copy.showcase.items[index] };
        return <OverviewActionLink key={image.file} item={item} onPreview={onPreview} className={styles.showcaseCard} data-tool-artwork>
          <OverviewPicture image={image} /><span className={styles.showcaseShade} /><span className={styles.showcaseText}><h3>{image.title}</h3><span>{item.description}</span>{item.planned ? <small>{copy.controls.comingSoon}</small> : null}</span>
        </OverviewActionLink>;
      })}
    </LandingCarousel>
  </section>;
}

export function OverviewClosing() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="start" className={styles.closing}><h2>{copy.closing.title}</h2><p>{copy.closing.description}</p><Link href="/login?mode=register">{copy.closing.action}<span aria-hidden="true">→</span></Link></section>;
}

const publicCopySourcesForLocale={copy,reference};
