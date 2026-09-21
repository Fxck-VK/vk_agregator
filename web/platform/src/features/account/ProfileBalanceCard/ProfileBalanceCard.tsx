"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";

import styles from "./ProfileBalanceCard.module.css";

type ProfileBalanceCardProps = {
  balance: number | null;
};

export function ProfileBalanceCard({ balance }: ProfileBalanceCardProps) {
  const t = useDictionary();
  const isBalanceAvailable = balance !== null;

  return (
    <section aria-label={t.profile.tariffSectionTitle} className={styles.card}>
      <div className={styles.plan}>
        <p>{t.profile.planLabel}</p>
        <strong>{t.profile.planPlaceholder}</strong>
      </div>
      <div
        aria-busy={!isBalanceAvailable || undefined}
        aria-label={isBalanceAvailable ? undefined : t.profile.balanceUnavailable}
        className={styles.balance}
      >
        <span>{t.profile.balanceLabel}</span>
        <strong>{isBalanceAvailable ? <CreditAmount value={balance} /> : t.profile.balanceUnavailable}</strong>
      </div>
    </section>
  );
}
