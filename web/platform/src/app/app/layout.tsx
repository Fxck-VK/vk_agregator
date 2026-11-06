import type { Metadata } from "next";
import { Suspense, type ReactNode } from "react";

import { GuestWorkspaceFrame, WorkspaceFrame, LoadingWorkspaceFrame, SessionFailureNotice } from "@/components/layout/WorkspaceFrame/WorkspaceFrame";
import { initialModelCatalog } from "@/features/models/model-catalog-server";
import { AccountControl } from "@/features/account/AccountControl/AccountControl";
import { SessionRefresh } from "@/features/session/SessionRefresh/SessionRefresh";
import { WorkspaceLogoutBoundary } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import { loadWorkspaceSession } from "@/features/session/session-data";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";
import { getRequestReleaseState } from "@/features/release/server";
import { WorkspaceHome } from "@/features/workspace/WorkspaceHome/WorkspaceHome";
import { WorkspaceNavigationMetrics } from "@/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics";
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

async function loadWorkspaceSessionWithTiming() {
  const measureIdentity = !isLocalWorkspacePreviewEnabled()
    && (process.env.NODE_ENV === "development" || process.env.WEB_ORIGIN === "https://dev-web.epifera.com");
  const identityStartedAt = measureIdentity ? performance.now() : undefined;
  const session = await loadWorkspaceSession();
  const identityDurationMs = identityStartedAt === undefined ? undefined : performance.now() - identityStartedAt;
  return { session, identityDurationMs };
}

export async function WorkspaceSessionContent({ children }: Readonly<{ children: ReactNode }>) {
  const { session, identityDurationMs } = await loadWorkspaceSessionWithTiming();
  const preview = process.env.NODE_ENV === "development" && isLocalWorkspacePreviewEnabled();

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
      <LoadingWorkspaceFrame><WorkspaceNavigationMetrics identityDurationMs={identityDurationMs} identityOutcome="failure" /><div className={styles.unavailableState}><SessionFailureNotice /></div></LoadingWorkspaceFrame>
    );
  }

  const release = await getRequestReleaseState();
  const catalogue = initialModelCatalog(release.view);
  return (
    <WorkspaceLogoutBoundary
      guest={(
        <GuestWorkspaceFrame>
          <WorkspaceHome access="guest" />
        </GuestWorkspaceFrame>
      )}
    >
      <WorkspaceFrame
        identityDurationMs={identityDurationMs}
        preview={preview}
        initialCatalog={catalogue.source}
        pendingCatalog={catalogue.pending}
        deferred={session.deferred}
        deferConversations={preview || session.deferred}
        account={<AccountControl profile={session.profile} />}
        accountId={session.profile.account_id}
        balance={session.balance}
        conversations={session.conversations}
        profile={session.profile}
      >
        {children}
      </WorkspaceFrame>
    </WorkspaceLogoutBoundary>
  );
}
