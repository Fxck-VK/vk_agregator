"use client";
import {useState} from "react";
import {CopyIcon} from "@/components/icons/CopyIcon";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {mcpEn as t,mcpPrompts} from "@/i18n/public/integration-en";
import styles from "./Integration.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText,translatePublicMessage} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export function IntegrationBriefs({onConnect}:{onConnect:()=>void}){
  const publicCopyLocale=useLocale();

  const {mcpPrompts,t} = usePublicCopy(publicCopySourcesForLocale,{"t":"mcpEn","mcpPrompts":"mcpPrompts"});

 const[selected,setSelected]=useState(0),[copying,setCopying]=useState<string|null>(null),[copied,setCopied]=useState<string|null>(null),[error,setError]=useState<string|null>(null),active=mcpPrompts[selected];
 const copy=async(id:string,text:string)=>{if(copying)return;setCopying(id);setError(null);try{await navigator.clipboard.writeText(text);setCopied(id);}catch{setCopied(null);setError(id);}finally{setCopying(null);}};
 return <section className={styles.briefs} aria-labelledby="integration-briefs-title"><div className={styles.sectionHeading}><div><h2 id="integration-briefs-title">{t.promptsTitle}</h2><p>{t.promptsDescription}</p></div><button type="button" className={styles.outline} onClick={onConnect}>{t.controls.connect}<span aria-hidden="true">→</span></button></div><div className={styles.briefGrid}><div className={styles.briefList}>{mcpPrompts.map((prompt,index)=><article key={prompt.id} data-integration-prompt data-selected={selected===index||undefined}><button type="button" className={styles.briefSelect} aria-label={`${t.controls.previewBrief} ${prompt.title}`} aria-pressed={selected===index} onClick={()=>{setSelected(index);setError(null);}}><span className={styles.briefTitle}>{prompt.title}</span><span className={styles.briefText}>{prompt.text}</span></button><button type="button" className={styles.copy} aria-label={`${t.controls.copyBrief} ${prompt.title}`} disabled={copying!==null} aria-busy={copying===prompt.id} onClick={()=>void copy(prompt.id,prompt.text)}>{copying===prompt.id?t.controls.copying:copied===prompt.id?t.controls.copied:t.controls.copy}<CopyIcon/></button></article>)}</div><div className={styles.briefPreview} role="region" aria-label={translatePublicText("Selected brief preview",publicCopyLocale)} data-selected-prompt={active.id}><ol className={styles.stages}>{active.stages.map((stage,index)=><li key={stage}><span>{index+1}.</span> {stage}<i aria-hidden="true">✧</i></li>)}</ol><MediaImage key={active.image.src} src={active.image.src} alt={translatePublicMessage('Reference visual for {title}',publicCopyLocale,{title:active.title.toLowerCase()})} width={active.image.width} height={active.image.height} fit="contain" passive showLoading={false} loading="lazy"/><p>{t.controls.reference}</p></div></div><p role="status" aria-label={translatePublicText("Brief copy status",publicCopyLocale)} className={styles.copyStatus}>{error?t.controls.copyError:copied?t.controls.copySuccess:""}</p>{error?<p className={styles.copyFallback} tabIndex={0} data-testid="integration-copy-fallback">{mcpPrompts.find(item=>item.id===error)?.text}</p>:null}<p className={styles.sourceNote}>{t.promptsNote}</p></section>;
}

const publicCopySourcesForLocale={t,mcpPrompts};
