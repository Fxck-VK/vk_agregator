"use client";

import {useEffect,useRef,useState} from "react";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import type {EditorialLabels, EditorialSection} from "./types";
import Link from "@/i18n/Link";
import styles from "./Editorial.module.css";

export function EditorialIcon({kind}:{kind:"share"|"listen"|"copy"|"up"}) {
  return <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">{kind==="share"?<><circle cx="18" cy="5" r="2"/><circle cx="6" cy="12" r="2"/><circle cx="18" cy="19" r="2"/><path d="m8 11 8-5M8 13l8 5"/></>:kind==="listen"?<><path d="M5 14v-3a7 7 0 0 1 14 0v3M16 19h-4"/><rect x="3" y="12" width="4" height="7" rx="2"/><rect x="17" y="12" width="4" height="7" rx="2"/></>:kind==="copy"?<><rect x="8" y="8" width="12" height="13" rx="2"/><path d="M15 8V3H3v12h5"/></>:<path d="m5 12 7-7 7 7m-7-7v15"/>}</svg>;
}

export function EditorialOutline({entries,label}:{entries:readonly EditorialSection[];label:string}) {
  const [active,setActive]=useState(entries[0]?.id);
  const links=useRef<HTMLElement>(null);
  useEffect(()=>{
    if(typeof IntersectionObserver==="undefined")return;
    const visible=new Set<string>();
    const observer=new IntersectionObserver(observations=>{observations.forEach(e=>{if(e.isIntersecting)visible.add(e.target.id);else visible.delete(e.target.id);});const first=entries.find(s=>visible.has(s.id));if(first)setActive(first.id);},{rootMargin:"-15% 0px -55% 0px",threshold:0});
    entries.forEach(s=>{const node=document.getElementById(s.id);if(node)observer.observe(node);});return()=>observer.disconnect();
  },[entries]);
  useEffect(()=>{const node=links.current,selected=node?.querySelector<HTMLElement>('[aria-current="location"]');if(node&&selected){const top=selected.offsetTop,bottom=top+selected.offsetHeight;if(top<node.scrollTop||bottom>node.scrollTop+node.clientHeight)node.scrollTo?.({top:Math.max(0,top-node.clientHeight/2),behavior:"instant"});}},[active]);
  return <nav aria-label={label} className={styles.outline} ref={links}><p>{label}</p><ol>{entries.map(s=><li key={s.id}><a href={`#${s.id}`} aria-current={active===s.id?"location":undefined} onClick={()=>setActive(s.id)}>{s.label}</a></li>)}</ol></nav>;
}

export function EditorialShare({labels,onClose}:{labels:EditorialLabels;onClose:()=>void}) {
  const [url]=useState(()=>typeof window==="undefined"?"":`${window.location.origin}${window.location.pathname}`);
  const [status,setStatus]=useState<"idle"|"pending"|"copied"|"error">("idle");
  const copyLink=async()=>{setStatus("pending");try{if(!navigator.clipboard?.writeText)throw new Error("Clipboard unavailable");await navigator.clipboard.writeText(url);setStatus("copied");}catch{setStatus("error");}};
  return <LandingDialog title={labels.share} closeLabel={labels.close} onClose={onClose}><div className={styles.shareForm}><label htmlFor="editorial-share-url">{labels.pageLink}</label><input id="editorial-share-url" value={url} readOnly onFocus={e=>e.currentTarget.select()}/><button type="button" disabled={status==="pending"} onClick={()=>void copyLink()}><EditorialIcon kind="copy"/>{status==="pending"?labels.copying:labels.copy}</button>{status==="copied"||status==="error"?<p role="status">{status==="copied"?labels.copied:labels.copyFailure}</p>:null}</div></LandingDialog>;
}

export function EditorialListen({labels,onClose}:{labels:EditorialLabels;onClose:()=>void}) {
  return <LandingDialog title={labels.audioTitle} closeLabel={labels.close} onClose={onClose}><p className={styles.comingSoon}>{labels.comingSoon}</p><p className={styles.dialogDescription}>{labels.audioDescription}</p><Link href="/app/speech" className={styles.dialogLink}>{labels.speech}</Link></LandingDialog>;
}

export function EditorialBackToTop({label}:{label:string}) {
  const [visible,setVisible]=useState(false);
  useEffect(()=>{const update=()=>setVisible(window.scrollY>600);update();window.addEventListener('scroll',update,{passive:true});return()=>window.removeEventListener('scroll',update);},[]);
  return visible?<button type="button" className={styles.backTop} aria-label={label} onClick={()=>window.scrollTo({top:0,behavior:window.matchMedia?.('(prefers-reduced-motion: reduce)').matches?'instant':'smooth'})}><EditorialIcon kind="up"/></button>:null;
}
