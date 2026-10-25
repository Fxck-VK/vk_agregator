import {localizePublicData} from "@/i18n/public/public-copy";
import type { Metadata } from "next";
import { ToolsOverview } from "@/features/ai-tools-overview/ToolsOverview";
import { aiToolsEn } from "@/i18n/public/ai-tools-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { toolsOverviewPublication } from "./publication";

export async function generateMetadata(): Promise<Metadata> {
  return { ...publicPageMetadata(await getRequestLocale(), toolsOverviewPublication), ...localizePublicData(aiToolsEn.metadata,await getRequestLocale()) };
}
export default async function ToolsPage() {
  return <ToolsOverview />;
}
