"use client";

import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { webBrowserMutation } from "@/lib/web-api/browser";

import styles from "./SessionRefresh.module.css";

export function SessionRefresh() {
  const router = useRouter();
  const hasAttemptedRefresh = useRef(false);

  useEffect(() => {
    if (hasAttemptedRefresh.current) {
      return;
    }
    hasAttemptedRefresh.current = true;

    const refreshSession = async () => {
      try {
        const response = await webBrowserMutation("/web/v1/auth/refresh", { method: "POST" });
        if (response.status === 200) {
          router.refresh();
          return;
        }
      } catch {
        // The login route provides the neutral recovery path for all refresh failures.
      }

      router.replace("/login");
    };

    void refreshSession();
  }, [router]);

  return (
    <main className={styles.state}>
      <div className={styles.surface}>
        <p>{ru.workspace.refreshPending}</p>
      </div>
    </main>
  );
}
