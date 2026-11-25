import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import { LandingFooter } from "@/components/public/landing/LandingFooter";
import { LandingHeader } from "@/components/public/landing/LandingHeader";
import { LandingArrow, LandingLink } from "@/components/public/landing/LandingPrimitives";
import palette from "@/features/landing-variants/third/Palette.module.css";
import landing from "@/features/landing-variants/third/PublicHome.module.css";
import { SessionRefresh } from "@/features/session/SessionRefresh/SessionRefresh";
import { WorkspaceLogoutBoundary } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import Link from "@/i18n/Link";
import type { HomeEnglishCopy } from "@/i18n/public/home-en";
import { presentationPageCopy as labels } from "@/i18n/presentation-studio";
import type { PresentationAccess } from "./access.server";
import { PresentationStudio } from "./PresentationStudio";
import { PresentationBrief } from "./PresentationBrief";
import type { DeckRun } from "./deck-run";
import styles from "./Studio.module.css";

export function PresentationsPage({ copy, access, showLocalExample, localApiUnavailable, run }: {
  copy: HomeEnglishCopy;
  access: PresentationAccess;
  showLocalExample: boolean;
  localApiUnavailable: boolean;
  run?: DeckRun;
}) {
  const guest = showLocalExample ? <PresentationStudio local localApiUnavailable={localApiUnavailable} /> :
    <section className={styles.signIn}><h2>{labels.create}</h2><p>{labels.guestIntro}</p><LandingLink href="/login">{labels.signIn}</LandingLink></section>;
  return <div className={`${landing.page} ${palette.root} ${styles.page}`}>
    <a href="#public-main" className={landing.skip}>{copy.skip}</a>
    <LandingHeader variant="editorial" navigation={[]} labels={{ ...copy.controls, login: copy.login, signup: copy.signup }}
      siteAction={access.kind === "authenticated" ? { label: labels.account, href: "/app" } : undefined} />
    <main id="public-main"><PageContainer size="wide" className={styles.container}>
      <Link href="/#workflow" className={styles.home}><LandingArrow direction="left" /> {labels.home}</Link>
      <header className={styles.intro}><h1>{labels.title}</h1><p>{labels.intro}</p></header>
      {access.kind === "unavailable" && <p className={styles.error} role="alert">{labels.accountUnavailable}</p>}
      {access.kind === "authenticated" ? <WorkspaceLogoutBoundary key={access.accountId} guest={guest}>
        <PresentationStudio key={run && [run.conversation, run.job].join(":")} accountId={access.accountId} local={showLocalExample} initialRun={run} />
        <details className={styles.separateBrief}><summary>{labels.separateBrief}</summary><PresentationBrief /></details>
      </WorkspaceLogoutBoundary> : access.kind === "refresh_required" ? <SessionRefresh /> : guest}
    </PageContainer></main>
    <LandingFooter {...copy.footer} />
  </div>;
}
