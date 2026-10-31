"use client";
import Image from 'next/image';
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {LandingHeader} from '@/components/public/landing/LandingHeader';
import {LandingFooter} from '@/components/public/landing/LandingFooter';
import {LandingArrow} from '@/components/public/landing/LandingPrimitives';
import {FAQ} from '@/components/ui/FAQ/FAQ';
import {HomeSearch} from '@/features/public-home/HomeSearch';
import {homeEn} from '@/i18n/public/home-en';
import {aiToolsEn} from '@/i18n/public/ai-tools-en';
import {eventProject,eventProjectEn as t} from '@/i18n/public/event-project-en';
import {eventProjectReference as media} from '@/i18n/public/event-project-reference-en';
import Link from '@/i18n/Link';
import {EventProgramme,EventFilm,EventDiscussion,EventFeatures,EventJournal} from './EventSections';
import {EventDialogs,type EventDialogKind} from './EventDialogs';
import styles from './EventProject.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function EventLanding(){
  const {t,aiToolsEn,homeEn,media,eventProject} = usePublicCopy(publicCopySourcesForLocale,{"eventProject":"eventProject","t":"eventProjectEn","media":"eventProjectReference"});

  const[dialog,setDialog]=useState<EventDialogKind|'search'|null>(null);
  const root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),open=dialog!==null;
  const launch=(kind:EventDialogKind)=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog(kind);};
  const search=useCallback(()=>{if(!dialog){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog('search');}},[dialog]);
  useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
  useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
  return <>
    <div className={styles.page} ref={root} data-event-page>
      <a href="#public-main" className={styles.skip}>{t.skip}</a>
      <Link href="/ai" className={styles.announcement}>{t.announcement}<LandingArrow/></Link>
      <LandingHeader navigation={[...aiToolsEn.navigation.slice(0,-1),{label:t.controls.pricing,href:'/pricing'}]} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={search}/>
      <main id="public-main">
        <section className={styles.hero} aria-labelledby="event-title">
          <Image src={media.hero.src} fill alt="" sizes="100vw" preload/>
          <div className={styles.heroContent}>
            <nav className={styles.breadcrumbs} aria-label={t.breadcrumbs.label}><Link href="/">{t.breadcrumbs.home}</Link><span aria-hidden="true">›</span><a href="#project">{t.breadcrumbs.projects}</a><span aria-hidden="true">›</span><span aria-current="page">{eventProject.name}</span></nav>
            <span className={styles.heroPill}>{t.hero.label}</span><h1 id="event-title">{t.hero.title}</h1><p>{t.hero.description}</p>
            <div className={styles.heroActions}><a href="#schedule" className={styles.whiteButton}>{t.hero.primary}</a><button type="button" onClick={()=>launch('details')}>{t.hero.secondary}</button></div>
            <span className={styles.heroCredit}>{t.controls.reference}</span>
          </div>
        </section>
        <div id="project" className={styles.frame}>
          <EventProgramme/><EventFilm onTrailer={()=>launch('trailer')}/><EventDiscussion/><EventFeatures/><EventJournal/>
          <section className={styles.questions} aria-labelledby="event-faq"><h2 id="event-faq">{t.faqTitle}</h2><FAQ items={t.faq} name="event-project-faq"/></section>
          <section className={styles.closing} aria-labelledby="event-closing"><h2 id="event-closing">{t.closing.title}</h2><p>{t.closing.description}</p><button type="button" className={styles.darkButton} onClick={()=>launch('details')}>{t.closing.action}<LandingArrow/></button><span>{t.status}</span></section>
          <button type="button" className={styles.sourceAction} onClick={()=>launch('source')}>{t.source.action}<span aria-hidden="true"> ↗</span></button>
        </div>
      </main>
      <LandingFooter {...aiToolsEn.footer} layout="centered"/>
    </div>
    {dialog==='search'?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:dialog?<EventDialogs kind={dialog} onClose={()=>setDialog(null)}/>:null}
  </>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,eventProject,t,media};
