"use client";
import {ModelPicture} from "@/components/public/generator-landing/ModelMedia";
import {workflowEn as copy,type WorkflowRecord} from "@/i18n/public/workflow-en";
import {workflowReferenceEn as media} from "@/i18n/public/workflow-reference-en";
import styles from "./Workflow.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";


export function WorkflowHero({workflow,onExplore}:{workflow:WorkflowRecord;onExplore:(step?:number)=>void}){
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale,{"copy":"workflowEn","media":"workflowReferenceEn"});
  const locale=useLocale();

  return <section className={styles.hero} aria-labelledby="workflow-title">
    <div className={styles.heroCopy}><h1 id="workflow-title">{workflow.title}</h1><p>{workflow.description}</p><button type="button" className={styles.primary} onClick={()=>onExplore()}>{copy.hero.action}</button></div>
    <div className={styles.canvas}>
      <svg className={`${styles.connections} ${styles.desktopLines}`} viewBox="0 0 1264 960" fill="none" preserveAspectRatio="none" aria-hidden="true"><path d="M200 270C330 270 290 870 445 870H1040" stroke="#6b5ec4"/><path d="M175 500C360 660 340 720 600 720S750 570 1000 570" stroke="#386e4c"/><path d="M620 770C750 770 780 555 1000 555" stroke="#386e4c"/></svg>
      <svg className={`${styles.connections} ${styles.mobileLines}`} viewBox="0 0 1264 960" fill="none" aria-hidden="true"><path d="M300 240C460 240 340 480 480 480S740 260 900 260" stroke="#6b5ec4"/><path d="M200 390C300 600 390 720 900 720" stroke="#386e4c"/><path d="M400 720C700 720 680 360 900 360" stroke="#386e4c"/></svg>
      <button type="button" className={`${styles.imageNode} ${styles.inputNode}`} onClick={()=>onExplore(0)} aria-label={copy.hero.inspectBrief}><span>{copy.hero.input}</span><div><ModelPicture assetBase="workflows" image={{...media.input,alt:translatePublicText("Reference trainers on a white background",locale)}} preload sizes="(max-width:767px) 25vw,320px"/></div><i aria-hidden="true"/></button>
      <button type="button" className={`${styles.imageNode} ${styles.resultOne}`} onClick={()=>onExplore(1)} aria-label={copy.hero.inspectImage}><span>{copy.hero.result}</span><div><ModelPicture assetBase="workflows" image={{...media.resultOne,alt:translatePublicText("Reference fashion image with trainers and a blue sky",locale)}} preload sizes="(max-width:767px) 30vw,340px"/></div><i aria-hidden="true"/></button>
      <button type="button" className={`${styles.imageNode} ${styles.resultTwo}`} onClick={()=>onExplore(3)} aria-label={copy.hero.inspectReview}><span>{copy.hero.result}</span><div><ModelPicture assetBase="workflows" image={{...media.resultTwo,alt:translatePublicText("Reference detail of trainers against a blue background",locale)}} sizes="(max-width:767px) 25vw,252px"/></div><i aria-hidden="true"/></button>
      <div className={`${styles.textNode} ${styles.briefNode}`}><span>✧ {copy.hero.brief}</span><p>{copy.hero.note}</p><i aria-hidden="true"/></div>
      <div className={`${styles.textNode} ${styles.reviewNode}`}><span>{copy.hero.review}</span><p>{copy.hero.reviewNote}</p></div>
      {copy.hero.tags.map((label,index)=><span className={styles.stageTag} data-stage={index} key={label}>{label}</span>)}
    </div>
  </section>;
}

const publicCopySourcesForLocale={copy,media};
