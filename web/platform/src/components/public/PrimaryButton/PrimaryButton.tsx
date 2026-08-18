import Link, { type LinkProps } from "next/link";
import type { AnchorHTMLAttributes } from "react";

import styles from "./PrimaryButton.module.css";

type PrimaryButtonProps = LinkProps & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, keyof LinkProps>;

export function PrimaryButton({ children, className, ...props }: PrimaryButtonProps) {
  const classes = [styles.link, className].filter(Boolean).join(" ");

  return <Link className={classes} {...props}>{children}</Link>;
}
