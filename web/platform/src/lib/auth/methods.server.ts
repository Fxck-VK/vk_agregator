import "server-only";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";
import { webServerFetch } from "@/lib/web-api/server";
import { authMethodsSchema, defaultAuthMethods, previewAuthMethods } from "./methods";

export async function loadAuthMethods() {
  if (isLocalWorkspacePreviewEnabled()) return previewAuthMethods;
  try {
    const response = await webServerFetch("/web/v1/auth/methods", { signal: AbortSignal.timeout(2500) });
    return response.ok ? authMethodsSchema.parse(await response.json()) : defaultAuthMethods;
  } catch { return defaultAuthMethods; }
}
