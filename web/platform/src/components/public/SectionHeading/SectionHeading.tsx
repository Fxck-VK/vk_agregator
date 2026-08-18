import type { ReactNode } from "react";

import styles from "./SectionHeading.module.css";

type SectionHeadingProps = {
  action?: ReactNode;
  align?: "start" | "center";
  description?: string;
  eyebrow?: string;
  level?: 1 | 2 | 3;
  title: string;
};

export function SectionHeading({
  action,
  align = "start",
  description,
  eyebrow,
  level = 2,
  title,
}: SectionHeadingProps) {
  const Heading = level === 1 ? "h1" : level === 3 ? "h3" : "h2";

  return (
    <div className={styles.root} data-align={align}>
      <div className={styles.copy}>
        {eyebrow ? <p className={styles.eyebrow}>{eyebrow}</p> : null}
        <Heading className={styles.title}>{title}</Heading>
        {description ? <p className={styles.description}>{description}</p> : null}
      </div>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
