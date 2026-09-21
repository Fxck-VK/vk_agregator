"use client";

import { useDictionary } from "@/i18n/LocaleProvider";


import styles from "./ProfileIdentityCard.module.css";

type ProfileIdentityCardProps = {
  hasVerifiedIdentity: boolean;
  identityLabel: string;
};

export function ProfileIdentityCard({ hasVerifiedIdentity, identityLabel }: ProfileIdentityCardProps) {
  const t = useDictionary();
  return (
    <section aria-label={t.profile.identityCardLabel} className={styles.card}>
      <span aria-hidden="true" className={styles.identityMark}>ID</span>
      <div className={styles.content}>
        <strong>{identityLabel}</strong>
        <p>{hasVerifiedIdentity ? t.profile.verifiedIdentity : t.profile.noVerifiedIdentity}</p>
      </div>
    </section>
  );
}
