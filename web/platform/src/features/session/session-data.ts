import "server-only";

import { cookies } from "next/headers";

import {
  parseAccountProfile,
  parseConversationList,
  type AccountProfile,
  type ConversationItem,
} from "../../lib/web-api/contracts";
import { webServerFetch } from "../../lib/web-api/server";

export type WorkspaceSession =
  | { kind: "authenticated"; profile: AccountProfile; conversations: ConversationItem[] }
  | { kind: "unauthenticated" }
  | { kind: "refresh_required" }
  | { kind: "unavailable" };

export async function loadWorkspaceSession(): Promise<WorkspaceSession> {
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

    const conversationsResponse = await webServerFetch("/web/v1/conversations?limit=20");
    if (conversationsResponse.status !== 200) {
      return { kind: "unavailable" };
    }
    const conversations = parseConversationList(await conversationsResponse.json());
    return { kind: "authenticated", profile, conversations: conversations.items };
  } catch {
    return { kind: "unavailable" };
  }
}
