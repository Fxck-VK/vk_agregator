import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { getLegalCatalog } from "@/features/legal/catalog.server";
import { LegalPageFrame } from "@/features/legal/LegalPageFrame";
import { LegalIndexContent } from "@/features/legal/LegalPageContent";

export const dynamic = "force-dynamic";
export async function generateMetadata() {
  const { legal } = await getRequestDictionary();
  return { title: legal.title, robots: { index: false, follow: true } };
}
export default async function LegalIndex() {
  const locale = await getRequestLocale();
  const { legal } = await getRequestDictionary();
  const catalog = getLegalCatalog();
  const documents = catalog.documents.map(doc => ({ slug: doc.slug, title: legal.titles[doc.slug] }));
  return <LegalPageFrame locale={locale} documents={documents}><LegalIndexContent catalog={catalog} locale={locale} /></LegalPageFrame>;
}
