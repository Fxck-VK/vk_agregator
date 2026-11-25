"use client";

import { WorkspacePageFrame } from "@/components/layout/WorkspacePageFrame/WorkspacePageFrame";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { presentationPageCopy } from "@/i18n/presentation-studio";
import { useDictionary } from "@/i18n/LocaleProvider";
import { PresentationStudio } from "./PresentationStudio";
import type { DeckRun } from "./deck-run";
import styles from "./Studio.module.css";

export function PresentationWorkspace({ run, preview }: { run?: DeckRun; preview: boolean }) {
  const { profile } = useWorkspaceAccountSnapshot();
  const t = useDictionary();
  return <WorkspacePageFrame>
    <section className={styles.workspacePage} aria-labelledby="workspace-presentations-title">
      <header className={styles.workspaceIntro}>
        <h1 id="workspace-presentations-title">{t.navigation.presentations}</h1>
        <p>{presentationPageCopy.intro}</p>
      </header>
      <div className={[styles.page, styles.workspaceStudio].join(" ")}>
        <PresentationStudio key={run && [run.conversation, run.job].join(":")}
          accountId={profile.account_id} initialRun={run} local={preview} localApiUnavailable={preview} />
      </div>
    </section>
  </WorkspacePageFrame>;
}
