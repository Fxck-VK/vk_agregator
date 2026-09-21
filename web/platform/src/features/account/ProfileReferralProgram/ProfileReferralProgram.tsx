"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import { ProfileReferralFaq } from "@/features/account/ProfileReferralFaq/ProfileReferralFaq";

import styles from "./ProfileReferralProgram.module.css";

export function ProfileReferralProgram() {
  const t = useDictionary();
  return (
    <div className={styles.program}>
      <section aria-labelledby="profile-referral-launch-title" className={styles.launchCard}>
        <h2 id="profile-referral-launch-title">{t.profile.referralLaunchTitle}</h2>
        <p>{t.profile.referralLaunchDescription}</p>
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
        <p className={styles.unavailable}>{t.profile.referralStatisticsUnavailable}</p>
        <div className={styles.statistics}>
          {t.profile.referralStatisticsCards.map((label) => (
            <article className={styles.statistic} key={label}>
              <h3>{label}</h3>
              <p>{t.profile.referralStatisticsUnavailable}</p>
            </article>
          ))}
        </div>
      </section>

      <ProfileReferralFaq />
    </div>
  );
}
