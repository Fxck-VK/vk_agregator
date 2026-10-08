"use client";

import { useEffect, useState } from "react";
import { useDictionary } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";
import { Button } from "@/components/ui/Button/Button";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { webBrowserFetch } from "@/lib/web-api/browser";
import styles from "../auth/LoginForm/LoginForm.module.css";

// Strict-mode remounts share only an in-flight capture, never account data.
const captures = new Map<string, Promise<void>>();
function capture(code: string): Promise<void> {
  const existing = captures.get(code);
  if (existing) return existing;
  const pending = (async () => {
    const response = await webBrowserFetch("/web/v1/referrals/visit", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code }), signal: AbortSignal.timeout(10_000),
    });
    if (!response.ok) throw new Error("invitation unavailable");
  })();
  captures.set(code, pending);
  void pending.finally(() => captures.delete(code)).catch(() => undefined);
  return pending;
}

export function InviteEntry({ code }: { code: string }) {
  const t = useDictionary();
  const router = useRouter();
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    void capture(code).then(() => { if (active) setState("ready"); }, () => { if (active) setState("error"); });
    return () => { active = false; };
  }, [attempt, code]);
  return <section className={styles.form}>
    <h1>{t.profile.inviteTitle}</h1>
    <StateNotice kind={state === "loading" ? "loading" : state === "error" ? "error" : "success"}
      action={state === "error" ? { label: t.profile.referralRetry, onClick: () => { setState("loading"); setAttempt(value => value + 1); } } : undefined}>
      {state === "loading" ? t.profile.inviteLoading : state === "ready" ? t.profile.inviteReady : t.profile.inviteFailure}
    </StateNotice>
    <p>{t.profile.referralRulesPending}</p>
    <Button disabled={state === "loading"} onClick={() => router.replace("/login")}>
      {state === "error" ? t.profile.inviteWithoutReferral : t.profile.inviteContinue}
    </Button>
  </section>;
}
