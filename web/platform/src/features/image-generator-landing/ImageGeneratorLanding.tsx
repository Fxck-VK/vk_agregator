"use client";

import { useCallback, useState } from "react";
import { GeneratorSectionNavigation } from "@/components/public/generator-landing/GeneratorSectionNavigation";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { HomeSearch } from "@/features/public-home/HomeSearch";
import { stageImagePrompt } from "@/features/image-generation/image-prompt-draft";
import { homeEn } from "@/i18n/public/home-en";
import { imageGeneratorEn as copy } from "@/i18n/public/image-generator-en";
import { imageGeneratorReferenceEn as reference } from "@/i18n/public/image-generator-reference-en";
import Link from "@/i18n/Link";
import { useRouter } from "@/i18n/navigation";
import { ImageGeneratorCreate } from "./ImageGeneratorCreate";
import { ImageGeneratorGallery, ImageGeneratorGalleryDialog } from "./ImageGeneratorGallery";
import { GeneratorPicture, type ReferenceImage } from "./ImageGeneratorMedia";
import { ImageGeneratorFinale, ImageGeneratorModels, ImageGeneratorPlans, ImageGeneratorTrust, ImageGeneratorUseCases, ImageGeneratorWorkflow } from "./ImageGeneratorSections";
import { ImageGeneratorStudy } from "./ImageGeneratorStudy";
import styles from "@/components/public/generator-landing/GeneratorLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function ImageGeneratorLanding() {
  const {copy,homeEn,reference} = usePublicCopy(publicCopySourcesForLocale);

  const [prompt, setPrompt] = useState(""); const [search, setSearch] = useState(false); const [gallery, setGallery] = useState<ReferenceImage | null>(null); const [paused, setPaused] = useState(false);
  const router = useRouter(); const openSearch = useCallback(() => { if (gallery === null) setSearch(true); }, [gallery]);
  return <div className={styles.page} data-image-generator-page data-paused={paused || gallery !== null || search || undefined}>
    <a className={styles.skip} href="#public-main">{copy.skip}</a><Link href="/" className={styles.announcement}>{copy.announcement}<LandingArrow /></Link><LandingHeader navigation={copy.navigation} labels={{ ...homeEn.controls, login: homeEn.login, signup: homeEn.signup }} onSearch={openSearch} />
    <main id="public-main"><section id="top" className={styles.hero} aria-labelledby="generator-title"><GeneratorPicture image={reference.hero} className={styles.heroBg} sizes="100vw" preload decorative /><div className={styles.heroOverlay} /><div className={styles.heroInner}>
      <nav className={styles.heroCrumbs} aria-label={copy.controls.breadcrumb}><Link href="/" className={styles.heroCrumbLink}>{copy.controls.home}</Link><span aria-hidden="true">/</span><Link href="/app/models" className={styles.heroCrumbLink}>{copy.controls.tools}</Link><span aria-hidden="true">/</span><span className={styles.heroCrumbCurrent}>{copy.controls.generator}</span></nav>
      <h1 id="generator-title" className={styles.heroH1}>{copy.hero.title}</h1><p className={styles.heroDescription}>{copy.hero.description}</p><div className={styles.heroPromptWrap}><form className={styles.heroPrompt} onSubmit={event => { event.preventDefault(); stageImagePrompt(prompt.trim()); router.push("/app/image"); }}><label htmlFor="image-landing-prompt" className={styles.srOnly}>{copy.hero.placeholder}</label><textarea id="image-landing-prompt" rows={2} value={prompt} maxLength={12000} onChange={event => setPrompt(event.target.value)} placeholder={copy.hero.placeholder} className={styles.heroTextarea} /><div className={styles.heroPromptRow}><span className={styles.heroPromptSpacer} /><button type="submit" className={styles.heroGenerate}>{copy.hero.generate}</button></div></form><div className={styles.heroMeta}><p>{copy.hero.note}</p><a href="#use-cases">{copy.hero.see}</a></div></div>
    </div></section><div className={styles.motionToolbar}><button type="button" onClick={() => setPaused(!paused)} aria-pressed={paused}>{paused ? copy.controls.play : copy.controls.pause}<span aria-hidden="true">{paused ? "▶" : "Ⅱ"}</span></button></div><ImageGeneratorGallery onOpen={setGallery} paused={paused || gallery!==null} /><div className={styles.grid}><GeneratorSectionNavigation sections={copy.sections} label={copy.controls.onThisPage} /><div className={styles.main}><ImageGeneratorCreate /><ImageGeneratorModels /><ImageGeneratorUseCases onOpen={setGallery} /><ImageGeneratorWorkflow /><ImageGeneratorTrust /><ImageGeneratorStudy /><ImageGeneratorPlans /></div></div><ImageGeneratorFinale /></main><LandingFooter {...homeEn.footer} />
    {search ? <HomeSearch copy={homeEn} onClose={() => setSearch(false)} /> : null}{gallery ? <ImageGeneratorGalleryDialog image={gallery} onClose={() => setGallery(null)} /> : null}
  </div>;
}

const publicCopySourcesForLocale={homeEn,copy,reference};
