import type { HTMLAttributes } from "react";

import styles from "./ContentCard.module.css";

type ContentCardProps = HTMLAttributes<HTMLElement>;

export function ContentCard({ children, className, ...props }: ContentCardProps) {
  const classes = [styles.card, className].filter(Boolean).join(" ");

  return <article className={classes} {...props}>{children}</article>;
}
