"use client";

import { useState } from "react";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { assetPaths } from "@/assets/asset-paths";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { HomeHero } from "./HomeHero";
import { HomeSuite } from "./HomeSuite";
import { HomeModels, HomePossibilities, HomeTools, HomeWorkflow } from "./HomeExplore";
import { HomeClosing, HomeOriginals, HomePlans, HomeQuestions, HomeTeams } from "./HomeFinale";
import styles from "./PublicHome.module.css";
import palette from "./Palette.module.css";

const hiddenFooterDestinations = new Set([
  "/ai", "/company/about-us", "/events/creative-sessions", "/blog/author/epifera", "#questions", "mailto:support@epifera.com",
]);

export function PublicHome({ copy }: { copy: HomeEnglishCopy }) {
  const [dialog, setDialog] = useState<"intro" | null>(null);
  const openIntro = () => setDialog("intro");
  const footerGroups = copy.footer.groups.map(group => ({ ...group, links: group.links.filter(link => !hiddenFooterDestinations.has(link.href)) }));
  return <div className={`${styles.page} ${palette.root}`}>
    <a href="#public-main" className={styles.skip}>{copy.skip}</a>
    <LandingHeader navigation={[]} labels={{ ...copy.controls, login: copy.login, signup: copy.signup }} />
    <main id="public-main"><HomeHero copy={copy} onIntro={openIntro} />
      <HomeSuite copy={copy} onTour={openIntro} /><HomeModels copy={copy} /><HomeWorkflow copy={copy} /><HomeTools copy={copy} /><HomePossibilities copy={copy} /><HomeOriginals copy={copy} /><HomeTeams copy={copy} /><HomePlans copy={copy} /><HomeQuestions copy={copy} /><HomeClosing copy={copy} />
    </main>
    <LandingFooter {...copy.footer} groups={footerGroups} />
    {dialog === "intro" ? <LandingDialog title={copy.why} closeLabel={copy.controls.close} onClose={() => setDialog(null)} className={palette.root} presentation="media"><div className={styles.introVideo}><MediaVideo src={assetPaths.videos.workspace.howItWorks} poster={assetPaths.images.workspace.howItWorksPoster} aria-label={copy.why} tabIndex={0} controls autoPlay playsInline fit="contain" preload="metadata" /></div></LandingDialog> : null}
  </div>;
}
