"use client";

import { useState } from "react";

import { ProfileBalanceCard } from "@/features/account/ProfileBalanceCard/ProfileBalanceCard";
import { ProfileIdentityCard } from "@/features/account/ProfileIdentityCard/ProfileIdentityCard";
import { ProfileLoginMethods } from "@/features/account/ProfileLoginMethods/ProfileLoginMethods";
import { ProfileReferralProgram } from "@/features/account/ProfileReferralProgram/ProfileReferralProgram";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { ru } from "@/i18n/ru";

import styles from "./ProfileWorkspace.module.css";

const overviewTabId = "profile-overview-tab";
const overviewPanelId = "profile-overview-panel";
const referralTabId = "profile-referral-tab";
const referralPanelId = "profile-referral-panel";

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
  const [activeTab, setActiveTab] = useState<"overview" | "referral">("overview");

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
          aria-selected={activeTab === "overview"}
          className={styles.tab}
          id={overviewTabId}
          onClick={() => setActiveTab("overview")}
          role="tab"
          type="button"
        >
          {ru.profile.overviewTabLabel}
        </button>
        <button
          aria-controls={referralPanelId}
          aria-selected={activeTab === "referral"}
          className={styles.tab}
          id={referralTabId}
          onClick={() => setActiveTab("referral")}
          role="tab"
          type="button"
        >
          {ru.profile.referralTabLabel}
        </button>
      </div>

      {activeTab === "overview" ? (
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
      ) : (
        <div aria-labelledby={referralTabId} className={styles.content} id={referralPanelId} role="tabpanel">
          <ProfileReferralProgram />
        </div>
      )}
    </section>
  );
}
