"use client";

import { type ReactNode } from "react";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import { brandName } from "@/config/brand";
import Link from "@/i18n/Link";
import { LoginShowcase } from "./LoginShowcase";
import styles from "./LoginWindow.module.css";

export function LoginWindow({ children }: Readonly<{ children: ReactNode }>) {
  return <div className={styles.window} data-testid="login-window" data-presentation="reference">
    <div className={styles.account}>
      <div className={styles.formArea}><Link className={styles.brand} href="/" aria-label={brandName}><BrandLogo size={44} /></Link>{children}</div>
    </div>
    <LoginShowcase />
  </div>;
}
