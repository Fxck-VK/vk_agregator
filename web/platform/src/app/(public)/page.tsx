import {localizePublicData} from "@/i18n/public/public-copy";
import type { Metadata } from "next";
import { connection } from "next/server";
import { PublicHome } from "@/features/public-home/PublicHome";
import { homeEn } from "@/i18n/public/home-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { homePublication } from "./publication";

export async function generateMetadata(): Promise<Metadata> {
  const locale = await getRequestLocale();
  return { ...publicPageMetadata(locale, homePublication), ...localizePublicData(homeEn.metadata,locale) };
}

export default async function HomePage() {
  await connection();
  return <PublicHome copy={localizePublicData(homeEn,await getRequestLocale())} />;
}
