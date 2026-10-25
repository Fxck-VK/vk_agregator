import {localizePublicData} from "@/i18n/public/public-copy";
import type { Metadata } from "next";
import { TattooSolution } from "@/features/task-solution/TattooSolution";
import { tattooEn } from "@/i18n/public/tattoo-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { tattooPublication } from "./publication";

export async function generateMetadata(): Promise<Metadata> {
  return { ...publicPageMetadata(await getRequestLocale(), tattooPublication), ...localizePublicData(tattooEn.metadata,await getRequestLocale()) };
}
export default async function TattooPage() {
  return <TattooSolution />;
}
