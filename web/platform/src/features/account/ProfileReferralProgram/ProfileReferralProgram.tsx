"use client";

import { useEffect, useState, useSyncExternalStore } from "react";
import { z } from "zod";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import { readReferralCode, rememberReferralCode, subscribeReferralCode } from "@/features/referrals/referral-code-cache";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";

import { ProfileReferralFaq } from "@/features/account/ProfileReferralFaq/ProfileReferralFaq";

import styles from "./ProfileReferralProgram.module.css";

type ProfileReferralProgramProps = {
  accountId: string;
  preview?: boolean;
};

const referralStatistics = z.object({
  visits: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  registered: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  activated: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  rewarded: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  rewards_enabled: z.literal(false),
});
const referralSummary = referralStatistics.extend({ code: z.string().regex(/^[A-Za-z0-9_-]{4,64}$/) });
type Statistics = z.infer<typeof referralStatistics>;

export function ProfileReferralProgram(props: ProfileReferralProgramProps) {
  return <AccountReferralProgram key={`${props.accountId}:${Boolean(props.preview)}`} {...props} />;
}

function AccountReferralProgram({ accountId, preview = false }: ProfileReferralProgramProps) {
  const t = useDictionary();
  const locale = useLocale();
  const code = useSyncExternalStore(subscribeReferralCode, () => preview ? null : readReferralCode(accountId), () => null);
  const revision = browserSessionSnapshot().revision;
  const [load, setLoad] = useState<{ revision: number; attempt: number; data: Statistics | null; error: boolean } | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [copyStatus, setCopyStatus] = useState<"copied" | "failed" | null>(null);
  useEffect(() => {
    if (preview) return;
    const session = browserSessionSnapshot();
    if (session.accountId !== accountId) return;
    const controller = new AbortController();
    const signal = AbortSignal.any([session.signal, controller.signal]);
    const hasCode = readReferralCode(accountId) !== null;
    void (async () => {
      try {
        const response = await webBrowserFetch(hasCode ? "/web/v1/referrals/stats" : "/web/v1/referrals", { cache: "no-store", signal });
        if (!response.ok) throw new Error("referral unavailable");
        const body: unknown = await response.json();
        if (signal.aborted || browserSessionSnapshot().revision !== session.revision) return;
        if (!hasCode && !rememberReferralCode(accountId, referralSummary.parse(body).code, session)) return;
        const data = referralStatistics.parse(body);
        setLoad({ revision: session.revision, attempt, data, error: false });
      } catch {
        if (!signal.aborted && browserSessionSnapshot().revision === session.revision) {
          setLoad({ revision: session.revision, attempt, data: null, error: true });
        }
      }
    })();
    return () => controller.abort();
  }, [accountId, attempt, preview]);
  const currentLoad = !preview && load?.revision === revision && load.attempt === attempt ? load : null;
  const data = currentLoad?.data ?? null;
  const error = currentLoad?.error ?? false;
  const inviteURL = code && typeof window !== "undefined" ? `${window.location.origin}/${locale}/invite/${code}` : "";
  const values = data ? [data.visits, data.registered, data.activated, data.rewarded] : [];
  const retry = () => { setCopyStatus(null); setAttempt(value => value + 1); };
  const copy = async () => {
    try { await navigator.clipboard.writeText(inviteURL); setCopyStatus("copied"); }
    catch { setCopyStatus("failed"); }
  };
  return (
    <div className={styles.program}>
      <section aria-labelledby="profile-referral-launch-title" className={styles.launchCard}>
        <h2 id="profile-referral-launch-title">{t.profile.referralLaunchTitle}</h2>
        <p>{t.profile.referralLaunchDescription}</p>
        {preview ? <StateNotice>{t.profile.referralStatisticsUnavailable}</StateNotice> : code ? <div className={styles.invitation}>
            <label htmlFor="profile-referral-url">{t.profile.referralLinkLabel}</label>
            <input id="profile-referral-url" value={inviteURL} readOnly onFocus={event => event.currentTarget.select()} />
            <Button variant="outline" onClick={() => void copy()}>{t.profile.referralCopy}</Button>
            {copyStatus ? <StateNotice kind={copyStatus === "copied" ? "success" : "error"}>
              {copyStatus === "copied" ? t.profile.referralCopied : t.profile.referralCopyFailure}
            </StateNotice> : null}
          </div> : error ?
          <StateNotice kind="error" action={{ label: t.profile.referralRetry, onClick: retry }}>{t.profile.referralLoadFailure}</StateNotice> : !data ?
          <StateNotice kind="loading">{t.profile.referralLoading}</StateNotice> : null}
      </section>

      <section aria-labelledby="profile-referral-steps-title" className={styles.section}>
        <h2 id="profile-referral-steps-title">{t.profile.referralStepsTitle}</h2>
        <ol className={styles.steps}>
          {t.profile.referralSteps.map((step, index) => (
            <li key={step.title}>
              <span aria-hidden="true">{index + 1}</span>
              <strong>{step.title}</strong>
              <p>{step.description}</p>
            </li>
          ))}
        </ol>
      </section>

      <section aria-labelledby="profile-referral-statistics-title" className={styles.section}>
        <h2 id="profile-referral-statistics-title">{t.profile.referralStatisticsTitle}</h2>
        <p className={styles.unavailable}>{t.profile.referralRulesPending}</p>
        {!preview && code && !data ? <StateNotice kind={error ? "error" : "loading"}
          action={error ? { label: t.profile.referralRetry, onClick: retry } : undefined}>
          {error ? t.profile.referralStatisticsFailure : t.profile.referralStatisticsLoading}
        </StateNotice> : null}
        {!preview && !error && data ? <div className={styles.statistics}>
          {t.profile.referralStatisticsCards.map((label, index) => (
            <article className={styles.statistic} key={label}>
              <h3>{label}</h3>
              <p>{values[index].toLocaleString(locale)}</p>
            </article>
          ))}
        </div> : null}
      </section>

      <ProfileReferralFaq />
    </div>
  );
}
