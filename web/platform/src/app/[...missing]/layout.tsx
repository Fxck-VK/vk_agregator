import type { Metadata } from "next";
import type { ReactNode } from "react";

import { GuestWorkspaceFrame, WorkspaceFrame } from "@/components/layout/WorkspaceFrame/WorkspaceFrame";
import { AccountControl } from "@/features/account/AccountControl/AccountControl";
import { loadWorkspaceSession } from "@/features/session/session-data";
import { WorkspaceLogoutBoundary } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import { isServicePath } from "@/i18n/routing";
import { initialModelCatalog } from "@/features/models/model-catalog-server";

export const dynamic = "force-dynamic";
export const revalidate = 0;
export const metadata: Metadata = { robots: { index: false, follow: false } };

type MissingPageLayoutProps = Readonly<{
  children: ReactNode;
  params: Promise<{ missing: string[] }>;
}>;

export default async function MissingPageLayout({ children, params }: MissingPageLayoutProps) {
  const { missing } = await params;
  // Missing assets and technical endpoints must not load or serialize an account.
  if (isServicePath(`/${missing.join("/")}`)) return <main>{children}</main>;

  const session = await loadWorkspaceSession();
  const guest = <GuestWorkspaceFrame>{children}</GuestWorkspaceFrame>;
  // A 404 remains navigable even when the session is absent or cannot be read.
  // Following a real workspace link resumes the existing session/refresh flow.
  if (session.kind !== "authenticated") return guest;

  return (
    <WorkspaceLogoutBoundary guest={guest}>
      <WorkspaceFrame
        deferred={session.deferred}
        initialCatalog={await initialModelCatalog()}
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
