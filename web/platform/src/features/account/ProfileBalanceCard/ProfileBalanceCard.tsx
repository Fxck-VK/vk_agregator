import { ru } from "@/i18n/ru";

import styles from "./ProfileBalanceCard.module.css";

type ProfileBalanceCardProps = {
  balance: number | null;
};

export function ProfileBalanceCard({ balance }: ProfileBalanceCardProps) {
  const isBalanceAvailable = balance !== null;

  return (
    <section aria-label={ru.profile.tariffSectionTitle} className={styles.card}>
      <div className={styles.plan}>
        <p>{ru.profile.planLabel}</p>
        <strong>{ru.profile.planPlaceholder}</strong>
      </div>
      <div
        aria-busy={!isBalanceAvailable || undefined}
        aria-label={isBalanceAvailable ? `${balance} ★` : ru.profile.balanceUnavailable}
        className={styles.balance}
      >
        <span>{ru.profile.balanceLabel}</span>
        <strong>{isBalanceAvailable ? <>{balance} <span aria-hidden="true">★</span></> : ru.profile.balanceUnavailable}</strong>
      </div>
    </section>
  );
}
