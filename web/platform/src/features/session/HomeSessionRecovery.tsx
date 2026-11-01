"use client";

import { useEffect } from "react";
import { useRouter } from "@/i18n/navigation";
import { refreshBrowserSession } from "@/lib/web-api/browser-session";

/** Keep the guest landing visible while the existing session layer restores cookies. */
export function HomeSessionRecovery() {
  const router = useRouter();
  useEffect(() => {
    const controller = new AbortController();
    void refreshBrowserSession(controller.signal).then(response => {
      if (!controller.signal.aborted && response.status === 200 && response.headers.get("X-NeiroHub-Dev-Access") !== "required") {
        // The server re-verifies /me before making a localized redirect.
        router.refresh();
      }
    }).catch(() => { /* Failed recovery leaves the public landing available. */ });
    return () => controller.abort();
  }, [router]);
  return null;
}
