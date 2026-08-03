import { ru } from "@/i18n/ru";

import styles from "./ProfileIdentityCard.module.css";

type ProfileIdentityCardProps = {
  hasVerifiedIdentity: boolean;
  identityLabel: string;
};

export function ProfileIdentityCard({ hasVerifiedIdentity, identityLabel }: ProfileIdentityCardProps) {
  return (
    <section aria-label={ru.profile.identityCardLabel} className={styles.card}>
      <span aria-hidden="true" className={styles.identityMark}>ID</span>
      <div className={styles.content}>
        <strong>{identityLabel}</strong>
        <p>{hasVerifiedIdentity ? ru.profile.verifiedIdentity : ru.profile.noVerifiedIdentity}</p>
      </div>
    </section>
  );
}
