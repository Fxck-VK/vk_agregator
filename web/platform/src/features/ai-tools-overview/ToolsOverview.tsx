"use client";

import { useCallback, useState } from "react";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { HomeSearch } from "@/features/public-home/HomeSearch";
import { homeEn } from "@/i18n/public/home-en";
import { aiToolsEn as copy, type OverviewAction } from "@/i18n/public/ai-tools-en";
import { aiToolsReferenceEn as reference } from "@/i18n/public/ai-tools-reference-en";
import Link from "@/i18n/Link";
import { OverviewDirectory } from "./OverviewDirectory";
import { OverviewConnections, OverviewModels } from "./OverviewPanels";
import { OverviewClosing, OverviewDocumentation, OverviewPlugins, OverviewShowcase, OverviewTeams } from "./OverviewEditorial";
import { OverviewActionLink, OverviewIcon, OverviewPicture } from "./OverviewPrimitives";
import styles from "./ToolsOverview.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function ToolsOverview() {
  const {copy,homeEn,reference} = usePublicCopy(publicCopySourcesForLocale);

  const [search,setSearch] = useState(false);
  const [preview,setPreview] = useState<OverviewAction | null>(null);
  const openSearch = useCallback(() => { if (!preview) setSearch(true); },[preview]);
  return <div className={styles.page} data-ai-tools-overview>
    <a href="#public-main" className={styles.skip}>{copy.skip}</a><Link href="/" className={styles.announcement}>{copy.announcement}<LandingArrow /></Link>
    <LandingHeader navigation={copy.navigation} labels={{ ...homeEn.controls, login:homeEn.login, signup:homeEn.signup }} onSearch={openSearch} />
    <main id="public-main">
      <section className={styles.hero} aria-labelledby="tools-title"><OverviewPicture image={reference.hero} className={styles.heroImage} sizes="100vw" preload decorative /><div className={styles.heroShade} />
        <div className={styles.heroInner}><h1 id="tools-title">{copy.hero.title}</h1><p>{copy.hero.description}</p><div className={styles.shortcuts}>{copy.hero.shortcuts.map(item => <OverviewActionLink item={item} key={item.title} className={styles.shortcut} onPreview={setPreview}>
          <span className={styles.shortcutIcon} data-tone={item.tone}><OverviewIcon name={item.icon} /></span><span>{item.title}</span>{"planned" in item && item.planned ? <small>{copy.controls.comingSoon}</small> : null}
        </OverviewActionLink>)}</div></div>
      </section>
      <div className={styles.sections}><OverviewDirectory onPreview={setPreview} /><OverviewPlugins onPreview={setPreview} /><OverviewModels /><OverviewConnections /><OverviewTeams /><OverviewDocumentation onPreview={setPreview} /><OverviewShowcase onPreview={setPreview} /><OverviewClosing /></div>
    </main>
    <LandingFooter {...copy.footer} layout="centered" />
    {search ? <HomeSearch copy={homeEn} onClose={() => setSearch(false)} /> : null}
    {preview ? <LandingDialog title={preview.title} closeLabel={copy.controls.close} onClose={() => setPreview(null)}><span className={styles.comingSoon}>{copy.controls.comingSoon}</span><p className={styles.previewDescription}>{preview.description}</p><p className={styles.previewDescription}>{copy.planned.description}</p><Link href="/ai#tools" className={styles.previewAction} onClick={() => setPreview(null)}>{copy.controls.explore}<LandingArrow /></Link></LandingDialog> : null}
  </div>;
}

const publicCopySourcesForLocale={homeEn,copy,reference};
