"use client";

import { useCallback, useState } from "react";
import { GeneratorSectionNavigation } from "@/components/public/generator-landing/GeneratorSectionNavigation";
import { ModelGallery, ModelGalleryDialog, ModelPicture, type ModelExample } from "@/components/public/generator-landing/ModelMedia";
import { LandingHeader, type LandingNavigationItem } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { HomeSearch } from "@/features/public-home/HomeSearch";
import { homeEn } from "@/i18n/public/home-en";
import { klingEn } from "@/i18n/public/kling-en";
import { kling30En as copy } from "@/i18n/public/kling30-en";
import { kling30ReferenceEn as reference } from "@/i18n/public/kling30-reference-en";
import Link from "@/i18n/Link";
import { Kling30Details, Kling30Finale, Kling30Research, Kling30Scenarios } from "./Kling30Sections";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import model from "@/components/public/generator-landing/ModelLanding.module.css";
import styles from "./ModelVersion.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



const navigation=(homeEn.navigation as readonly LandingNavigationItem[]).map(item=>({...item,href:item.href?.startsWith("#")?(item.href==="#plans"?"/ai/kling#plans":"/"+item.href):item.href,groups:item.groups?.map(group=>({...group,links:group.links.map(link=>({...link,href:link.href.startsWith("#")?"/"+link.href:link.href}))}))}));
export function Kling30Landing(){
  const publicCopyLocale=useLocale();

  const {homeEn,navigation,reference,copy,klingEn} = usePublicCopy(publicCopySourcesForLocale,{"klingEn":"klingEn","copy":"kling30En","reference":"kling30ReferenceEn"});

  const [search,setSearch]=useState(false),[gallery,setGallery]=useState<ModelExample|null>(null),[paused,setPaused]=useState(false);
  const openSearch=useCallback(()=>{if(!gallery)setSearch(true);},[gallery]);
  const stop=paused||search||!!gallery;
  return <div className={layout.page} data-model-version-page data-paused={stop||undefined}><a href="#public-main" className={layout.skip}>{homeEn.skip}</a><Link href="/" className={layout.announcement}>{homeEn.announcement}<LandingArrow/></Link><LandingHeader navigation={navigation} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={openSearch}/><main id="public-main"><section className={`${layout.hero} ${model.familyHero}`} aria-labelledby="version-title"><ModelPicture image={reference.hero} assetBase="model-version" className={layout.heroBg} sizes="100vw" preload decorative/><div className={layout.heroOverlay}/><div className={layout.heroInner}><nav className={layout.heroCrumbs} aria-label={translatePublicText("Breadcrumb",publicCopyLocale)}><Link href="/" className={layout.heroCrumbLink}>{translatePublicText("Home",publicCopyLocale)}</Link><span aria-hidden="true">/</span><Link href="/app/models" className={layout.heroCrumbLink}>{translatePublicText("AI video models",publicCopyLocale)}</Link><span aria-hidden="true">/</span><Link href="/ai/kling" className={layout.heroCrumbLink}>{translatePublicText("Kling",publicCopyLocale)}</Link><span aria-hidden="true">/</span><span className={layout.heroCrumbCurrent} aria-current="page">{translatePublicText("Kling 3.0",publicCopyLocale)}</span></nav><div className={styles.heroMeta}><span className={`${model.status} ${model.statusLatest}`}>{copy.hero.status}</span><p className={styles.heroEyebrow}>{copy.hero.eyebrow}</p></div><h1 id="version-title" className={`${layout.heroH1} ${model.heroH1}`}>{copy.hero.title}</h1><p className={layout.heroDescription}>{copy.hero.description}</p><div className={model.heroCtas}><Link href={`/app?model=${copy.modelId}`} className={model.ctaPrimary}>{copy.hero.action}</Link><a href="#specs" className={model.ctaSecondary}>{copy.hero.secondary}</a></div></div></section><ul className={model.trustStrip}>{klingEn.trust.map(item=><li key={item.figure} className={model.trustItem}><span className={model.trustFigure}>{item.figure}</span><span className={model.trustLabel}>{item.label}</span></li>)}</ul><div className={layout.motionToolbar}><button type="button" aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?klingEn.controls.resume:klingEn.controls.pause}<span aria-hidden="true">{paused?"▶":"Ⅱ"}</span></button></div><ModelGallery items={reference.gallery} assetBase="model-version" title={copy.galleryTitle} paused={stop} onOpen={setGallery}/><div className={layout.grid}><GeneratorSectionNavigation sections={copy.sections} label={klingEn.controls.navigation}/><div className={layout.main}><Kling30Details/><Kling30Scenarios/><Kling30Research/></div></div><Kling30Finale/></main><LandingFooter {...homeEn.footer}/>{search?<HomeSearch copy={homeEn} onClose={()=>setSearch(false)}/>:null}{gallery?<ModelGalleryDialog item={gallery} assetBase="model-version" title={copy.dialogTitle} href={`/app?model=${copy.modelId}`} testId="version-gallery-backdrop" onClose={()=>setGallery(null)}/>:null}</div>;
}

const publicCopySourcesForLocale={homeEn,klingEn,copy,reference,navigation};
