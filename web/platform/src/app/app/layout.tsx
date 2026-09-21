import type { Metadata } from "next";
import { Suspense, type ReactNode } from "react";

import { GuestWorkspaceFrame, WorkspaceFrame, LoadingWorkspaceFrame, SessionFailureNotice } from "@/components/layout/WorkspaceFrame/WorkspaceFrame";
import { initialModelCatalog } from "@/features/models/model-catalog-server";
import { AccountControl } from "@/features/account/AccountControl/AccountControl";
import { SessionRefresh } from "@/features/session/SessionRefresh/SessionRefresh";
import { WorkspaceLogoutBoundary } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import { loadWorkspaceSession } from "@/features/session/session-data";
import { isLocalWorkspacePreviewEnabled, localWorkspacePreviewConversations, localWorkspacePreviewConversationMessages, localWorkspacePreviewImageModels } from "@/features/session/local-workspace-preview";
import { WorkspaceHome } from "@/features/workspace/WorkspaceHome/WorkspaceHome";
import { getRequestDictionary } from "@/i18n/server";

import styles from "./layout.module.css";

export const dynamic = "force-dynamic";
export const revalidate = 0;

export async function generateMetadata(): Promise<Metadata> {
  const t = await getRequestDictionary();
  return {
  title: t.workspace.title,
  robots: {
    index: false,
    follow: false,
  },
  };
}

export default function WorkspaceLayout({ children }: Readonly<{ children: ReactNode }>) {
  return <Suspense fallback={<LoadingWorkspaceFrame />}><WorkspaceSessionContent>{children}</WorkspaceSessionContent></Suspense>;
}

export async function WorkspaceSessionContent({ children }: Readonly<{ children: ReactNode }>) {
  const session = await loadWorkspaceSession();
  let developmentTools: ReactNode = null;
  if (process.env.NODE_ENV === "development" && isLocalWorkspacePreviewEnabled()) {
    const { LocalDevelopmentTools } = await import("@/features/local-development/LocalDevelopmentTools");
    developmentTools = <LocalDevelopmentTools seed={{ conversations: localWorkspacePreviewConversations, messages: localWorkspacePreviewConversationMessages, imageModels: localWorkspacePreviewImageModels.items }} />;
  }

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
      <LoadingWorkspaceFrame><div className={styles.unavailableState}><SessionFailureNotice /></div></LoadingWorkspaceFrame>
    );
  }

  return (
    <WorkspaceLogoutBoundary
      guest={(
        <GuestWorkspaceFrame>
          <WorkspaceHome access="guest" />
        </GuestWorkspaceFrame>
      )}
    >
      <WorkspaceFrame
        initialCatalog={await initialModelCatalog()}
        deferred={session.deferred}
        account={<AccountControl profile={session.profile} />}
        accountId={session.profile.account_id}
        balance={session.balance}
        conversations={session.conversations}
        profile={session.profile}
      >
        {children}
      </WorkspaceFrame>
      {developmentTools}
    </WorkspaceLogoutBoundary>
  );
}
