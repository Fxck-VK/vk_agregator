"use client";

import { useCallback, useState } from "react";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { LandingArrow, LandingLink } from "@/components/public/landing/LandingPrimitives";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import Link from "@/i18n/Link";
import { HomeHero } from "./HomeHero";
import { HomeSuite } from "./HomeSuite";
import { HomeModels, HomePossibilities, HomeTools, HomeWorkflow } from "./HomeExplore";
import { HomeClosing, HomeOriginals, HomePlans, HomeQuestions, HomeTeams } from "./HomeFinale";
import { HomeSearch } from "./HomeSearch";
import { HomeMediaPrompt } from "./HomeMedia";
import styles from "./PublicHome.module.css";

export function PublicHome({ copy }: { copy: HomeEnglishCopy }) {
  const [paused, setPaused] = useState(false);
  const [dialog, setDialog] = useState<"intro" | "search" | null>(null);
  const openSearch = useCallback(() => setDialog("search"), []);
  const openIntro = () => setDialog("intro");
  return <div className={styles.page} data-paused={paused || undefined}>
    <a href="#public-main" className={styles.skip}>{copy.skip}</a>
    <Link href="#creative-suite" className={styles.announcement}>{copy.announcement}<LandingArrow /></Link>
    <LandingHeader variant="editorial" navigation={copy.navigation} labels={{ ...copy.controls, login: copy.login, signup: copy.signup }} onSearch={openSearch} />
    <main id="public-main"><HomeHero copy={copy} paused={paused} onPause={() => setPaused(!paused)} onIntro={openIntro} />
      <HomeSuite copy={copy} onTour={openIntro} /><HomeModels copy={copy} paused={paused} /><HomeWorkflow copy={copy} /><HomeTools copy={copy} /><HomePossibilities copy={copy} /><HomeOriginals copy={copy} /><HomeTeams copy={copy} /><HomePlans copy={copy} /><HomeQuestions copy={copy} /><HomeClosing copy={copy} />
    </main>
    <LandingFooter {...copy.footer} />
    {dialog === "search" ? <HomeSearch copy={copy} onClose={() => setDialog(null)} /> : null}
    {dialog === "intro" ? <LandingDialog title={copy.why} closeLabel={copy.controls.close} onClose={() => setDialog(null)} wide><HomeMediaPrompt brief="overview" /><p className={styles.introCaption}>{copy.suite.intro}</p><LandingLink href="/app">{copy.start}<LandingArrow /></LandingLink></LandingDialog> : null}
  </div>;
}
