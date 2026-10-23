"use client";
import Link from "@/i18n/Link";
import { useDictionary } from "@/i18n/LocaleProvider";
import { legalSlugs } from "./model";
import styles from "./Legal.module.css";

export function LegalLinks(){
 const {legal:t}=useDictionary();
 return <nav className={styles.links} aria-label={t.navigation}>{legalSlugs.map(slug=><Link key={slug} href={`/legal/${slug}`}>{t.titles[slug]}</Link>)}</nav>;
}
export function LegalNotice(){
 const {legal:t}=useDictionary();
 return <p className={styles.notice}>{t.notice}{" "}<Link href="/legal/user-agreement" target="_blank" rel="noopener noreferrer">{t.titles["user-agreement"]}</Link>{" "}{t.and}{" "}<Link href="/legal/privacy-policy" target="_blank" rel="noopener noreferrer">{t.titles["privacy-policy"]}</Link>.</p>;
}
export function LegalPaymentNotice(){
 const {legal:t}=useDictionary();
 return <div className={styles.notice}><p>{t.checkout}</p><Link href="/legal/paid-access" target="_blank" rel="noopener noreferrer">{t.titles["paid-access"]}</Link>{" · "}<Link href="/legal/payment-details" target="_blank" rel="noopener noreferrer">{t.titles["payment-details"]}</Link></div>;
}
