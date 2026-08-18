import Link, { type LinkProps } from "next/link";
import type { AnchorHTMLAttributes } from "react";

import styles from "./SecondaryButton.module.css";

type SecondaryButtonProps = LinkProps & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, keyof LinkProps>;

export function SecondaryButton({ children, className, ...props }: SecondaryButtonProps) {
  const classes = [styles.link, className].filter(Boolean).join(" ");

  return <Link className={classes} {...props}>{children}</Link>;
}
