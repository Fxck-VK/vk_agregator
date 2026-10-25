"use client";
import Image from 'next/image';
import {LandingArrow,LandingToolIcon} from '@/components/public/landing/LandingPrimitives';
import {aboutEn as t,aboutStats} from '@/i18n/public/about-en';
import {aboutReference as media} from '@/i18n/public/about-reference-en';
import Link from '@/i18n/Link';
import styles from './About.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

const scenarios=[media['scenario-1'],media['scenario-2'],media['scenario-3'],media['scenario-4'],media['scenario-5'],media['scenario-6'],media['scenario-7'],media['scenario-8']];
export function AboutStats(){
  const {t,aboutStats} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.stats} aria-labelledby="about-stats"><h2 id="about-stats">{t.stats.title}</h2><p>{t.stats.description}</p><dl>{aboutStats.map(stat=><div key={stat.label}><dt>{stat.value}</dt><dd>{stat.label}</dd></div>)}</dl></section>;}
export function AboutFocus(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.section} aria-labelledby="about-focus"><h2 id="about-focus">{t.people.title}</h2><p className={styles.sectionIntro}>{t.people.description}</p><div className={styles.focusGrid}>{t.people.items.map(item=><Link href={item.href} key={item.title} data-about-focus><span><strong>{item.title}</strong><span>{item.description}</span></span><LandingArrow/></Link>)}</div><a href="#contact" className={styles.textLink}>{t.people.action}<LandingArrow/></a></section>;}
export function AboutScenarios(){
  const {t,scenarios} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.section} aria-labelledby="about-scenarios"><h2 id="about-scenarios">{t.scenarios.title}</h2><p className={styles.sectionIntro}>{t.scenarios.description}</p><div className={styles.scenarioGrid}>{t.scenarios.items.map((item,index)=><Link href={item.href} key={item.title} data-about-scenario><Image src={scenarios[index].src} fill alt="" sizes="(max-width:679px) 90vw,(max-width:1000px) 45vw,33vw"/><strong>{item.title}</strong><div><p>{item.description}</p><span><LandingArrow/></span></div><small>{t.controls.reference}</small></Link>)}</div><Link href="/customers/football-campaign" className={styles.referenceBar}>{t.scenarios.action}<LandingArrow/></Link><div className={styles.assurance}><h3>{t.assurance.title}</h3><p>{t.assurance.description}</p><div>{t.assurance.items.map(item=><Link href={item.href} key={item.title} data-about-assurance><LandingToolIcon name={item.icon}/><div><strong>{item.title}</strong><p>{item.description}</p></div></Link>)}</div><Link href="/business" className={styles.textLink}>{t.teamsAction}<LandingArrow/></Link></div></section>;}
export function AboutStory(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.section} aria-labelledby="about-story"><span className={styles.eyebrow}>{t.story.label}</span><h2 id="about-story">{t.story.title}</h2><ol className={styles.timeline}>{t.story.items.map((item,index)=><li key={item.title} data-about-step><span aria-hidden="true">{String(index+1).padStart(2,'0')}</span><div><h3>{item.title}</h3><p>{item.text}</p><Link href={item.href}>{item.action}<span aria-hidden="true"> ↗</span></Link></div></li>)}</ol></section>;}
export function AboutValues(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <section className={styles.section} aria-labelledby="about-values"><span className={styles.eyebrow}>{t.values.label}</span><h2 id="about-values">{t.values.title}</h2><div className={styles.valueGrid}>{t.values.items.map((item,index)=><article key={item.title} data-about-value><span>{String(index+1).padStart(2,'0')}</span><h3>{item.title}</h3><p>{item.text}</p></article>)}</div></section>;}
export function AboutProducts({onSource}:{onSource:()=>void}){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale);
return <section className={`${styles.section} ${styles.products}`} aria-labelledby="about-products"><span className={styles.eyebrow}>{t.products.label}</span><h2 id="about-products">{t.products.title}</h2><figure><Image src={media.products.src} width={media.products.width} height={media.products.height} alt={t.products.reference} sizes="95vw"/><figcaption>{t.products.reference}</figcaption></figure><div className={styles.productLinks}>{t.products.items.map(item=><Link href={item.href} key={item.name} data-about-product>{item.name}</Link>)}</div><button type="button" className={styles.sourceAction} onClick={onSource}>{t.sourceAction}<span aria-hidden="true"> ↗</span></button></section>;}
export function AboutContacts(){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <><section className={styles.contribute} aria-labelledby="about-contribute"><div><span className={styles.pill}>{t.contribution.label}</span><h2 id="about-contribute">{t.contribution.title}</h2><p>{t.contribution.description}</p></div><a href="mailto:support@epifera.com?subject=An%20idea%20for%20Epifera" className={styles.darkButton}>{t.contribution.action}<LandingArrow/></a></section><section id="contact" className={`${styles.section} ${styles.contacts}`} aria-labelledby="about-contact"><span className={styles.eyebrow}>{t.contacts.label}</span><h2 id="about-contact">{t.contacts.title}</h2><div>{t.contacts.items.map(item=><Link href={item.href} key={item.title} data-about-contact><h3>{item.title}</h3><p>{item.description}</p><span>{item.display}<LandingArrow/></span></Link>)}</div></section></>;}

const publicCopySourcesForLocale={t,aboutStats,media,scenarios};
