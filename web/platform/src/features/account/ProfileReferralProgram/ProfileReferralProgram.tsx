import { ProfileReferralFaq } from "@/features/account/ProfileReferralFaq/ProfileReferralFaq";
import { ru } from "@/i18n/ru";

import styles from "./ProfileReferralProgram.module.css";

export function ProfileReferralProgram() {
  return (
    <div className={styles.program}>
      <section aria-labelledby="profile-referral-launch-title" className={styles.launchCard}>
        <h2 id="profile-referral-launch-title">{ru.profile.referralLaunchTitle}</h2>
        <p>{ru.profile.referralLaunchDescription}</p>
      </section>

      <section aria-labelledby="profile-referral-steps-title" className={styles.section}>
        <h2 id="profile-referral-steps-title">{ru.profile.referralStepsTitle}</h2>
        <ol className={styles.steps}>
          {ru.profile.referralSteps.map((step, index) => (
            <li key={step.title}>
              <span aria-hidden="true">{index + 1}</span>
              <strong>{step.title}</strong>
              <p>{step.description}</p>
            </li>
          ))}
        </ol>
      </section>

      <section aria-labelledby="profile-referral-statistics-title" className={styles.section}>
        <h2 id="profile-referral-statistics-title">{ru.profile.referralStatisticsTitle}</h2>
        <p className={styles.unavailable}>{ru.profile.referralStatisticsUnavailable}</p>
        <div className={styles.statistics}>
          {ru.profile.referralStatisticsCards.map((label) => (
            <article className={styles.statistic} key={label}>
              <h3>{label}</h3>
              <p>{ru.profile.referralStatisticsUnavailable}</p>
            </article>
          ))}
        </div>
      </section>

      <ProfileReferralFaq />
    </div>
  );
}
