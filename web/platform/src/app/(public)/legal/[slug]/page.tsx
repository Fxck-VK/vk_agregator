import { notFound } from "@/i18n/server";
import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import Link from "@/i18n/Link";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { getLegalCatalog } from "@/features/legal/catalog.server";
import styles from "@/features/legal/Legal.module.css";

export const dynamic="force-dynamic";
export async function generateMetadata({params}:{params:Promise<{slug:string}>}){
 const {slug}=await params;const catalog=getLegalCatalog();const doc=catalog.documents.find(d=>d.slug===slug);
 return {title:doc?`${doc.title} — Epifera`:undefined,robots:{index:false,follow:true}};
}
export default async function LegalDocument({params}:{params:Promise<{slug:string}>}){
 const {slug}=await params;const {legal:t}=await getRequestDictionary();const locale=await getRequestLocale();const catalog=getLegalCatalog();
 const doc=catalog.documents.find(item=>item.slug===slug);if(!doc)notFound();
 return <PageContainer className={styles.page} size="content"><Link href="/legal">← {t.title}</Link><h1>{locale==="en"?doc.title_en:doc.title}</h1>
  <p className={styles.meta}>{t.version} {catalog.metadata.version} · {catalog.metadata.published?t.effectiveDate:t.date} {catalog.date}</p>
  {!catalog.metadata.published?<p className={styles.draft}>{t.draft}</p>:null}
  {locale==="en"?<p>{t.language}</p>:null}
  {!catalog.configured?<p className={styles.draft}>{t.missing}</p>:<article className={styles.document} lang="ru">{doc.sections.map((section,index)=><section key={index}><h2>{section.title}</h2>{section.paragraphs.map((p,i)=><p key={i}>{p}</p>)}{section.table.length?<table><thead><tr>{section.table[0].map((c,i)=><th key={i} scope="col">{c}</th>)}</tr></thead><tbody>{section.table.slice(1).map((row,i)=><tr key={i}>{row.map((c,j)=><td key={j}>{c}</td>)}</tr>)}</tbody></table>:null}</section>)}</article>}
 </PageContainer>;
}
