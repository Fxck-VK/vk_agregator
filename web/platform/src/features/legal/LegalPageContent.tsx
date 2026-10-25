import type { Locale } from "@/i18n/locales";
import Link from "@/i18n/Link";
import { legalText } from "@/i18n/legal";
import { legalPageText } from "@/i18n/public/legal-page";
import type { getLegalCatalog } from "./catalog.server";
import styles from "@/components/public/help-center/HelpCenter.module.css";

type Catalog = ReturnType<typeof getLegalCatalog>;
type Document = Catalog["documents"][number];

function Edition({ catalog, locale }: { catalog: Catalog; locale: Locale }) {
  const t = legalText[locale];
  return <div className={styles.edition}>
    <p><span>{catalog.metadata.published ? t.effectiveDate : t.date} <time dateTime={catalog.date}>{catalog.date}</time></span><span className={styles.version}>{t.version} {catalog.metadata.version}</span></p>
    {!catalog.metadata.published ? <p className={styles.status} role="status">{t.draft}</p> : null}
    {locale === "en" ? <p className={styles.language}>{t.language}</p> : null}
  </div>;
}

export function LegalDocumentContent({ catalog, document, locale }: { catalog: Catalog; document: Document; locale: Locale }) {
  const t = legalText[locale], page = legalPageText[locale];
  return <><h1 className={styles.title}>{locale === "en" ? document.title_en : document.title}</h1><Edition catalog={catalog} locale={locale} />
    {!catalog.configured ? <p className={styles.status} role="status">{t.missing}</p> : <article className={styles.document} lang="ru">{document.sections.map((section, index) => <section key={index}>
      <h2 id={`section-${index + 1}`}>{section.title}</h2>{section.paragraphs.map((paragraph, i) => <p key={i}>{paragraph}</p>)}
      {section.table.length ? <><div className={styles.tableHint}>{page.tableHint}</div><div className={styles.tableScroll} role="region" aria-label={`${page.table}: ${section.title}`} tabIndex={0}><table><thead><tr>{section.table[0].map((cell, i) => <th key={i} scope="col">{cell}</th>)}</tr></thead><tbody>{section.table.slice(1).map((row, i) => <tr key={i}>{row.map((cell, j) => <td key={j}>{cell}</td>)}</tr>)}</tbody></table></div></> : null}
    </section>)}</article>}
  </>;
}

export function LegalIndexContent({ catalog, locale }: { catalog: Catalog; locale: Locale }) {
  const t = legalText[locale], page = legalPageText[locale];
  return <><h1 className={styles.title}>{page.title}</h1><p className={styles.intro}>{page.intro}</p><Edition catalog={catalog} locale={locale} />
    <ul className={styles.documentIndex}>{catalog.documents.map(document => <li key={document.slug}><Link href={`/legal/${document.slug}`}><span>{t.titles[document.slug]}</span><span aria-hidden="true">↗</span></Link></li>)}</ul>
    <p className={styles.contact}><a href="mailto:support@epifera.com">{page.support}</a></p>
  </>;
}
