"use client";

import { type KeyboardEvent, useRef, useState } from "react";

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
const profileTabs = ["overview", "referral"] as const;

type ProfileTab = (typeof profileTabs)[number];

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
  const [activeTab, setActiveTab] = useState<ProfileTab>("overview");
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  function selectTab(tab: ProfileTab) {
    setActiveTab(tab);
  }

  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, currentTab: ProfileTab) {
    const currentIndex = profileTabs.indexOf(currentTab);
    let nextIndex: number | null = null;

    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      nextIndex = (currentIndex + 1) % profileTabs.length;
    } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      nextIndex = (currentIndex - 1 + profileTabs.length) % profileTabs.length;
    } else if (event.key === "Home") {
      nextIndex = 0;
    } else if (event.key === "End") {
      nextIndex = profileTabs.length - 1;
    }

    if (nextIndex === null) {
      return;
    }

    event.preventDefault();
    selectTab(profileTabs[nextIndex]);
    tabRefs.current[nextIndex]?.focus();
  }

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
          onClick={() => selectTab("overview")}
          onKeyDown={(event) => handleTabKeyDown(event, "overview")}
          ref={(element) => {
            tabRefs.current[0] = element;
          }}
          role="tab"
          tabIndex={activeTab === "overview" ? 0 : -1}
          type="button"
        >
          {ru.profile.overviewTabLabel}
        </button>
        <button
          aria-controls={referralPanelId}
          aria-selected={activeTab === "referral"}
          className={styles.tab}
          id={referralTabId}
          onClick={() => selectTab("referral")}
          onKeyDown={(event) => handleTabKeyDown(event, "referral")}
          ref={(element) => {
            tabRefs.current[1] = element;
          }}
          role="tab"
          tabIndex={activeTab === "referral" ? 0 : -1}
          type="button"
        >
          {ru.profile.referralTabLabel}
        </button>
      </div>

      <div
        aria-labelledby={overviewTabId}
        className={styles.content}
        hidden={activeTab !== "overview"}
        id={overviewPanelId}
        role="tabpanel"
      >
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

      <div
        aria-labelledby={referralTabId}
        className={styles.content}
        hidden={activeTab !== "referral"}
        id={referralPanelId}
        role="tabpanel"
      >
        <ProfileReferralProgram />
      </div>
    </section>
  );
}
