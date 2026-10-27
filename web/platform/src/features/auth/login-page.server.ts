import "server-only";

import { cookies } from "next/headers";
import { getLegalCatalog } from "@/features/legal/catalog.server";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";
import { safeReturnPath } from "@/lib/auth/return-path";
import { loadAuthMethods } from "@/lib/auth/methods.server";

export type LoginSearchParams = Promise<Record<string, string | string[] | undefined>>;

/** All canonical auth steps share server-owned availability, return cookie and legal catalogue. */
export async function loadLoginPageProps(searchParams?: LoginSearchParams) {
  const returnTo = safeReturnPath((await cookies()).get("__Host-nh-return-to")?.value ?? "");
  const preview = isLocalWorkspacePreviewEnabled();
  const methods = await loadAuthMethods();
  const query = await searchParams;
  return {
    entry: "choice" as const,
    legalMetadata: getLegalCatalog().metadata,
    initialMode: query?.mode === "register" ? "register" as const : "login" as const,
    ...(returnTo ? { returnTo } : {}),
    methods: preview ? { ...methods, providers: methods.providers.filter(provider => provider === "yandex" || provider === "vk") } : methods,
    preview,
    oauthFailed: query?.oauth === "failed",
  };
}
