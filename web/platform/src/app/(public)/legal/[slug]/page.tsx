import { notFound, getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { brandName } from "@/config/brand";
import { getLegalCatalog } from "@/features/legal/catalog.server";
import { LegalPageFrame } from "@/features/legal/LegalPageFrame";
import { LegalDocumentContent } from "@/features/legal/LegalPageContent";

type Props = { params: Promise<{ slug: string }> };
export const dynamic = "force-dynamic";
export async function generateMetadata({ params }: Props) {
  const { slug } = await params;
  const locale = await getRequestLocale();
  const doc = getLegalCatalog().documents.find(item => item.slug === slug);
  return { title: doc ? `${locale === "en" ? doc.title_en : doc.title} — ${brandName}` : undefined, robots: { index: false, follow: true } };
}
export default async function LegalDocument({ params }: Props) {
  const { slug } = await params;
  const locale = await getRequestLocale();
  const { legal } = await getRequestDictionary();
  const catalog = getLegalCatalog();
  const document = catalog.documents.find(item => item.slug === slug);
  if (!document) notFound();
  const documents = catalog.documents.map(doc => ({ slug: doc.slug, title: legal.titles[doc.slug] }));
  return <LegalPageFrame locale={locale} documents={documents} currentSlug={document.slug}><LegalDocumentContent catalog={catalog} document={document} locale={locale} /></LegalPageFrame>;
}
