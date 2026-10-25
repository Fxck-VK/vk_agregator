"use client";
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {LandingHeader} from '@/components/public/landing/LandingHeader';
import {LandingFooter} from '@/components/public/landing/LandingFooter';
import {LandingDialog} from '@/components/public/landing/LandingDialog';
import {MediaVideo} from '@/components/media/MediaVideo/MediaVideo';
import {HomeSearch} from '@/features/public-home/HomeSearch';
import {homeEn} from '@/i18n/public/home-en';
import {aiToolsEn} from '@/i18n/public/ai-tools-en';
import {footballCaseStudy as study,customerStoryEn as t} from '@/i18n/public/customer-story-en';
import {customerStoryReference as media} from '@/i18n/public/customer-story-reference-en';
import {StoryHero,StoryFacts,StoryQuote,StoryCredits,StoryClosing} from './StorySections';
import {StoryFilm,StoryGallery,StoryViewer} from './StoryMedia';
import styles from './CustomerStory.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function CustomerStory(){
  const {t,aiToolsEn,homeEn,study,media} = usePublicCopy(publicCopySourcesForLocale);

 const[dialog,setDialog]=useState<'search'|'source'|'video'|null>(null),[image,setImage]=useState<number|null>(null),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),open=dialog!==null||image!==null;
 const launch=(action:()=>void)=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;action();};
 const search=useCallback(()=>{if(!dialog&&image===null){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog('search');}},[dialog,image]);
 useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
 useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
 return <><div className={styles.page} data-customer-story ref={root}><a href="#public-main" className={styles.skip}>{t.skip}</a><LandingHeader variant="editorial" navigation={[...aiToolsEn.navigation.slice(0,-1),{label:t.controls.pricing,href:'/pricing'}]} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={search}/><main id="public-main"><StoryHero/><div className={styles.frame}><div className={styles.intro}><p>{t.intro}</p></div><div className={styles.storyLayout}><StoryFacts onSource={()=>launch(()=>setDialog('source'))}/><article className={styles.body}><StoryQuote/><StoryFilm index={0}/>{t.sections.map((section,index)=><div key={section.id}><section className={styles.narrative} data-case-section id={`case-${section.id}`} aria-labelledby={`case-${section.id}-title`}><span className={styles.kicker}>{section.label}</span><h2 id={`case-${section.id}-title`}>{section.title}</h2><p>{section.body}</p><p className={styles.editorialNote}>{section.note}</p></section>{index<3?<StoryQuote editorial={index}/>:null}{index===0?<StoryFilm index={1}/>:index===1?<StoryFilm index={2}/>:index===2?<StoryGallery onView={index=>launch(()=>setImage(index))}/>:<StoryFilm index={3}/>}</div>)}<StoryCredits/></article></div></div><StoryClosing onVideo={()=>launch(()=>setDialog('video'))}/></main><LandingFooter {...aiToolsEn.footer} layout="centered"/></div>{dialog==='search'?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:null}{dialog==='source'?<LandingDialog title={t.sourceTitle} closeLabel={t.controls.close} onClose={()=>setDialog(null)}><div className={styles.sourceCopy}><span>{study.source.publisher}</span><p>{t.referenceNotice}</p><p>{t.reportedMetrics}</p><p>{t.sourceCapture}</p><p>{t.sourceNote}</p><a href={study.source.url} target="_blank" rel="noopener noreferrer">{t.originalStory}<span aria-hidden="true"> ↗</span></a></div></LandingDialog>:null}{dialog==='video'?<LandingDialog title={t.closing.videoTitle} closeLabel={t.controls.close} onClose={()=>setDialog(null)} wide><div className={styles.videoDialog}><MediaVideo src={media.video.src} poster={media.video.poster} controls playsInline preload="metadata" fit="contain" aria-label={t.closing.videoTitle}/><p>{t.closing.videoNote}</p></div></LandingDialog>:null}{image!==null?<StoryViewer index={image} onSelect={setImage} onClose={()=>setImage(null)}/>:null}</>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,study,t,media};
