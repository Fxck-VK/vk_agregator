import { ru } from "@/i18n/ru";

import styles from "./loading.module.css";

export default function WorkspaceLoading() {
  return (
    <section aria-live="polite" className={styles.loading} role="status">
      <div className={styles.indicator} />
      <p>{ru.workspace.navigationLoading}</p>
    </section>
  );
}
