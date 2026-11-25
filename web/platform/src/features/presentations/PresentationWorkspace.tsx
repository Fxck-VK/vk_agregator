"use client";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { presentationHistoryCopy as historyCopy, presentationPageCopy } from "@/i18n/presentation-studio";
import { useDictionary } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { PresentationHistory } from "./PresentationHistory";
import { PresentationStudio } from "./PresentationStudio";
import type { DeckRun } from "./deck-run";
import { presentationWorkspacePath } from "./route";
import styles from "./Studio.module.css";

export function PresentationWorkspace({ run, newPresentation = false, preview }: {
  run?: DeckRun;
  newPresentation?: boolean;
  preview: boolean;
}) {
  const { profile } = useWorkspaceAccountSnapshot();
  const t = useDictionary();
  const studio = newPresentation || Boolean(run);
  return <WorkspacePageFrame>
    <section className={styles.workspacePage} aria-labelledby="workspace-presentations-title">
      <header className={styles.workspaceIntro}>
        {studio && <Link href={presentationWorkspacePath} className={styles.workspaceBack}>
          <span aria-hidden="true">←</span> {historyCopy.title}
        </Link>}
        <div className={styles.workspaceTitleRow}>
          <div>
            <h1 id="workspace-presentations-title">{t.navigation.presentations}</h1>
            {studio && <p>{presentationPageCopy.intro}</p>}
          </div>
          {!studio && <Link href={presentationWorkspacePath + "?new=1"} className={styles.workspaceNew}>
            {historyCopy.newPresentation}
          </Link>}
        </div>
      </header>
      {studio ? <div className={[styles.page, styles.workspaceStudio].join(" ")}>
        <PresentationStudio key={run && [run.conversation, run.job].join(":")}
          accountId={profile.account_id} initialRun={run} local={preview} localApiUnavailable={preview} />
      </div> : <PresentationHistory key={profile.account_id} />}
    </section>
  </WorkspacePageFrame>;
}
