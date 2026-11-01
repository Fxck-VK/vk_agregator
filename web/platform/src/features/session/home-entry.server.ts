import "server-only";

import { cookies } from "next/headers";
import { parseAccountProfile } from "@/lib/web-api/contracts";
import { webServerFetch } from "@/lib/web-api/server";
import { isLocalWorkspacePreviewEnabled } from "./local-workspace-preview";

type HomeEntrySession = "guest" | "authenticated" | "refresh_required";

/** Cookies only trigger a bounded identity lookup; they never prove a login. */
export async function loadHomeEntrySession(): Promise<HomeEntrySession> {
  // The sample workspace account must not hide the landing during local design.
  if (isLocalWorkspacePreviewEnabled()) return "guest";
  const cookieStore = await cookies();
  const access = cookieStore.get("nh_access")?.value;
  const refresh = cookieStore.get("nh_refresh")?.value;
  if (!access && !refresh) return "guest";
  try {
    const response = await webServerFetch("/web/v1/me", { signal: AbortSignal.timeout(3_000) });
    if (response.status === 200) {
      parseAccountProfile(await response.json());
      return "authenticated";
    }
    if (response.status === 401 && refresh) return "refresh_required";
  } catch {
    // Public content stays accessible if identity cannot be established.
  }
  return "guest";
}
