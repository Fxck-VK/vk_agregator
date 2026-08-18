import type { Metadata } from "next";
import type { ReactNode } from "react";

import { GuestWorkspaceFrame, WorkspaceFrame } from "@/components/layout/WorkspaceFrame/WorkspaceFrame";
import { AccountControl } from "@/features/account/AccountControl/AccountControl";
import { SessionRefresh } from "@/features/session/SessionRefresh/SessionRefresh";
import { loadWorkspaceSession } from "@/features/session/session-data";
import { WorkspaceHome } from "@/features/workspace/WorkspaceHome/WorkspaceHome";
import { ru } from "@/i18n/ru";

import styles from "./layout.module.css";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export const metadata: Metadata = {
  title: ru.workspace.title,
  robots: {
    index: false,
    follow: false,
  },
};

export default async function WorkspaceLayout({ children }: Readonly<{ children: ReactNode }>) {
  const session = await loadWorkspaceSession();

  if (session.kind === "unauthenticated") {
    return (
      <GuestWorkspaceFrame>
        <WorkspaceHome access="guest" />
      </GuestWorkspaceFrame>
    );
  }

  if (session.kind === "refresh_required") {
    return <SessionRefresh />;
  }

  if (session.kind === "unavailable") {
    return (
      <main className={styles.unavailableState}>
        <div className={styles.unavailableSurface}>
          <p>{ru.workspace.unavailable}</p>
        </div>
      </main>
    );
  }

  return (
    <WorkspaceFrame
      account={<AccountControl profile={session.profile} />}
      accountId={session.profile.account_id}
      balance={session.balance}
      conversations={session.conversations}
      profile={session.profile}
    >
      {children}
    </WorkspaceFrame>
  );
}
