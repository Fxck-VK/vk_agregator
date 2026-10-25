"use client";
import Image from 'next/image';
import {ScrollArea} from '@/components/ui/ScrollArea/ScrollArea';
import {BrandLogo} from '@/components/brand/BrandLogo/BrandLogo';
import {LandingArrow} from '@/components/public/landing/LandingPrimitives';
import Link from '@/i18n/Link';
import {footballCaseStudy as study,customerStoryEn as t} from '@/i18n/public/customer-story-en';
import {customerStoryReference as media} from '@/i18n/public/customer-story-reference-en';
import styles from './CustomerStory.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function StoryHero(){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.hero} aria-labelledby="case-title"><div className={styles.heroCopy}><span className={styles.kicker}>{t.category}</span><p className={styles.project}>{t.project}</p><h1 id="case-title">{t.title}</h1><p className={styles.subtitle}>{t.subtitle}</p><p className={styles.referenceNotice}>{t.referenceNotice}</p></div><figure className={styles.heroMedia}><Image src={media.hero.src} width={media.hero.width} height={media.hero.height} alt={t.controls.heroAlt} sizes="(max-width:1000px) 90vw, 45vw" preload/><figcaption>{t.controls.reference}</figcaption></figure></section>;}
export function StoryFacts({onSource}:{onSource:()=>void}){
  const {t,study} = usePublicCopy(publicCopySourcesForLocale);
return <aside className={styles.facts} aria-label={t.atGlance}><span className={styles.kicker}>{t.atGlance}</span><ScrollArea orientation="vertical" className={styles.factsScroll} viewportClassName={styles.factsViewport} viewportProps={{role:"region","aria-label":t.controls.facts,tabIndex:0}}><div className={styles.factsBody}><div className={styles.metrics}>{study.results.metrics.map(metric=><div key={metric.label}><strong>{metric.value}</strong><span>{metric.label}</span></div>)}</div><p className={styles.metricNotice}>{t.reportedMetrics}</p><dl>{t.facts.map(fact=><div key={fact.label}><dt>{fact.label}</dt><dd>{fact.value}</dd></div>)}</dl><button type="button" className={styles.sourceButton} onClick={onSource}>{t.sourceDetails}<span aria-hidden="true">↗</span></button><Link href="/business" className={styles.businessButton}>{t.business}<LandingArrow/></Link></div></ScrollArea></aside>;}
export function StoryQuote({editorial}:{editorial?:number}){
  const {t,study,media} = usePublicCopy(publicCopySourcesForLocale);
const note=editorial===undefined?null:t.takeaways[editorial];return <figure className={styles.quote} data-case-quote={note?'editorial':'source'}><blockquote>{note?note.text:study.quote.text}</blockquote><figcaption>{note?<span>{note.label}</span>:<><strong>{study.quote.author}</strong><span>{study.quote.role}</span></>}</figcaption>{note?<BrandLogo size={32}/>:<Image src={media.portrait.src} width={44} height={44} alt={study.quote.author}/>}</figure>;}
export function StoryCredits(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.credits} aria-labelledby="case-credits"><h2 id="case-credits">{t.creditsTitle}</h2><p>{t.creditsNote}</p><div>{t.credits.map(group=><section key={group.company} data-case-credit aria-label={group.company}><h3>{group.company}</h3><dl>{group.people.map(person=><div key={person.role+person.name}><dt>{person.role}</dt><dd>{person.name}</dd></div>)}</dl></section>)}</div></section>;}
export function StoryClosing({onVideo}:{onVideo:()=>void}){
  const {media,t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.closing} aria-labelledby="case-closing"><div className={styles.closingCard}><div className={styles.closingVisual}><Image src={media.closing.src} fill alt="" sizes="(max-width:1000px) 90vw, 45vw"/><button type="button" onClick={onVideo} aria-label={t.closing.watch}><span aria-hidden="true">▶</span>{t.closing.watch}</button><small>{t.controls.reference}</small></div><div className={styles.closingCopy}><span className={styles.kicker}>{t.closing.label}</span><h2 id="case-closing">{t.closing.title}</h2><p>{t.closing.description}</p><Link href="/business" className={styles.businessButton}>{t.business}<LandingArrow/></Link><ul>{t.closing.tags.map(tag=><li key={tag}>{tag}</li>)}</ul></div></div></section>;}

const publicCopySourcesForLocale={study,t,media};
