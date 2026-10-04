import { cookies } from "next/headers";

import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import { safeReturnPath } from "@/lib/auth/return-path";
import { loadAuthMethods } from "@/lib/auth/methods.server";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";

import styles from "./page.module.css";

export default async function LoginPage({ searchParams }: { searchParams?: Promise<Record<string, string | string[] | undefined>> } = {}) {
  const returnTo = safeReturnPath((await cookies()).get("__Host-nh-return-to")?.value ?? "");

  return (
    <main className={styles.page}>
      <LoginForm {...(returnTo ? { returnTo } : {})} methods={await loadAuthMethods()} preview={isLocalWorkspacePreviewEnabled()} oauthFailed={(await searchParams)?.oauth === "failed"} />
    </main>
  );
}
