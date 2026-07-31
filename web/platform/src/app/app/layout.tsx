import type { Metadata } from "next";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";

import { AppShell } from "@/components/layout/AppShell/AppShell";
import { Sidebar } from "@/components/layout/Sidebar/Sidebar";
import { AccountControl } from "@/features/account/AccountControl/AccountControl";
import { SidebarConversations } from "@/features/conversations/SidebarConversations/SidebarConversations";
import { SessionRefresh } from "@/features/session/SessionRefresh/SessionRefresh";
import { loadWorkspaceSession } from "@/features/session/session-data";
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
    redirect("/login");
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
    <AppShell
      sidebar={
        <Sidebar
          account={<AccountControl profile={session.profile} />}
          conversations={<SidebarConversations conversations={session.conversations} />}
        />
      }
    >
      {children}
    </AppShell>
  );
}
