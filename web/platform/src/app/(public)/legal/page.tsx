import { PageContainer } from "@/components/public/PageContainer/PageContainer";
import Link from "@/i18n/Link";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { getLegalCatalog } from "@/features/legal/catalog.server";
import styles from "@/features/legal/Legal.module.css";

export const dynamic="force-dynamic";
export async function generateMetadata(){
 const {legal:t}=await getRequestDictionary();
 return {title:t.title,robots:{index:false,follow:true}};
}
export default async function LegalIndex(){
 const {legal:t}=await getRequestDictionary();const locale=await getRequestLocale();const catalog=getLegalCatalog();
 return <PageContainer size="content" className={styles.page}><h1>{t.title}</h1><p className={styles.meta}>{t.version} {catalog.metadata.version} · {catalog.metadata.published?t.effectiveDate:t.date} {catalog.date}</p>
  {!catalog.metadata.published?<p className={styles.draft}>{t.draft}</p>:null}
  <p>{t.language}</p><ul className={styles.index}>{catalog.documents.map(doc=><li key={doc.slug}><Link href={`/legal/${doc.slug}`}>{locale==="en"?doc.title_en:doc.title}</Link></li>)}</ul>
  <p><a href="mailto:support@epifera.com">{t.support}</a></p>
 </PageContainer>;
}
