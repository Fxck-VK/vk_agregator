"use client";

import { useState } from "react";

import { ru } from "@/i18n/ru";

import styles from "./ProfileReferralFaq.module.css";

export function ProfileReferralFaq() {
  const [expandedIndex, setExpandedIndex] = useState<number | null>(null);

  return (
    <section aria-labelledby="profile-referral-faq-title" className={styles.section}>
      <h2 id="profile-referral-faq-title">{ru.profile.referralFaqTitle}</h2>
      <div className={styles.list}>
        {ru.profile.referralFaqItems.map((item, index) => {
          const questionId = `profile-referral-faq-question-${index}`;
          const answerId = `profile-referral-faq-answer-${index}`;
          const isExpanded = expandedIndex === index;

          return (
            <article className={styles.item} key={item.question}>
              <button
                aria-controls={answerId}
                aria-expanded={isExpanded}
                id={questionId}
                onClick={() => setExpandedIndex(isExpanded ? null : index)}
                type="button"
              >
                <span>{item.question}</span>
                <span aria-hidden="true" className={styles.indicator}>{isExpanded ? "−" : "+"}</span>
              </button>
              {isExpanded ? (
                <p aria-labelledby={questionId} id={answerId} role="region">{item.answer}</p>
              ) : null}
            </article>
          );
        })}
      </div>
    </section>
  );
}
