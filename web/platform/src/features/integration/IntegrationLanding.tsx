"use client";
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from "react";
import {LandingHeader} from "@/components/public/landing/LandingHeader";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {LandingArrow} from "@/components/public/landing/LandingPrimitives";
import {MediaVideo} from "@/components/media/MediaVideo/MediaVideo";
import {FAQ} from "@/components/ui/FAQ/FAQ";
import {HomeSearch} from "@/features/public-home/HomeSearch";
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {mcpIntegration as integration,mcpClients,mcpEn as t} from "@/i18n/public/integration-en";
import {integrationReference as reference} from "@/i18n/public/integration-reference-en";
import Link from "@/i18n/Link";
import {IntegrationHero} from "./IntegrationHero";
import {IntegrationCapabilities,IntegrationComparison,IntegrationFeatures} from "./IntegrationSections";
import {IntegrationBriefs} from "./IntegrationBriefs";
import styles from "./Integration.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


type Dialog="search"|"status"|"guide"|"video"|null;
export function IntegrationLanding(){
  const {mcpClients,t,aiToolsEn,homeEn,integration,reference} = usePublicCopy(publicCopySourcesForLocale);

 const[selected,setSelected]=useState(0),[dialog,setDialog]=useState<Dialog>(null),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),client=mcpClients[selected],open=dialog!==null;
 const launch=(target:Dialog)=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog(target);};
 const search=useCallback(()=>{if(!dialog){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog("search");}},[dialog]);
 useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
 useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
 const connect=()=>{const element=root.current?.querySelector<HTMLElement>('#connection');element?.scrollIntoView({block:"center",behavior:window.matchMedia?.('(prefers-reduced-motion: reduce)').matches?"instant":"smooth"});element?.querySelector<HTMLElement>('[role=tab][aria-selected=true]')?.focus({preventScroll:true});};
 return <><div ref={root} className={styles.page} data-integration-page><a className={styles.skip} href="#public-main">{t.skip}</a><Link href="/" className={styles.announcement}>{t.announcement}<LandingArrow/></Link><LandingHeader navigation={aiToolsEn.navigation} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={search}/><main id="public-main"><IntegrationHero selected={selected} onSelect={setSelected} onStatus={()=>launch("status")} onGuide={()=>launch("guide")}/><div className={styles.sections}><IntegrationCapabilities onVideo={()=>launch("video")}/><IntegrationFeatures onConnect={connect}/><IntegrationComparison/><IntegrationBriefs onConnect={connect}/><section id="questions" className={styles.questions}><h2>{t.faqTitle}</h2><FAQ items={t.faq} name="integration-faq"/></section></div></main><LandingFooter {...aiToolsEn.footer} layout="centered"/></div>
 {dialog==="search"?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:null}{dialog==="status"||dialog==="guide"?<LandingDialog title={dialog==="status"?t.statusTitle:`${t.guideTitle} — ${client.name}`} closeLabel={t.controls.close} onClose={()=>setDialog(null)}><div className={styles.guideContent}><span className={styles.plannedLabel}>{t.planned}</span><p>{t.statusNote}</p>{dialog==="guide"?<><h3>{client.name}</h3><p>{client.context}</p><ol>{client.steps.map(step=><li key={step}>{step}</li>)}</ol></>:null}<h3>{t.accessTitle}</h3><ul>{integration.accessConditions.map(item=><li key={item}>{item}</li>)}</ul><h3>{t.compatibilityTitle}</h3><ul>{integration.limitations.map(item=><li key={item}>{item}</li>)}</ul><div className={styles.guideLinks}><Link href={integration.verifiedDestination}>{t.controls.workspace}<span aria-hidden="true"> ↗</span></Link><Link href="/ai/docs">{t.controls.docs}<span aria-hidden="true"> ↗</span></Link><a href="mailto:support@epifera.com">{t.controls.support}<span aria-hidden="true"> ↗</span></a></div></div></LandingDialog>:null}{dialog==="video"?<LandingDialog title={t.controls.videoTitle} closeLabel={t.controls.close} onClose={()=>setDialog(null)} wide><div className={styles.videoDialog}><MediaVideo src={reference.video.src} poster={reference.video.poster} aria-label={t.controls.videoTitle} controls playsInline preload="metadata" fit="contain"/><p>{t.controls.videoNote}</p></div></LandingDialog>:null}</>;
}


const publicCopySourcesForLocale={homeEn,aiToolsEn,integration,mcpClients,t,reference};
