"use client";
import Image from 'next/image';
import {useEffect,useRef,useState} from 'react';
import {LandingCarousel} from '@/components/public/landing/LandingCarousel';
import {MediaVideo} from '@/components/media/MediaVideo/MediaVideo';
import {ModelIcon} from '@/features/models/ModelIcon/ModelIcon';
import {modelIconRegistry} from '@/features/models/model-icon-registry';
import {useLandingReducedMotion} from '@/features/public-home/HomeMedia';
import {aboutEn as t,aboutModels,type AboutModel} from '@/i18n/public/about-en';
import Link from '@/i18n/Link';
import styles from './About.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

function AboutModelCard({model,paused}:{model:AboutModel;paused:boolean}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"aboutEn","aboutModels":"aboutModels"});

 const[hovered,setHovered]=useState(false),[focused,setFocused]=useState(false),reduced=useLandingReducedMotion(),active=(hovered||focused)&&!paused&&!reduced,anchor=useRef<HTMLAnchorElement>(null),video=useRef<HTMLVideoElement>(null);
 useEffect(()=>{const node=video.current,element=anchor.current;if(!active||!node||!element)return;let visible=true;const update=()=>{if(document.hidden||!visible)node.pause();else void node.play()?.catch(()=>undefined);};const bounds=element.getBoundingClientRect();visible=bounds.bottom>0&&bounds.top<window.innerHeight;const observer=typeof IntersectionObserver==='undefined'?null:new IntersectionObserver(entries=>{visible=entries.some(entry=>entry.isIntersecting);update();});observer?.observe(element);document.addEventListener('visibilitychange',update);node.addEventListener('canplay',update);update();return()=>{observer?.disconnect();document.removeEventListener('visibilitychange',update);node.removeEventListener('canplay',update);node.pause();};},[active,model.video]);
 return <Link href={`/app?model=${model.id}`} aria-label={`${t.controls.explore} ${model.name}`} ref={anchor} className={styles.modelCard} data-about-model data-active={active||undefined} data-static={!model.video||undefined} onMouseEnter={()=>setHovered(true)} onMouseLeave={()=>setHovered(false)} onFocus={()=>setFocused(true)} onBlur={event=>{if(!event.currentTarget.contains(event.relatedTarget))setFocused(false);}}><Image src={model.poster} fill alt="" sizes="240px" className={styles.modelPoster}/>{active&&model.video?<span className={styles.modelVideo}><MediaVideo src={model.video} videoRef={video} muted loop playsInline preload="none" loading="eager" fit="cover" passive aria-hidden tabIndex={-1}/></span>:null}<span className={styles.modelLabel}><ModelIcon className={styles.modelLogo} src={modelIconRegistry[model.id]}/><span>{model.name}</span></span></Link>;
}
export function AboutModels({paused}:{paused:boolean}){
  const {t,aboutModels} = usePublicCopy(publicCopySourcesForLocale,{"t":"aboutEn","aboutModels":"aboutModels"});
return <section className={styles.models} aria-labelledby="about-models"><div className={styles.wide}><h2 id="about-models">{t.models.title}</h2><p>{t.models.description}</p></div><LandingCarousel label={t.controls.models} previous={t.controls.previous} next={t.controls.next} auto paused={paused} bounded className={styles.modelStrip}>{aboutModels.map(model=><AboutModelCard key={model.id} model={model} paused={paused}/>)}</LandingCarousel><p className={styles.mediaNote}>{t.models.note}</p></section>;}

const publicCopySourcesForLocale={t,aboutModels};
