"use client";

import { useRef, useState } from "react";
import { PromoCodeDialog } from "@/components/layout/WorkspaceHeader/PromoCodeDialog";
import { useSubscriptionPromotion } from "@/features/subscriptions/SubscriptionPromotion";

import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { ProfileBalanceCard } from "@/features/account/ProfileBalanceCard/ProfileBalanceCard";
import { ProfileSubscriptionCard } from "@/features/subscriptions/ProfileSubscriptionCard";
import { ProfileIdentityCard } from "@/features/account/ProfileIdentityCard/ProfileIdentityCard";
import { ProfileLoginMethods } from "@/features/account/ProfileLoginMethods/ProfileLoginMethods";
import { ProfileReferralProgram } from "@/features/account/ProfileReferralProgram/ProfileReferralProgram";
import { ProfileBillingHistory } from "@/features/account/ProfileBillingHistory/ProfileBillingHistory";
import { AccountSecurity } from "@/features/account/AccountSecurity/AccountSecurity";
import { getAccountDisplayIdentity } from "@/features/account/account-display";
import { defaultAuthMethods, type AuthMethods } from "@/lib/auth/methods";
import { useWorkspaceAccountSnapshot } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { useDictionary } from "@/i18n/LocaleProvider";
import { LegalConsentPreferences } from "@/features/legal/LegalConsentPreferences";
import type { LegalMetadata } from "@/features/legal/model";

import styles from "./ProfileWorkspace.module.css";

const overviewTabId = "profile-overview-tab";
const referralTabId = "profile-referral-tab";
const profilePanelId = "profile-content-panel";
type ProfileTab = "overview" | "referral" | "security";

export function ProfileWorkspace({ methods = defaultAuthMethods, preview = false, oauthStatus, legalMetadata }: { methods?: AuthMethods; preview?: boolean; oauthStatus?: string; legalMetadata?:LegalMetadata }) {
  const t = useDictionary();
  const { balance, profile } = useWorkspaceAccountSnapshot();
  const primaryIdentity = getAccountDisplayIdentity(profile.identity_refs, t);
  const [activeTab, setActiveTab] = useState<ProfileTab>(oauthStatus ? "security" : "overview");
  const promotion = useSubscriptionPromotion(), promoButtonRef = useRef<HTMLButtonElement>(null);
  const [promoOpen, setPromoOpen] = useState(false);

  return (
    <section aria-labelledby="profile-title" className={styles.workspace}>
      <h1 className={styles.screenReaderOnly} id="profile-title">{t.profile.title}</h1>
      <ProfileIdentityCard
        key={profile.account_id}
        hasVerifiedIdentity={primaryIdentity.hasVerifiedIdentity}
        identityLabel={primaryIdentity.label}
        publicID={profile.public_id}
      />

      <ModeSwitchPanel<ProfileTab>
        activeID={activeTab}
        ariaLabel={t.profile.tabsLabel}
        items={[
          { id: "overview", label: t.profile.overviewTabLabel, elementID: overviewTabId, ariaControls: profilePanelId },
          { id: "referral", label: t.profile.referralTabLabel, elementID: referralTabId, ariaControls: profilePanelId },
          { id: "security", label: t.auth.security, elementID: "profile-security-tab", ariaControls: profilePanelId },
        ]}
        onChange={setActiveTab}
        semantics="tabs"
      />

      <div
        aria-labelledby={activeTab === "overview" ? overviewTabId : activeTab === "referral" ? referralTabId : "profile-security-tab"}
        className={styles.content}
        id={profilePanelId}
        role="tabpanel"
      >
        {activeTab === "overview" ? (
          <>
            <section aria-labelledby="profile-tariff-title" className={styles.section}>
              <h2 id="profile-tariff-title">{t.profile.tariffSectionTitle}</h2>
              <ProfileBalanceCard balance={balance} />
              <ProfileSubscriptionCard key={profile.account_id} preview={preview}/>
            </section>

            <section aria-labelledby="profile-promo-title" className={styles.section}>
              <h2 id="profile-promo-title">{t.profile.promoTitle}</h2>
              <div className={styles.placeholderCard}>
                <p className={styles.promotionSummary}>{promotion.quote ? t.promotions.applied(promotion.quote.code, promotion.quote.percent) : t.promotions.description}</p>
                <button aria-haspopup="dialog" aria-expanded={promoOpen} disabled={promotion.pending} onClick={() => setPromoOpen(true)} ref={promoButtonRef} type="button">{t.profile.promoActionLabel}</button>
              </div>
            </section>

            <ProfileLoginMethods identityRefs={profile.identity_refs} />
            {legalMetadata?<LegalConsentPreferences metadata={legalMetadata} accountId={profile.account_id} preview={preview}/>:null}

            <section aria-labelledby="profile-billing-title" className={styles.section}>
              <h2 id="profile-billing-title">{t.profile.billingTitle}</h2>
              <ProfileBillingHistory accountId={profile.account_id} preview={preview} />
            </section>
          </>
        ) : activeTab === "security" ? (
          <AccountSecurity key={profile.account_id} profile={profile} methods={methods} preview={preview} oauthStatus={oauthStatus} />
        ) : (
          <ProfileReferralProgram key={profile.account_id} accountId={profile.account_id} preview={preview} />
        )}
      </div>
      {promoOpen ? <PromoCodeDialog onClose={() => setPromoOpen(false)} returnFocusRef={promoButtonRef} preview={preview} /> : null}
    </section>
  );
}
