"use client";

import { FAQ } from "@/components/ui/FAQ/FAQ";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import Link from "@/i18n/Link";
import { formatRewardPolicy, type ReferralRewardPolicy } from "@/features/account/ProfileReferralProgram/reward-policy";

import styles from "./ProfileReferralFaq.module.css";

export function ProfileReferralFaq({ policy }: { policy?: ReferralRewardPolicy }) {
  const t = useDictionary();
  const locale = useLocale();
  const items = t.profile.referralFaqItems.map((item, index) => ({ ...item,
    answer: index === 0 ? policy ? formatRewardPolicy(item.answer, policy, locale) : t.profile.referralRulesUnavailable : item.answer,
  }));

  return (
    <section aria-labelledby="profile-referral-faq-title" className={styles.section}>
      <h2 id="profile-referral-faq-title">{t.profile.referralFaqTitle}</h2>
      <FAQ items={items} name="profile-referral-faq" />
      <Link href="/legal/paid-access">{t.profile.referralLegalLink}</Link>
    </section>
  );
}
