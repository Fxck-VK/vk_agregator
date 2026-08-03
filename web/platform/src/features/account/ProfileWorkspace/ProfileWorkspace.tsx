"use client";

import { ProfileBalanceCard } from "@/features/account/ProfileBalanceCard/ProfileBalanceCard";
import { ProfileIdentityCard } from "@/features/account/ProfileIdentityCard/ProfileIdentityCard";
import { ProfileLoginMethods } from "@/features/account/ProfileLoginMethods/ProfileLoginMethods";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { ru } from "@/i18n/ru";

import styles from "./ProfileWorkspace.module.css";

const overviewTabId = "profile-overview-tab";
const overviewPanelId = "profile-overview-panel";

type PrimaryIdentity = {
  hasVerifiedIdentity: boolean;
  label: string;
};

function getPrimaryIdentity(identityRefs: ReturnType<typeof useWorkspaceAccountSnapshot>["profile"]["identity_refs"]): PrimaryIdentity {
  const primaryIdentity = identityRefs.find((identity) => identity.verified && identity.label.trim() !== "");

  return {
    hasVerifiedIdentity: primaryIdentity !== undefined,
    label: primaryIdentity?.label.trim() ?? ru.account.unavailableLabel,
  };
}

export function ProfileWorkspace() {
  const { balance, profile } = useWorkspaceAccountSnapshot();
  const primaryIdentity = getPrimaryIdentity(profile.identity_refs);

  return (
    <section aria-labelledby="profile-title" className={styles.workspace}>
      <h1 className={styles.screenReaderOnly} id="profile-title">{ru.profile.title}</h1>
      <ProfileIdentityCard
        hasVerifiedIdentity={primaryIdentity.hasVerifiedIdentity}
        identityLabel={primaryIdentity.label}
      />

      <div aria-label={ru.profile.tabsLabel} className={styles.tabs} role="tablist">
        <button
          aria-controls={overviewPanelId}
          aria-selected="true"
          className={styles.tab}
          id={overviewTabId}
          role="tab"
          type="button"
        >
          {ru.profile.overviewTabLabel}
        </button>
        <button aria-selected="false" className={styles.tab} disabled role="tab" type="button">{ru.profile.referralTabLabel}</button>
      </div>

      <div aria-labelledby={overviewTabId} className={styles.content} id={overviewPanelId} role="tabpanel">
        <section aria-labelledby="profile-tariff-title" className={styles.section}>
          <h2 id="profile-tariff-title">{ru.profile.tariffSectionTitle}</h2>
          <ProfileBalanceCard balance={balance} />
        </section>

        <section aria-labelledby="profile-promo-title" className={styles.section}>
          <h2 id="profile-promo-title">{ru.profile.promoTitle}</h2>
          <div className={styles.placeholderCard}>
            <p>{ru.profile.promoDescription}</p>
            <button disabled type="button">{ru.profile.promoActionLabel}</button>
          </div>
        </section>

        <ProfileLoginMethods identityRefs={profile.identity_refs} />

        <section aria-labelledby="profile-billing-title" className={styles.section}>
          <h2 id="profile-billing-title">{ru.profile.billingTitle}</h2>
          <div className={styles.placeholderCard}>
            <p>{ru.profile.billingPlaceholder}</p>
          </div>
        </section>
      </div>
    </section>
  );
}
