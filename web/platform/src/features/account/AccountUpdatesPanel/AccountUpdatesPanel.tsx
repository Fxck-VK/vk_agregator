"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./AccountUpdatesPanel.module.css";

type AccountUpdatesPanelProps = {
  id: string;
};

export function AccountUpdatesPanel({ id }: AccountUpdatesPanelProps) {
  const t = useDictionary();
  return (
    <section aria-label={t.account.updatesPanelLabel} className={styles.panel} id={id} role="region">
      <h2 className={styles.heading}>{t.account.updatesPanelTitle}</h2>

      <ScrollArea className={styles.feed}>
        <article className={styles.updateCard}>
          <h3 className={styles.updateTitle}>{t.account.updatesIdeaTitle}</h3>
          <p className={styles.updateDescription}>{t.account.updatesIdeaDescription}</p>
          <button className={styles.updateAction} disabled type="button">
            {t.account.updatesIdeaAction}
          </button>
        </article>
      </ScrollArea>
    </section>
  );
}
