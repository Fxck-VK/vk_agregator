import type { Metadata } from "next";
import { connection } from "next/server";
import { MarketingHome } from "@/features/marketing/MarketingHome";
import { brandName } from "@/config/brand";
import { getRequestDictionary, getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getRequestDictionary();
  return { ...publicPageMetadata(await getRequestLocale(), "/"), title: `${brandName} — ${t.marketing.title}`, description: t.marketing.description };
}

export default async function HomePage() {
  await connection();
  return <MarketingHome />;
}
