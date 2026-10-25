import {localizePublicData} from "@/i18n/public/public-copy";
import type { Metadata } from "next";
import { KlingFamily } from "@/features/model-family/KlingFamily";
import { klingEn } from "@/i18n/public/kling-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { klingFamilyPublication } from "./publication";

export async function generateMetadata():Promise<Metadata> {
  return {...publicPageMetadata(await getRequestLocale(),klingFamilyPublication),...localizePublicData(klingEn.metadata,await getRequestLocale())};
}
export default async function KlingPage() {
  return <KlingFamily/>;
}
