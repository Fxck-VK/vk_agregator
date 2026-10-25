import { FileIcon } from "@/components/icons/FileIcon";
import Link from "@/i18n/Link";
import type { Locale } from "@/i18n/locales";
import { legalNavigationGroups, legalPageText, type LegalNavigationItem } from "@/i18n/public/legal-page";
import type { LegalSlug } from "./model";
import styles from "@/components/public/help-center/HelpCenter.module.css";

export function LegalNavigation({ locale, documents, currentSlug, onNavigate }: {
  locale: Locale; documents: readonly LegalNavigationItem[]; currentSlug?: LegalSlug; onNavigate?: () => void;
}) {
  const t = legalPageText[locale];
  return <nav className={styles.documentNavigation} aria-label={t.title}>
    <Link href="/legal" className={styles.navigationTitle} onClick={onNavigate}><FileIcon />{t.title}</Link>
    <div className={styles.groups}>
      {legalNavigationGroups.map(group => <details key={group.key} open={group.slugs.some(slug => slug === currentSlug) || undefined}>
        <summary><span className={styles.chevron} aria-hidden="true">›</span>{t.groups[group.key]}</summary>
        <div className={styles.groupLinks}>{documents.filter(doc => (group.slugs as readonly LegalSlug[]).includes(doc.slug)).map(doc =>
          <Link href={`/legal/${doc.slug}`} key={doc.slug} aria-current={doc.slug === currentSlug ? "page" : undefined} onClick={onNavigate}>{doc.title}</Link>,
        )}</div>
      </details>)}
      <details><summary><span className={styles.chevron} aria-hidden="true">›</span>{t.help}</summary>
        <div className={styles.groupLinks}><a href="mailto:support@epifera.com" onClick={onNavigate}>{t.support}</a></div>
      </details>
    </div>
  </nav>;
}
