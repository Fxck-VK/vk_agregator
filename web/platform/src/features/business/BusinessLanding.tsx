"use client";
import Image from 'next/image';
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {LandingHeader} from '@/components/public/landing/LandingHeader';
import {LandingFooter} from '@/components/public/landing/LandingFooter';
import {FAQ} from '@/components/ui/FAQ/FAQ';
import {HomeSearch} from '@/features/public-home/HomeSearch';
import {homeEn} from '@/i18n/public/home-en';
import {aiToolsEn} from '@/i18n/public/ai-tools-en';
import {businessEn as t} from '@/i18n/public/business-en';
import {businessReference as media} from '@/i18n/public/business-reference-en';
import {BusinessHero} from './BusinessHero';
import {BusinessExamples,BusinessFeatures} from './BusinessFeatures';
import {BusinessTrust,BusinessCredits,BusinessSetup} from './BusinessSections';
import {BusinessPricing} from './BusinessPricing';
import {BusinessInformation,type BusinessDialog} from './BusinessDialogs';
import styles from './Business.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function BusinessLanding(){
  const {t,aiToolsEn,homeEn,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"businessEn","media":"businessReference"});

 const[dialog,setDialog]=useState<BusinessDialog|null>(null),[sticky,setSticky]=useState(false),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),open=dialog!==null;
 const launch=(target:BusinessDialog)=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog(target);};
 const search=useCallback(()=>{if(!dialog){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog({kind:'search'});}},[dialog]);
 useLayoutEffect(()=>{const element=root.current;if(!element)return;const prior=element.inert;element.inert=open;return()=>{element.inert=prior;};},[open]);
 useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
 useEffect(()=>{const hero=root.current?.querySelector('[aria-labelledby="business-title"]');if(!hero||typeof IntersectionObserver==='undefined')return;const observer=new IntersectionObserver(entries=>{const entry=entries[0];setSticky(!entry.isIntersecting&&entry.boundingClientRect.bottom<0);});observer.observe(hero);return()=>observer.disconnect();},[]);
 const plan=()=>{const card=root.current?.querySelector('#business-pricing');card?.scrollIntoView({block:'start',behavior:window.matchMedia?.('(prefers-reduced-motion: reduce)').matches?'instant':'smooth'});card?.querySelector<HTMLElement>('input')?.focus({preventScroll:true});};
 return <><div ref={root} className={styles.page} data-business-page><a className={styles.skip} href="#public-main">{t.skip}</a><LandingHeader navigation={[...aiToolsEn.navigation.slice(0,-1),{label:t.controls.pricing,href:'/pricing'}]} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={search}/><main id="public-main"><BusinessHero onPlan={plan} paused={open}/><BusinessExamples paused={open}/><div className={styles.container}><BusinessFeatures/></div><BusinessTrust/><div className={styles.container}><BusinessCredits/><BusinessSetup/><BusinessPricing onQuote={(seats,annual)=>launch({kind:'quote',seats,annual})} onGuide={()=>launch({kind:'guide'})}/><section id="questions" className={styles.questions} aria-labelledby="business-faq-title"><div><span className={styles.tag}>{t.controls.faq}</span><h2 id="business-faq-title">{t.faqTitle}</h2></div><FAQ items={t.faq} name="business-faq"/></section></div><section className={styles.closing}><Image src={media.closing.src} fill alt="" sizes="100vw"/><div><h2>{t.closingTitle}</h2><button type="button" className={styles.whiteButton} onClick={plan}>{t.closingAction}</button></div></section></main><LandingFooter {...aiToolsEn.footer} layout="centered"/>{sticky?<button type="button" className={styles.stickyAction} onClick={plan}>{t.stickyAction}</button>:null}</div>{dialog?.kind==='search'?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:dialog?<BusinessInformation key={dialog.kind} dialog={dialog} onClose={()=>setDialog(null)}/>:null}</>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,t,media};
