"use client";

import { useCallback, useState } from "react";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingPlanCards } from "@/components/public/landing/LandingPlanCards";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { ModelPicture, ModelVideo } from "@/components/public/generator-landing/ModelMedia";
import { HomeSearch } from "@/features/public-home/HomeSearch";
import { homeEn } from "@/i18n/public/home-en";
import { aiToolsEn } from "@/i18n/public/ai-tools-en";
import { marketingEn as copy } from "@/i18n/public/marketing-en";
import { marketingReferenceEn as media } from "@/i18n/public/marketing-reference-en";
import Link from "@/i18n/Link";
import { MarketingExamples, MarketingGuides, MarketingNeeds, MarketingTools, type MarketingPreview } from "./MarketingPanels";
import styles from "./MarketingAudience.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function MarketingAudience() {
  const {copy,aiToolsEn,homeEn,media} = usePublicCopy(publicCopySourcesForLocale);

  const [search,setSearch]=useState(false),[paused,setPaused]=useState(false),[preview,setPreview]=useState<MarketingPreview|null>(null);
  const openSearch=useCallback(()=>{if(!preview)setSearch(true);},[preview]);
  return <div className={styles.page} data-marketing-page><a className={styles.skip} href="#public-main">{copy.skip}</a><Link href="/" className={styles.announcement}>{copy.announcement}<LandingArrow/></Link><LandingHeader navigation={aiToolsEn.navigation} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={openSearch}/>
    <main id="public-main"><section id="top" className={styles.hero} aria-labelledby="marketing-title" data-paused={paused||search||!!preview||undefined}><div className={styles.heroMedia} aria-hidden="true"><ModelVideo clip={media.hero} assetBase="marketing-audience" paused={paused||search||!!preview}/></div><div className={styles.heroShade}/><div className={styles.heroInner}><h1 id="marketing-title">{copy.hero.title}</h1><p>{copy.hero.description}</p></div><div className={styles.modelStrip}><p>{copy.hero.modelsLabel}</p><div className={styles.modelWindow}><div className={styles.modelRail}><div className={styles.modelTrack}>{copy.hero.models.map(model=><Link key={model.id} href={`/app?model=${encodeURIComponent(model.id)}`}>{model.label}</Link>)}</div><div className={styles.modelClone} aria-hidden="true">{copy.hero.models.map(model=><span key={model.id}>{model.label}</span>)}</div></div></div></div><button type="button" className={styles.motion} aria-label={paused?homeEn.controls.play:homeEn.controls.pause} aria-pressed={paused} onClick={()=>setPaused(value=>!value)}><svg aria-hidden="true" viewBox="0 0 20 20" fill="currentColor">{paused?<path d="m6 3 11 7L6 17Z"/>:<><rect x="5" y="4" width="3" height="12" rx="1"/><rect x="12" y="4" width="3" height="12" rx="1"/></>}</svg></button><span className={styles.heroCredit}>{copy.source}</span></section>
      <div className={styles.sections}><MarketingExamples onOpen={setPreview}/><MarketingNeeds/><MarketingGuides onOpen={setPreview}/><LandingPlanCards {...copy.plans}/><section id="feedback" className={styles.feedback}><h2>{copy.feedback.title}</h2><p>{copy.feedback.description}</p><a href={copy.feedback.href}>{copy.feedback.action}<LandingArrow/></a></section><MarketingTools onOpen={setPreview}/></div>
    </main><LandingFooter {...aiToolsEn.footer} layout="centered"/>
    {search?<HomeSearch copy={homeEn} onClose={()=>setSearch(false)}/>:null}
    {preview?<LandingDialog title={preview.title} closeLabel={copy.close} onClose={()=>setPreview(null)} wide><span className={styles.previewLabel}>{preview.planned?copy.comingSoon:copy.preview}</span>{preview.image?<div className={styles.previewImage}><ModelPicture image={{...preview.image,alt:preview.title}} assetBase="marketing-audience" sizes="(max-width:767px) 90vw,760px"/></div>:null}<p className={styles.previewDescription}>{preview.description}</p>{preview.steps.length?<ol className={styles.previewSteps}>{preview.steps.map(step=><li key={step}>{step}</li>)}</ol>:null}{preview.planned?<p className={styles.previewDescription}>{copy.planned}</p>:null}<p className={styles.previewSource}>{copy.source}</p><Link href={preview.href??"/ai#tools"} className={styles.previewAction} onClick={()=>setPreview(null)}>{preview.action??copy.explore}<LandingArrow/></Link></LandingDialog>:null}
  </div>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,copy,media};
