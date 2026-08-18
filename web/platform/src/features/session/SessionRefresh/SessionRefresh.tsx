"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";

import { webBrowserMutation } from "@/lib/web-api/browser";

import { SessionRestorationShell } from "../SessionRestorationShell/SessionRestorationShell";

const progressDelayMs = 150;
const refreshTimeoutMs = 8_000;
const invalidRefreshStatuses = new Set([400, 401, 403]);

type RefreshPhase = "pending" | "slow" | "retryable_error";

export function SessionRefresh() {
  const router = useRouter();
  const hasAttemptedRefresh = useRef(false);
  const attemptIdRef = useRef(0);
  const activeControllerRef = useRef<AbortController | null>(null);
  const [phase, setPhase] = useState<RefreshPhase>("pending");

  const refreshSession = useCallback(() => {
    const attemptId = attemptIdRef.current + 1;
    attemptIdRef.current = attemptId;

    activeControllerRef.current?.abort();
    const controller = new AbortController();
    activeControllerRef.current = controller;
    setPhase("pending");

    const progressTimer = window.setTimeout(() => {
      if (attemptIdRef.current === attemptId) {
        setPhase("slow");
      }
    }, progressDelayMs);
    const timeoutTimer = window.setTimeout(() => {
      controller.abort();
    }, refreshTimeoutMs);

    const runRefresh = async () => {
      try {
        const response = await webBrowserMutation("/web/v1/auth/refresh", {
          method: "POST",
          signal: controller.signal,
        });

        if (attemptIdRef.current !== attemptId) {
          return;
        }

        if (response.status === 200) {
          router.refresh();
          return;
        }

        if (invalidRefreshStatuses.has(response.status)) {
          router.replace("/login");
          return;
        }

        setPhase("retryable_error");
      } catch {
        if (attemptIdRef.current === attemptId) {
          setPhase("retryable_error");
        }
      } finally {
        window.clearTimeout(progressTimer);
        window.clearTimeout(timeoutTimer);
        if (attemptIdRef.current === attemptId) {
          activeControllerRef.current = null;
        }
      }
    };

    void runRefresh();
  }, [router]);

  useEffect(() => {
    if (hasAttemptedRefresh.current) {
      return;
    }
    hasAttemptedRefresh.current = true;

    refreshSession();
  }, [refreshSession]);

  return (
    <SessionRestorationShell
      isProgressVisible={phase === "slow"}
      isRetryableError={phase === "retryable_error"}
      onRetry={refreshSession}
    />
  );
}
