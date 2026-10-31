import { getRequestDictionary } from "@/i18n/server";
import {renderContentPage} from '@/content/templates/render';

export const dynamic = "force-dynamic";
export async function generateMetadata() {
  const { legal } = await getRequestDictionary();
  return { title: legal.title, robots: { index: false, follow: true } };
}
export default async function LegalIndex() {
  return renderContentPage('/legal');
}
