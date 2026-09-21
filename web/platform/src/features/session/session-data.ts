import "server-only";

import { cookies } from "next/headers";

import {
  parseAccountProfile,
  type AccountProfile,
  type ConversationItem,
} from "../../lib/web-api/contracts";
import { webServerFetch } from "../../lib/web-api/server";
import {
  isLocalWorkspacePreviewEnabled,
  localWorkspacePreviewBalance,
  localWorkspacePreviewConversations,
  localWorkspacePreviewProfile,
} from "./local-workspace-preview";

export type WorkspaceSession =
  | { kind: "authenticated"; profile: AccountProfile; conversations: ConversationItem[]; balance: number | null; deferred?: boolean }
  | { kind: "unauthenticated" }
  | { kind: "refresh_required" }
  | { kind: "unavailable" };

export async function loadWorkspaceSession(): Promise<WorkspaceSession> {
  if (isLocalWorkspacePreviewEnabled()) {
    return {
      kind: "authenticated",
      profile: localWorkspacePreviewProfile,
      balance: localWorkspacePreviewBalance,
      conversations: localWorkspacePreviewConversations,
    };
  }

  try {
    const profileResponse = await webServerFetch("/web/v1/me");
    if (profileResponse.status === 401) {
      const cookieStore = await cookies();
      if (cookieStore.has("nh_refresh")) {
        return { kind: "refresh_required" };
      }
      return { kind: "unauthenticated" };
    }
    if (profileResponse.status !== 200) {
      return { kind: "unavailable" };
    }
    const profile = parseAccountProfile(await profileResponse.json());

    // Only identity gates private content. Sidebar and balance read independently.
    return { kind: "authenticated", profile, conversations: [], balance: null, deferred: true };
  } catch {
    return { kind: "unavailable" };
  }
}
