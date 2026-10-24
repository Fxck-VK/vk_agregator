"use client";

import { useSyncExternalStore } from "react";
import { BrandLogo } from "@/components/brand/BrandLogo/BrandLogo";
import { BrandName } from "@/components/brand/BrandName/BrandName";
import { brandName } from "@/config/brand";
import Link from "@/i18n/Link";
import styles from "./LandingChrome.module.css";

const query = "(max-width: 767px)";
const subscribe = (listener: () => void) => { const media = window.matchMedia?.(query); media?.addEventListener("change", listener); return () => media?.removeEventListener("change", listener); };
const snapshot = () => window.matchMedia?.(query).matches ?? false;
type Destination = { readonly label: string; readonly href: string };

export function LandingFooter({ description, groups, legal, legalLabel, supportEmail, copyright }: { description: string; groups: readonly { readonly title: string; readonly links: readonly Destination[] }[]; legal: readonly Destination[]; legalLabel: string; supportEmail: string; copyright: string }) {
  const small = useSyncExternalStore(subscribe, snapshot, () => false);
  return <footer className={styles.footer} data-landing-footer>
    <div className={styles.footerInner}><div className={styles.footerTop}>
      <div className={styles.footerIdentity}><Link href="/" className={styles.brand} aria-label={brandName}><BrandLogo size={40} /><BrandName /></Link><p>{description}</p><a href={`mailto:${supportEmail}`}>{supportEmail} <span aria-hidden="true">↗</span></a></div>
      <div className={styles.footerGroups}>{groups.map(group => <details key={group.title} open={!small || undefined}><summary>{group.title}<span aria-hidden="true">+</span></summary><div>{group.links.map(link => <Link href={link.href} key={link.label}>{link.label}</Link>)}</div></details>)}</div>
    </div><div className={styles.footerBottom}><span>© {new Date().getUTCFullYear()} {brandName}. {copyright}</span><nav aria-label={legalLabel}>{legal.map(link => <Link href={link.href} key={link.label}>{link.label}</Link>)}</nav></div></div>
  </footer>;
}
