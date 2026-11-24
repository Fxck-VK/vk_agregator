import "server-only";

import { cookies } from "next/headers";
import { parseAccountProfile } from "@/lib/web-api/contracts";
import { webServerFetch } from "@/lib/web-api/server";

export type PresentationAccess =
  | { kind: "guest" | "refresh_required" | "unavailable" }
  | { kind: "authenticated"; accountId: string };

export async function presentationApiReachable() {
  try {
    const response = await webServerFetch("/web/v1/presentations/quote", { signal: AbortSignal.timeout(1500), redirect: "error" });
    return response.ok || response.status === 401;
  } catch { return false; }
}

export async function loadPresentationAccess(): Promise<PresentationAccess> {
  const request = await cookies();
  const refresh = request.has("nh_refresh");
  if (!request.has("nh_access") && !refresh) return { kind: "guest" };

  try {
    const result = await webServerFetch("/web/v1/me", { signal: AbortSignal.timeout(3_000), redirect: "error" });
    if (result.status === 401) return { kind: refresh ? "refresh_required" : "guest" };
    if (result.status !== 200) return { kind: "unavailable" };
    const profile = parseAccountProfile(await result.json());
    return { kind: "authenticated", accountId: profile.account_id };
  } catch {
    return { kind: "unavailable" };
  }
}
