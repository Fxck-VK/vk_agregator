import "server-only";
import { headers } from "next/headers";
import { getDevAccessConfig, hasDevAccess } from "@/lib/dev-access/session";
import { releaseViewCookie, type ReleaseState } from "./policy";
export function releaseStateForRequest(requestHeaders: Headers): ReleaseState {
  const localSimulation = process.env.NODE_ENV === "development" && process.env.NEIROHUB_LOCAL_WORKSPACE_PREVIEW === "1";
  const config = getDevAccessConfig();
  const canSwitch = localSimulation || !!config && hasDevAccess(config, requestHeaders.get("cookie"));
  if (!canSwitch) return { view: "prod", canSwitch: false, localSimulation: false };
  const values = (requestHeaders.get("cookie") ?? "").split(";").map(p => p.trim()).filter(p => p.startsWith(releaseViewCookie + "="));
  const view = values.length === 0 || values.length === 1 && values[0] === `${releaseViewCookie}=dev` ? "dev" : "prod";
  return { view, canSwitch: true, localSimulation };
}
export async function getRequestReleaseState(): Promise<ReleaseState> {
  try { return releaseStateForRequest(await headers()); }
  catch { return { view: "prod", canSwitch: false, localSimulation: false }; }
}
