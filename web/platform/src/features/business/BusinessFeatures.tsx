"use client";
import Image from 'next/image';
import {useEffect,useRef,useState} from 'react';
import {ScrollArea} from '@/components/ui/ScrollArea/ScrollArea';
import {LandingCarousel} from '@/components/public/landing/LandingCarousel';
import {LandingArrow,LandingToolIcon} from '@/components/public/landing/LandingPrimitives';
import Link from '@/i18n/Link';
import {businessEn as t} from '@/i18n/public/business-en';
import {businessReference as media} from '@/i18n/public/business-reference-en';
import styles from './Business.module.css';
import {translatePublicMessage} from '@/i18n/public/public-copy';
import {useLocale} from '@/i18n/LocaleProvider';
import {usePublicCopy} from "@/i18n/public/use-public-copy";



export function BusinessExamples({paused}:{paused:boolean}){
  const {t,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"businessEn","media":"businessReference"});
  const examples=[media['example-1'],media['example-2'],media['example-3'],media['example-4'],media['example-5'],media['example-6'],media['example-7'],media['example-8']];
return <section className={styles.examples} aria-labelledby="business-examples"><h2 id="business-examples">{t.examplesTitle}</h2><p>{t.examplesDescription} <Link href="/customers/football-campaign">{t.storyLink} ↗</Link></p><LandingCarousel label={t.controls.scenarios} previous={t.controls.previous} next={t.controls.next} bounded auto paused={paused} className={styles.exampleStrip}>{t.examples.map((example,index)=><Link href={example.href} className={styles.exampleCard} key={example.title}><div><Image src={examples[index].src} fill alt="" sizes="300px"/><span aria-hidden="true">↗</span></div><h3>{example.title}</h3><p>{example.description}</p><small>{t.controls.reference}</small></Link>)}</LandingCarousel></section>;}
export function BusinessFeatures(){
  const publicCopyLocale=useLocale();
  const {t,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"businessEn","media":"businessReference"});
  const features=[media['feature-1'],media['feature-2'],media['feature-3'],media['feature-4'],media['feature-5']];

 const[selected,setSelected]=useState('create'),sections=useRef<(HTMLElement|null)[]>([]),links=useRef<(HTMLAnchorElement|null)[]>([]);
 useEffect(()=>{if(typeof IntersectionObserver==='undefined')return;const observer=new IntersectionObserver(entries=>{const entry=entries.filter(e=>e.isIntersecting).sort((a,b)=>a.boundingClientRect.top-b.boundingClientRect.top)[0];if(entry){const id=entry.target.getAttribute('data-business-feature');if(id)setSelected(id);}},{rootMargin:'-18% 0px -50% 0px',threshold:0});sections.current.forEach(node=>{if(node)observer.observe(node);});return()=>observer.disconnect();},[]);
 useEffect(()=>{const link=links.current[t.features.findIndex(f=>f.id===selected)],list=link?.parentElement;if(link&&list){const left=link.offsetLeft-list.offsetLeft;list.scrollTo?.({left:Math.max(0,left-(list.clientWidth-link.offsetWidth)/2),behavior:window.matchMedia?.('(prefers-reduced-motion: reduce)').matches?'instant':'smooth'});}},[selected,t.features]);
 return <section className={styles.features} aria-labelledby="business-features-title"><h2 id="business-features-title">{t.featuresTitle}</h2><p className={styles.sectionIntro}>{t.featuresDescription}</p><div className={styles.featureGrid}><nav aria-label={t.featuresNavigation} className={styles.featureNav}><ScrollArea orientation="horizontal" viewportClassName={styles.featureNavList}>{t.features.map((feature,index)=><a key={feature.id} ref={node=>{links.current[index]=node;}} href={`#business-feature-${feature.id}`} aria-current={selected===feature.id?'location':undefined} onClick={event=>{event.preventDefault();setSelected(feature.id);sections.current[index]?.scrollIntoView({block:'start',behavior:window.matchMedia?.('(prefers-reduced-motion: reduce)').matches?'instant':'smooth'});}}><LandingToolIcon name={feature.icon}/>{feature.label}</a>)}</ScrollArea></nav><div className={styles.featureRows}>{t.features.map((feature,index)=><article id={`business-feature-${feature.id}`} data-business-feature={feature.id} key={feature.id} ref={node=>{sections.current[index]=node;}}><div className={styles.featureLabel}><LandingToolIcon name={feature.icon}/>{feature.label}<span>{feature.status}</span></div><h3>{feature.title}</h3><p>{feature.description}</p><div className={styles.featureArt}><Image src={features[index].src} width={features[index].width} height={features[index].height} alt={translatePublicMessage('Magnific {feature} interface illustration',publicCopyLocale,{feature:feature.label.toLowerCase()})} sizes="(max-width: 1000px) 90vw, 55vw"/><span>{t.controls.illustration}</span></div><Link href={feature.href}>{feature.action}<LandingArrow/></Link></article>)}</div></div></section>;
}

const publicCopySourcesForLocale={t,media};
