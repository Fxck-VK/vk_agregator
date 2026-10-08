"use client";

import { useEffect, useLayoutEffect, useState, type ReactNode } from "react";
import { acceptPendingInvitation } from "@/features/referrals/attribution";
import { useRouter } from "@/i18n/navigation";
import { accountChangedEvent, devAccessRequiredEvent, navigateToDevAccess, sessionRequiredEvent } from "@/lib/web-api/browser-session";
import { beginBrowserSession, endBrowserSession } from "@/lib/web-api/browser-session-state";
import { clearConversationCache } from "@/features/conversations/conversation-list-cache";

export function WorkspaceSessionHealth({ accountId, verification, children, guest }: { accountId: string; verification?: object; children: ReactNode; guest: ReactNode }) {
  const router = useRouter();
  const [state, setState] = useState({ verification, denied: false });
  if (state.verification !== verification) setState({ verification, denied: false });
  useEffect(() => {
    const controller = new AbortController();
    let retry: ReturnType<typeof setTimeout> | undefined;
    let tries = 0;
    const accept = async () => {
      const accepted = await acceptPendingInvitation(controller.signal);
      if (!accepted && !controller.signal.aborted && ++tries < 3) retry = setTimeout(() => void accept(), 3_000);
    };
    void accept();
    return () => { controller.abort(); clearTimeout(retry); };
  }, [accountId, verification]);
  useLayoutEffect(() => {
    beginBrowserSession(accountId);
    let navigating = false;
    const requireSession = (event: Event) => {
      if (navigating) return;
      navigating = true;
      if (event.type === devAccessRequiredEvent) {
        navigateToDevAccess();
        return;
      }
      endBrowserSession(); clearConversationCache(); setState({ verification, denied: true });
      if (event.type === accountChangedEvent) router.refresh(); else router.replace("/login");
    };
    let channel: BroadcastChannel | undefined;
    try {
      if (typeof BroadcastChannel !== "undefined") {
        channel = new BroadcastChannel("neirohub.workspace-session");
        channel.onmessage = (event: MessageEvent<unknown>) => {
          if (typeof event.data === "object" && event.data !== null && "type" in event.data && event.data.type === "account-changed") requireSession(new Event(accountChangedEvent));
        };
      }
    } catch { /* Optional notification; response account headers are authoritative. */ }
    for (const name of [sessionRequiredEvent, accountChangedEvent, devAccessRequiredEvent]) window.addEventListener(name, requireSession);
    return () => {
      channel?.close();
      for (const name of [sessionRequiredEvent, accountChangedEvent, devAccessRequiredEvent]) window.removeEventListener(name, requireSession);
    };
  }, [accountId, router, verification]);
  return state.verification === verification && state.denied ? guest : children;
}
