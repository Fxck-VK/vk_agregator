"use client";
import {useEffect,useRef} from 'react';
import {LandingDialog} from '@/components/public/landing/LandingDialog';
import {MediaVideo} from '@/components/media/MediaVideo/MediaVideo';
import {eventProjectEn as t} from '@/i18n/public/event-project-en';
import {eventProjectReference as media} from '@/i18n/public/event-project-reference-en';
import Link from '@/i18n/Link';
import styles from './EventProject.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";


function EventTrailer(){
  const {media,t} = usePublicCopy(publicCopySourcesForLocale,{"t":"eventProjectEn","media":"eventProjectReference"});

  const video=useRef<HTMLVideoElement>(null);
  useEffect(()=>{
    const node=video.current;
    const pauseWhenHidden=()=>{if(document.hidden)node?.pause();};
    document.addEventListener('visibilitychange',pauseWhenHidden);
    return()=>{document.removeEventListener('visibilitychange',pauseWhenHidden);node?.pause();};
  },[]);
  return <div className={styles.videoDialog}><MediaVideo src={media.trailer.src} poster={media['candela-cover'].src} videoRef={video} controls playsInline preload="metadata" fit="contain" aria-label={t.film.dialog}/><p>{t.film.credit}</p></div>;
}

export type EventDialogKind='details'|'source'|'trailer';
export function EventDialogs({kind,onClose}:{kind:EventDialogKind;onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"eventProjectEn","media":"eventProjectReference"});

  if(kind==='trailer')return <LandingDialog title={t.film.dialog} closeLabel={t.controls.close} onClose={onClose} wide><EventTrailer/></LandingDialog>;
  if(kind==='source')return <LandingDialog title={t.source.title} closeLabel={t.controls.close} onClose={onClose}><div className={styles.dialogCopy}><p>{t.source.description}</p><a href={t.source.href} target="_blank" rel="noopener noreferrer">{t.source.link}<span aria-hidden="true"> ↗</span></a></div></LandingDialog>;
  return <LandingDialog title={t.details.title} closeLabel={t.controls.close} onClose={onClose}><div className={styles.dialogCopy}><span className={styles.pill}>{t.status}</span><p>{t.details.description}</p><a href={t.details.href}>{t.details.contact}<span aria-hidden="true"> ↗</span></a><Link href="/ai">{t.details.tools}<span aria-hidden="true"> ↗</span></Link></div></LandingDialog>;
}

const publicCopySourcesForLocale={t,media};
