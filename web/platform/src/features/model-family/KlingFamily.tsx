"use client";

import { useCallback, useState } from "react";
import { GeneratorSectionNavigation } from "@/components/public/generator-landing/GeneratorSectionNavigation";
import { type LandingNavigationItem, LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { HomeSearch } from "@/features/public-home/HomeSearch";
import { homeEn } from "@/i18n/public/home-en";
import { klingEn as copy } from "@/i18n/public/kling-en";
import { klingReferenceEn as reference } from "@/i18n/public/kling-reference-en";
import Link from "@/i18n/Link";
import { FamilyGallery, FamilyGalleryDialog, FamilyPicture, type FamilyExample } from "./FamilyMedia";
import { FamilyChoose, FamilyComparison, FamilyTimeline, FamilyVersions, FamilyWorkflow, modelHref } from "./FamilySections";
import { FamilyResearch } from "./FamilyResearch";
import { FamilyFinale, FamilyPlans } from "./FamilyFinale";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import styles from "@/components/public/generator-landing/ModelLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



const navigation = (homeEn.navigation as readonly LandingNavigationItem[]).map(item=>({...item,href:item.href?.startsWith("#")&&item.href!=="#plans"?"/"+item.href:item.href,groups:item.groups?.map(group=>({...group,links:group.links.map(link=>({...link,href:link.href.startsWith("#")?"/"+link.href:link.href}))}))}));

export function KlingFamily() {
  const publicCopyLocale=useLocale();

  const {homeEn,navigation,reference,copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"klingEn","reference":"klingReferenceEn"});

  const [search,setSearch]=useState(false),[preview,setPreview]=useState<string|null>(null),[gallery,setGallery]=useState<FamilyExample|null>(null),[paused,setPaused]=useState(false);
  const openSearch=useCallback(()=>{if(!preview&&!gallery)setSearch(true);},[preview,gallery]);
  const stop=paused||search||!!preview||!!gallery;
  return <div className={layout.page} data-model-family-page data-paused={stop||undefined}><a href="#public-main" className={layout.skip}>{homeEn.skip}</a><Link href="/" className={layout.announcement}>{homeEn.announcement}<LandingArrow/></Link><LandingHeader navigation={navigation} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={openSearch}/><main id="public-main"><section className={`${layout.hero} ${styles.familyHero}`} aria-labelledby="family-title"><FamilyPicture image={reference.hero} className={layout.heroBg} sizes="100vw" preload decorative/><div className={layout.heroOverlay}/><div className={layout.heroInner}><nav className={layout.heroCrumbs} aria-label={copy.controls.breadcrumb}><Link href="/" className={layout.heroCrumbLink}>{copy.controls.home}</Link><span aria-hidden="true">/</span><Link href="/app/models" className={layout.heroCrumbLink}>{copy.controls.video}</Link><span aria-hidden="true">/</span><span className={layout.heroCrumbCurrent} aria-current="page">{translatePublicText("Kling",publicCopyLocale)}</span></nav><p className={styles.heroEyebrow}>{copy.hero.eyebrow}</p><h1 id="family-title" className={`${layout.heroH1} ${styles.heroH1}`}>{copy.hero.title}</h1><p className={layout.heroDescription}>{copy.hero.description}</p><p className={styles.heroSubline}>{copy.hero.subline}</p><div className={styles.heroCtas}><Link href={modelHref("video_kling_v3")} className={styles.ctaPrimary}>{copy.hero.action}</Link><a href="#versions" className={styles.ctaSecondary}>{copy.hero.secondary}</a></div></div></section><ul className={styles.trustStrip}>{copy.trust.map(item=><li key={item.figure} className={styles.trustItem}><span className={styles.trustFigure}>{item.figure}</span><span className={styles.trustLabel}>{item.label}</span></li>)}</ul><div className={layout.motionToolbar}><button type="button" aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?copy.controls.resume:copy.controls.pause}<span aria-hidden="true">{paused?"▶":"Ⅱ"}</span></button></div><FamilyGallery items={reference.gallery} onOpen={setGallery} paused={stop}/><div className={layout.grid}><GeneratorSectionNavigation sections={copy.sections} label={copy.controls.navigation}/><div className={layout.main}><FamilyVersions onPreview={setPreview}/><FamilyChoose/><FamilyTimeline onPreview={setPreview}/><FamilyComparison paused={stop}/><FamilyWorkflow onPreview={setPreview}/><FamilyResearch/><FamilyPlans/></div></div><FamilyFinale/></main><LandingFooter {...homeEn.footer}/>{search?<HomeSearch copy={homeEn} onClose={()=>setSearch(false)}/>:null}{gallery?<FamilyGalleryDialog item={gallery} onClose={()=>setGallery(null)}/>:null}{preview?<LandingDialog title={preview} closeLabel={copy.controls.close} onClose={()=>setPreview(null)}><p className={styles.previewNote}>{copy.controls.coming}</p><p className={styles.previewNote}>{copy.controls.preview}</p><Link href="/app/models" className={styles.previewAction}>{copy.controls.explore} →</Link></LandingDialog>:null}</div>;
}

const publicCopySourcesForLocale={homeEn,copy,reference,navigation};
