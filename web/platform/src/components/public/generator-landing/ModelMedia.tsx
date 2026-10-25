"use client";

import Image from "next/image";
import { useEffect, useRef, useState } from "react";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { klingEn as copy } from "@/i18n/public/kling-en";
import Link from "@/i18n/Link";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import styles from "./ModelLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


type AssetBase = "model-family" | "model-version" | "marketing-audience" | "workflows";
export type ModelImage = {readonly file:string; readonly alt:string};
export type ModelExample = ModelImage & {readonly poster?:string; readonly model:string};
export function ModelPicture({image,className,sizes="(max-width: 820px) 90vw, 40vw",preload=false,decorative=false,assetBase="model-family"}:{assetBase?:AssetBase;image:ModelImage;className?:string;sizes?:string;preload?:boolean;decorative?:boolean}) {
  return <Image src={`/assets/images/${assetBase}/${image.file}`} alt={decorative?"":image.alt} fill sizes={sizes} preload={preload} className={className}/>;
}

export function ModelVideo({clip,paused,controls=false,muted=true,assetBase="model-family"}:{assetBase?:AssetBase;clip:{readonly file:string;readonly poster?:string};paused:boolean;controls?:boolean;muted?:boolean}) {
  const ref=useRef<HTMLVideoElement>(null);
  const [visible,setVisible]=useState(false);
  useEffect(()=>{
    const video=ref.current;
    if(!video) return;
    const motion=window.matchMedia?.("(prefers-reduced-motion: reduce)");
    const update=()=>{ if(paused||document.hidden||motion?.matches||!visible) video.pause(); else void video.play()?.catch(()=>{}); };
    update(); document.addEventListener("visibilitychange",update);motion?.addEventListener("change",update);
    return ()=>{document.removeEventListener("visibilitychange",update);motion?.removeEventListener("change",update);video.pause();};
  },[paused,visible]);
  useEffect(()=>{
    if(typeof IntersectionObserver==="undefined") return;
    const observer=new IntersectionObserver(entries=>setVisible(entries.some(entry=>entry.isIntersecting)),{threshold:.05});
    if(ref.current)observer.observe(ref.current);
    return ()=>observer.disconnect();
  },[]);
  return <span className={styles.video}><MediaVideo videoRef={ref} src={`/assets/videos/${assetBase}/${clip.file}`} poster={clip.poster?`/assets/images/${assetBase}/${clip.poster}`:undefined} controls={controls} muted={muted} loop playsInline preload="metadata" loading={controls?"eager":"lazy"} fit="cover" passive={!controls} aria-hidden={!controls||undefined} tabIndex={controls?0:-1} onCanPlay={()=>{if((visible||controls)&&!paused&&!document.hidden&&!window.matchMedia?.("(prefers-reduced-motion: reduce)").matches)void ref.current?.play()?.catch(()=>{});}}/></span>;
}

export function ModelGallery({items,paused,onOpen,assetBase="model-family",title:providedTitle}:{assetBase?:AssetBase;title?:string;items:readonly ModelExample[];paused:boolean;onOpen:(item:ModelExample)=>void}) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);
  const title=providedTitle??copy.controls.gallery;

  return <section id="examples" className={layout.galleryStrip} aria-labelledby="family-gallery-title"><h2 id="family-gallery-title" className={layout.galleryTitle}>{title}</h2><div className={`${layout.marquee} ${styles.galleryZone}`} data-paused={paused||undefined}>{[0,1].map(track=><div className={`${layout.marqueeTrack} ${styles.galleryTrack}`} key={track} aria-hidden={track===1||undefined}>{items.map((item,index)=><button type="button" key={item.file} className={layout.galleryCard} tabIndex={track?-1:0} aria-label={`${copy.controls.openExample} ${index+1}: ${item.alt}`} onClick={()=>onOpen(item)}>{item.poster?<ModelVideo clip={item} paused={paused} assetBase={assetBase}/>:<ModelPicture image={item} assetBase={assetBase} className={layout.galleryImg} sizes="200px" decorative={track===1}/>}<span className={layout.galleryZoom} aria-hidden="true"><span className={layout.galleryZoomBadge}>+</span></span></button>)}</div>)}</div></section>;
}

function GallerySurface({item,close,assetBase,title,href}:{item:ModelExample;close:()=>void;assetBase:AssetBase;title:string;href:string}) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  const surface=useRef<HTMLElement>(null);
  useEffect(()=>{
    const previous=document.activeElement instanceof HTMLElement?document.activeElement:null;
    const page=document.querySelector<HTMLElement>("[data-model-family-page],[data-model-version-page]");
    const inert=page?.inert??false;if(page)page.inert=true;
    surface.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const trap=(event:KeyboardEvent)=>{if(event.key!=="Tab")return;const all=[...(surface.current?.querySelectorAll<HTMLElement>('button,a[href],video[controls]')??[])];const first=all[0],last=all.at(-1);if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus();}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus();}};
    const node=surface.current;node?.addEventListener("keydown",trap);
    return ()=>{node?.removeEventListener("keydown",trap);if(page)page.inert=inert;if(previous?.isConnected)previous.focus();};
  },[]);
  return <section ref={surface} role="dialog" aria-modal="true" aria-labelledby="family-example-title" className={styles.sheet}><h2 id="family-example-title" className={layout.srOnly}>{title}</h2><ModalCloseButton aria-label={copy.controls.close} className={styles.sheetClose} onClick={close}/><div className={styles.sheetVisual}>{item.poster?<ModelVideo clip={item} paused={false} controls assetBase={assetBase}/>:<ModelPicture image={item} assetBase={assetBase} sizes="(max-width: 600px) 90vw, 460px"/>}</div><div className={styles.sheetCaption}><p>{item.model} · {copy.controls.source}</p><p>{copy.controls.exampleNote}</p><Link href={href}>{copy.controls.view} →</Link></div></section>;
}
export function ModelGalleryDialog({item,onClose,assetBase="model-family",title:providedTitle,testId="family-gallery-backdrop",href="/app/models"}:{item:ModelExample;onClose:()=>void;assetBase?:AssetBase;title?:string;testId?:string;href?:string}) {
  const {copy}=usePublicCopy(publicCopySourcesForLocale);
  const title=providedTitle??copy.controls.example;
  return <ModalBackdrop onClose={onClose} testId={testId}>{close=><GallerySurface item={item} close={close} assetBase={assetBase} title={title} href={href}/>}</ModalBackdrop>;
}

const publicCopySourcesForLocale={copy};
