"use client";

import { useEffect, useState } from "react";
import { z } from "zod";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";

import { ProfileReferralFaq } from "@/features/account/ProfileReferralFaq/ProfileReferralFaq";

import styles from "./ProfileReferralProgram.module.css";

type ProfileReferralProgramProps = {
  preview?: boolean;
};

const referralSummary = z.object({
  code: z.string().regex(/^[A-Za-z0-9_-]{4,64}$/),
  visits: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  registered: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  activated: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  rewarded: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER),
  rewards_enabled: z.literal(false),
});
type Summary = z.infer<typeof referralSummary>;

export function ProfileReferralProgram({ preview = false }: ProfileReferralProgramProps = {}) {
  const t = useDictionary();
  const locale = useLocale();
  const [data, setData] = useState<Summary | null>(null);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [copyStatus, setCopyStatus] = useState<"copied" | "failed" | null>(null);
  useEffect(() => {
    if (preview) return;
    const controller = new AbortController();
    void (async () => {
      try {
        const response = await webBrowserFetch("/web/v1/referrals", { cache: "no-store", signal: controller.signal });
        if (!response.ok) throw new Error("referral unavailable");
        const parsed = referralSummary.parse(await response.json());
        if (!controller.signal.aborted) { setData(parsed); setError(false); }
      } catch {
        if (!controller.signal.aborted) { setData(null); setError(true); }
      }
    })();
    return () => controller.abort();
  }, [attempt, preview]);
  const inviteURL = data && typeof window !== "undefined" ? `${window.location.origin}/${locale}/invite/${data.code}` : "";
  const values = data ? [data.visits, data.registered, data.activated, data.rewarded] : [];
  const retry = () => { setData(null); setCopyStatus(null); setError(false); setAttempt(value => value + 1); };
  const copy = async () => {
    try { await navigator.clipboard.writeText(inviteURL); setCopyStatus("copied"); }
    catch { setCopyStatus("failed"); }
  };
  return (
    <div className={styles.program}>
      <section aria-labelledby="profile-referral-launch-title" className={styles.launchCard}>
        <h2 id="profile-referral-launch-title">{t.profile.referralLaunchTitle}</h2>
        <p>{t.profile.referralLaunchDescription}</p>
        {preview ? <StateNotice>{t.profile.referralStatisticsUnavailable}</StateNotice> : error ?
          <StateNotice kind="error" action={{ label: t.profile.referralRetry, onClick: retry }}>{t.profile.referralLoadFailure}</StateNotice> : !data ?
          <StateNotice kind="loading">{t.profile.referralLoading}</StateNotice> : <div className={styles.invitation}>
            <label htmlFor="profile-referral-url">{t.profile.referralLinkLabel}</label>
            <input id="profile-referral-url" value={inviteURL} readOnly onFocus={event => event.currentTarget.select()} />
            <Button variant="outline" onClick={() => void copy()}>{t.profile.referralCopy}</Button>
            {copyStatus ? <StateNotice kind={copyStatus === "copied" ? "success" : "error"}>
              {copyStatus === "copied" ? t.profile.referralCopied : t.profile.referralCopyFailure}
            </StateNotice> : null}
          </div>}
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
