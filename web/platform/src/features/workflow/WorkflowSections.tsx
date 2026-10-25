"use client";
import {LandingCarousel} from "@/components/public/landing/LandingCarousel";
import {ModelPicture} from "@/components/public/generator-landing/ModelMedia";
import {workflowEn as copy,type WorkflowRecord} from "@/i18n/public/workflow-en";
import {workflowReferenceEn as media} from "@/i18n/public/workflow-reference-en";
import Link from "@/i18n/Link";
import styles from "./Workflow.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";


export function WorkflowDiscover({onExplore}:{onExplore:()=>void}){
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale();

  return <section id="discover" className={styles.discover}><div className={styles.centerHeading}><h2>{copy.discover.title}</h2><p>{copy.discover.description}</p></div><button type="button" className={styles.onboarding} onClick={onExplore} aria-label={copy.discover.action}><ModelPicture assetBase="workflows" image={{...media.overview,alt:translatePublicText("Reference collage showing a connected creative process",locale)}} sizes="(max-width:767px) 90vw,1200px"/><span className={styles.play} aria-hidden="true"><svg viewBox="0 0 24 24" fill="currentColor"><path d="m8 4 13 8L8 20Z"/></svg></span><span className={styles.onboardingLabel}>{copy.discover.action}</span></button><p className={styles.source}>{copy.source}</p><div className={styles.models}><p>{copy.discover.modelsLabel}</p>{[0,1].map(row=><div key={row} className={styles.modelRow}>{copy.discover.models.slice(row*4,row*4+4).map(model=><Link key={model.label} href={model.href}>{model.label}</Link>)}</div>)}</div></section>;
}
export function WorkflowFeatured({onExplore}:{onExplore:()=>void}){
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale();

  return <section id="featured" className={styles.featured}><div className={styles.heading}><h2>{copy.featured.title}</h2><p>{copy.featured.description}</p></div><div className={styles.featuredBanner}><div className={styles.featuredArt}><ModelPicture assetBase="workflows" image={{...media.input,alt:translatePublicText("Reference product trainers",locale)}} sizes="(max-width:767px) 45vw,350px"/></div><div className={styles.featuredCopy}><h2>{copy.featured.heading}</h2><p>{copy.featured.body}</p><button type="button" className={styles.primary} onClick={onExplore}>{copy.featured.action}</button></div><div className={`${styles.featuredArt} ${styles.featuredResult}`}><ModelPicture assetBase="workflows" image={{...media.resultTwo,alt:translatePublicText("Reference fashion product detail",locale)}} sizes="(max-width:767px) 45vw,350px"/></div></div></section>;
}
export function WorkflowFeatures(){
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale();

  return <section id="process" className={styles.features}><div className={styles.heading}><h2>{copy.features.title}</h2><p>{copy.features.description}</p></div><div className={styles.featureGrid}>
    <article><div className={styles.featureMedia}><ModelPicture assetBase="workflows" image={{...media.process,alt:translatePublicText("Reference illustration of connected brief and image nodes",locale)}} sizes="(max-width:767px) 90vw,580px"/></div><div className={styles.featureText}><h3>{copy.features.items[0].title}</h3><p>{copy.features.items[0].description}</p></div></article>
    <article><div className={`${styles.featureMedia} ${styles.reviewBoard}`}><svg viewBox="0 0 600 500" aria-hidden="true"><path d="M0 50C180 50 130 350 600 350M0 180C280 180 290 20 600 20M0 450C380 450 360 250 600 250"/></svg><div className={styles.reviewPicture}><ModelPicture assetBase="workflows" image={{...media.resultOne,alt:translatePublicText("Reference image to review against a creative brief",locale)}} sizes="(max-width:767px) 50vw,280px"/></div><div className={styles.reviewChecklist}><p>{copy.hero.review}</p>{copy.features.checklist.map((item,i)=><div key={item}><span>{String(i+1).padStart(2,"0")}</span><p>{item}</p></div>)}</div></div><div className={styles.featureText}><h3>{copy.features.items[1].title}</h3><p>{copy.features.items[1].description}</p></div></article>
  </div></section>;
}
export function WorkflowAudiences(){
  const {copy} = usePublicCopy(publicCopySourcesForLocale);
return <section id="audiences" className={styles.audiences}><div className={styles.heading}><h2>{copy.audiences.title}</h2><p>{copy.audiences.description}</p></div><div className={styles.audienceGrid}>{copy.audiences.items.map(item=><article key={item.title}><span aria-hidden="true">{item.icon}</span><h3>{item.title}</h3><p>{item.description}</p></article>)}</div></section>;}
export function WorkflowTools(){
  const {copy} = usePublicCopy(publicCopySourcesForLocale);
return <section id="tools" className={styles.toolBand}><h2>{copy.tools.title}</h2><p>{copy.tools.description}</p><div>{copy.tools.links.map(item=><Link key={item.label} href={item.href}>{item.label}</Link>)}</div></section>;}
export function WorkflowScenarios({onExplore}:{onExplore:(workflow:WorkflowRecord)=>void}){
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="scenarios" className={styles.scenarios}><div className={styles.heading}><h2>{copy.scenariosTitle}</h2><p>{copy.scenariosDescription}</p></div><LandingCarousel label={copy.scenariosTitle} previous={copy.previous} next={copy.next} className={styles.scenarioTrack} bounded>{copy.scenarios.map(scenario=><button type="button" key={scenario.id} data-workflow-scenario className={styles.scenarioCard} onClick={()=>onExplore(scenario)}><div><ModelPicture assetBase="workflows" image={{file:scenario.image,alt:""}} sizes="(max-width:767px) 46vw,210px"/></div><h3>{scenario.title}</h3></button>)}</LandingCarousel></section>;
}

const publicCopySourcesForLocale={copy,media};
