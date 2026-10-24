import { cookies } from "next/headers";

import { getLegalCatalog } from "@/features/legal/catalog.server";
import { LoginForm } from "@/features/auth/LoginForm/LoginForm";
import { LoginWindow } from "@/features/auth/LoginWindow/LoginWindow";
import { safeReturnPath } from "@/lib/auth/return-path";
import { loadAuthMethods } from "@/lib/auth/methods.server";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";

import styles from "./page.module.css";

export default async function LoginPage({ searchParams }: { searchParams?: Promise<Record<string, string | string[] | undefined>> } = {}) {
  const returnTo = safeReturnPath((await cookies()).get("__Host-nh-return-to")?.value ?? "");
  const preview = isLocalWorkspacePreviewEnabled();
  const methods = await loadAuthMethods();
  const loginMethods = preview ? { ...methods, providers: methods.providers.filter(provider => provider === "yandex" || provider === "vk") } : methods;

  return (
    <main className={styles.page}>
      <LoginWindow><LoginForm entry="choice" legalMetadata={getLegalCatalog().metadata} initialMode={(await searchParams)?.mode === "register" ? "register" : "login"} {...(returnTo ? { returnTo } : {})} methods={loginMethods} preview={preview} oauthFailed={(await searchParams)?.oauth === "failed"} /></LoginWindow>
    </main>
  );
}
