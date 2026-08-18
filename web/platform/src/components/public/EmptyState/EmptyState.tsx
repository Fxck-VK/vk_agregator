import type { ReactNode } from "react";

import styles from "./EmptyState.module.css";

type EmptyStateProps = {
  action?: ReactNode;
  description: string;
  icon?: ReactNode;
  title: string;
};

export function EmptyState({ action, description, icon, title }: EmptyStateProps) {
  return (
    <section className={styles.root} role="status">
      <span aria-hidden="true" className={styles.icon}>{icon ?? "✦"}</span>
      <h2>{title}</h2>
      <p>{description}</p>
      {action ? <div className={styles.action}>{action}</div> : null}
    </section>
  );
}
