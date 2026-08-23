import { ru } from "@/i18n/ru";

import styles from "./AccountUpdatesPanel.module.css";

type AccountUpdatesPanelProps = {
  id: string;
};

export function AccountUpdatesPanel({ id }: AccountUpdatesPanelProps) {
  return (
    <section aria-label={ru.account.updatesPanelLabel} className={styles.panel} id={id} role="region">
      <h2 className={styles.heading}>{ru.account.updatesPanelTitle}</h2>

      <div className={styles.feed}>
        <article className={styles.updateCard}>
          <h3 className={styles.updateTitle}>{ru.account.updatesIdeaTitle}</h3>
          <p className={styles.updateDescription}>{ru.account.updatesIdeaDescription}</p>
          <button className={styles.updateAction} disabled type="button">
            {ru.account.updatesIdeaAction}
          </button>
        </article>
      </div>
    </section>
  );
}
