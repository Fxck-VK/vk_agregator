import type { Metadata } from "next";
import { connection } from "next/server";
import { brandName } from "@/config/brand";
import { PublicHome } from "@/features/public-home/PublicHome";
import { homeEn } from "@/i18n/public/home-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { homePublication } from "./publication";

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getRequestLocale();
  return { ...publicPageMetadata(locale, homePublication), ...(locale === "en" ? homeEn.metadata : { title: brandName }) };
}

export default async function HomePage() {
  await connection();
  return await getRequestLocale() === "en" ? <PublicHome copy={homeEn} /> : <main id="public-main" />;
}
