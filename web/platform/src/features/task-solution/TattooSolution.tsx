"use client";

import { useCallback, useState } from "react";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { HomeSearch } from "@/features/public-home/HomeSearch";
import { OverviewConnections } from "@/features/ai-tools-overview/OverviewPanels";
import { aiToolsEn, type OverviewAction } from "@/i18n/public/ai-tools-en";
import { homeEn } from "@/i18n/public/home-en";
import { tattooEn as copy } from "@/i18n/public/tattoo-en";
import Link from "@/i18n/Link";
import { TattooHero } from "./TattooHero";
import { TattooPicture, type TattooImage } from "./TattooPicture";
import { TattooBenefits, TattooDirectory, TattooFlows, TattooHowTo, TattooPlans, TattooQuestions, TattooWorkflow } from "./TattooSections";
import styles from "./TattooSolution.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function TattooSolution() {
  const {copy,aiToolsEn,homeEn} = usePublicCopy(publicCopySourcesForLocale);

  const [search, setSearch] = useState(false);
  const [image, setImage] = useState<TattooImage | null>(null);
  const [planned, setPlanned] = useState<OverviewAction | null>(null);
  const openSearch = useCallback(() => { if (!image && !planned) setSearch(true); }, [image, planned]);
  return <div className={styles.page} data-tattoo-page>
    <a href="#public-main" className={styles.skip}>{copy.skip}</a><Link href="/" className={styles.announcement}>{copy.announcement}<LandingArrow /></Link>
    <LandingHeader navigation={aiToolsEn.navigation} labels={{ ...homeEn.controls, login: homeEn.login, signup: homeEn.signup }} onSearch={openSearch} />
    <main id="public-main"><TattooHero onPreview={setImage} /><div className={styles.sections}><TattooHowTo /><TattooBenefits /><TattooFlows onPreview={setPlanned} /><TattooDirectory /><OverviewConnections /><TattooPlans /><TattooWorkflow /><TattooQuestions /></div></main>
    <LandingFooter {...aiToolsEn.footer} layout="centered" />
    {search ? <HomeSearch copy={homeEn} onClose={() => setSearch(false)} /> : null}
    {image ? <LandingDialog title={copy.hero.preview} closeLabel={copy.close} onClose={() => setImage(null)} wide><div className={styles.previewImage}><TattooPicture image={image} sizes="90vw" /></div><p className={styles.previewText}>{copy.hero.attribution}</p></LandingDialog> : null}
    {planned ? <LandingDialog title={planned.title} closeLabel={copy.close} onClose={() => setPlanned(null)}><span className={styles.comingSoon}>{aiToolsEn.controls.comingSoon}</span><p className={styles.previewText}>{planned.description}</p><p className={styles.previewText}>{aiToolsEn.planned.description}</p><Link href="/ai#tools" className={styles.outline} onClick={() => setPlanned(null)}>{aiToolsEn.controls.explore}<LandingArrow /></Link></LandingDialog> : null}
  </div>;
}

const publicCopySourcesForLocale={aiToolsEn,homeEn,copy};
