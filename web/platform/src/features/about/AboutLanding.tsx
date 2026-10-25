"use client";
import Image from 'next/image';
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {LandingHeader} from '@/components/public/landing/LandingHeader';
import {LandingFooter} from '@/components/public/landing/LandingFooter';
import {LandingDialog} from '@/components/public/landing/LandingDialog';
import {LandingArrow} from '@/components/public/landing/LandingPrimitives';
import {FAQ} from '@/components/ui/FAQ/FAQ';
import {HomeSearch} from '@/features/public-home/HomeSearch';
import {homeEn} from '@/i18n/public/home-en';
import {aiToolsEn} from '@/i18n/public/ai-tools-en';
import {aboutEn as t} from '@/i18n/public/about-en';
import {aboutReference as media} from '@/i18n/public/about-reference-en';
import Link from '@/i18n/Link';
import {AboutStats,AboutFocus,AboutScenarios,AboutStory,AboutValues,AboutProducts,AboutContacts} from './AboutSections';
import {AboutModels} from './AboutModels';
import styles from './About.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function AboutLanding(){
  const {t,aiToolsEn,homeEn,media} = usePublicCopy(publicCopySourcesForLocale);

 const[dialog,setDialog]=useState<'search'|'source'|null>(null),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),open=dialog!==null;
 const launch=(kind:'source')=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog(kind);};
 const search=useCallback(()=>{if(!dialog){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog('search');}},[dialog]);
 useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
 useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
 return <><div className={styles.page} data-about-page ref={root}><a className={styles.skip} href="#public-main">{t.skip}</a><Link href="/ai" className={styles.announcement}>{t.announcement}<LandingArrow/></Link><LandingHeader navigation={[...aiToolsEn.navigation.slice(0,-1),{label:t.controls.pricing,href:'/pricing'}]} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={search}/><main id="public-main"><section className={styles.hero} aria-labelledby="about-title"><Image src={media.hero.src} fill alt="" sizes="100vw" preload/><div><h1 id="about-title">{t.title}</h1><p>{t.description}</p><Link href="/ai" className={styles.whiteButton}>{t.discover}<LandingArrow/></Link></div></section><div className={styles.wide}><AboutStats/><AboutFocus/><AboutScenarios/><AboutStory/><AboutValues/><AboutProducts onSource={()=>launch('source')}/></div><AboutModels paused={open}/><div className={styles.wide}><section id="questions" className={styles.questions} aria-labelledby="about-faq"><h2 id="about-faq">{t.faqTitle}</h2><FAQ items={t.faq} name="about-faq"/></section><AboutContacts/></div></main><LandingFooter {...aiToolsEn.footer} layout="centered"/></div>{dialog==='search'?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:dialog==='source'?<LandingDialog title={t.sourceTitle} closeLabel={t.controls.close} onClose={()=>setDialog(null)}><div className={styles.sourceCopy}><p>{t.sourceNote}</p><a href="https://www.magnific.com/company/about-us" target="_blank" rel="noopener noreferrer">{t.sourceLink}<span aria-hidden="true"> ↗</span></a></div></LandingDialog>:null}</>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,t,media};
