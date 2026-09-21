"use client";

import { FAQ } from "@/components/ui/FAQ/FAQ";
import { useDictionary } from "@/i18n/LocaleProvider";

import styles from "./ProfileReferralFaq.module.css";

export function ProfileReferralFaq() {
  const t = useDictionary();

  return (
    <section aria-labelledby="profile-referral-faq-title" className={styles.section}>
      <h2 id="profile-referral-faq-title">{t.profile.referralFaqTitle}</h2>
      <FAQ items={t.profile.referralFaqItems} name="profile-referral-faq" />
    </section>
  );
}
