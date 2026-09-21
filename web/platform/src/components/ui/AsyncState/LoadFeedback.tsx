"use client";

import { useEffect, useState } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import { StateNotice } from "./AsyncState";

/** Supplements the owner's stable skeleton/content; never replaces it on refresh. */
export function LoadFeedback({ pending, failed = false, hasData = false, onRetry }: {
  pending: boolean; failed?: boolean; hasData?: boolean; onRetry?: () => void;
}) {
  const t = useDictionary();
  const [offline, setOffline] = useState(false);
  const [slow, setSlow] = useState(false);
  useEffect(() => {
    const update = () => setOffline(!navigator.onLine);
    update();
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);
  useEffect(() => {
    if (!pending) return;
    const timer = window.setTimeout(() => setSlow(true), 3000);
    return () => { clearTimeout(timer); setSlow(false); };
  }, [pending]);
  if (!failed && !(pending && (slow || offline))) return null;
  const message = offline ? t.preloading.offline : failed ? hasData ? t.preloading.refreshFailed : t.preloading.failed : t.preloading.slow;
  return <StateNotice inline kind={failed ? "error" : "loading"} action={failed && onRetry ? { label: t.files.retry, onClick: onRetry } : undefined}>{message}</StateNotice>;
}
