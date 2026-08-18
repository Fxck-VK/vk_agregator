import Link from "next/link";
import type { ReactNode } from "react";

import styles from "./ModelPreviewCard.module.css";

type ModelPreviewCardProps = {
  actionLabel: string;
  description: string;
  eyebrow?: string;
  href: string;
  icon?: ReactNode;
  title: string;
};

export function ModelPreviewCard({ actionLabel, description, eyebrow, href, icon, title }: ModelPreviewCardProps) {
  return (
    <article className={styles.card}>
      <span aria-hidden="true" className={styles.icon}>{icon ?? "✦"}</span>
      <div className={styles.copy}>
        {eyebrow ? <p className={styles.eyebrow}>{eyebrow}</p> : null}
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
      <Link className={styles.action} href={href}>{actionLabel} <span aria-hidden="true">→</span></Link>
    </article>
  );
}
