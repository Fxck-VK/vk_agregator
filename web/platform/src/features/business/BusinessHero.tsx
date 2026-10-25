"use client";
import Image from 'next/image';
import {useEffect,useRef,useState,type CSSProperties} from 'react';
import {MediaVideo} from '@/components/media/MediaVideo/MediaVideo';
import {useLandingReducedMotion} from '@/features/public-home/HomeMedia';
import {businessEn as t} from '@/i18n/public/business-en';
import {businessReference as media} from '@/i18n/public/business-reference-en';
import styles from './Business.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export function BusinessHero({onPlan,paused}:{onPlan:()=>void;paused:boolean}){
  const publicCopyLocale=useLocale();

  const {media,t} = usePublicCopy(publicCopySourcesForLocale);

 const video=useRef<HTMLVideoElement>(null),[userPaused,setUserPaused]=useState(false),reduced=useLandingReducedMotion(),stopped=userPaused||reduced||paused;
 useEffect(()=>{const node=video.current;if(!node)return;let visible=true;const update=()=>{if(stopped||document.hidden||!visible)node.pause();else void node.play().catch(()=>undefined);};const observer=typeof IntersectionObserver==='undefined'?null:new IntersectionObserver(entries=>{visible=entries.some(e=>e.isIntersecting);update();});observer?.observe(node);document.addEventListener('visibilitychange',update);node.addEventListener('canplay',update);update();return()=>{observer?.disconnect();document.removeEventListener('visibilitychange',update);node.removeEventListener('canplay',update);node.pause();};},[stopped]);
 return <section className={styles.hero} aria-labelledby="business-title"><div className={styles.heroMedia} aria-hidden="true"><Image src={media.wallpaper.src} fill alt="" sizes="100vw" preload/><MediaVideo src={media.video.src} poster={media.video.poster} videoRef={video} fit="cover" autoPlay={!stopped} muted loop playsInline preload="metadata" tabIndex={-1} passive/><div className={styles.heroMosaic} style={{'--mosaic':`url(${media.mosaic.src})`} as CSSProperties}>{Array.from({length:36},(_,index)=><i key={index} style={{backgroundPosition:`${(index%6)*20}% ${Math.floor(index/6)*20}%`}}/>)}</div></div><div className={styles.cursors} aria-hidden="true"><span>{translatePublicText("↖ Create",publicCopyLocale)}</span><span>{translatePublicText("↖ Review",publicCopyLocale)}</span><span>{translatePublicText("↖ Concept",publicCopyLocale)}</span></div><div className={styles.heroCopy}><span className={styles.tag}>{t.label}</span><h1 id="business-title">{t.title}</h1><p>{t.description}</p><button type="button" className={styles.whiteButton} onClick={onPlan}>{t.action}</button></div><div className={styles.heroCaption}><span>{t.referenceNote}</span><button type="button" disabled={reduced} aria-label={userPaused?t.play:t.pause} aria-pressed={userPaused} onClick={()=>setUserPaused(v=>!v)}>{userPaused?'▶':'Ⅱ'}</button></div></section>;
}

const publicCopySourcesForLocale={t,media};
