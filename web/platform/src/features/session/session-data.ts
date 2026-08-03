import "server-only";

import { cookies } from "next/headers";

import {
  parseAccountBalance,
  parseAccountProfile,
  parseConversationList,
  type AccountProfile,
  type ConversationItem,
} from "../../lib/web-api/contracts";
import { webServerFetch } from "../../lib/web-api/server";

export type WorkspaceSession =
  | { kind: "authenticated"; profile: AccountProfile; conversations: ConversationItem[]; balance: number | null }
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

    const [conversationsResponse, balanceResponse] = await Promise.all([
      webServerFetch("/web/v1/conversations?limit=20"),
      webServerFetch("/web/v1/balance"),
    ]);
    if (conversationsResponse.status !== 200) {
      return { kind: "unavailable" };
    }
    const conversations = parseConversationList(await conversationsResponse.json());
    let balance: number | null = null;
    if (balanceResponse.status === 200) {
      try {
        balance = parseAccountBalance(await balanceResponse.json()).balance;
      } catch {
        // The workspace remains usable; the header must not invent a balance if a safe response cannot be parsed.
      }
    }
    return { kind: "authenticated", profile, conversations: conversations.items, balance };
  } catch {
    return { kind: "unavailable" };
  }
}
