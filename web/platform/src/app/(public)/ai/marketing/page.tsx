import {localizePublicData} from "@/i18n/public/public-copy";
import type { Metadata } from "next";
import { MarketingAudience } from "@/features/audience/MarketingAudience";
import { marketingEn } from "@/i18n/public/marketing-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { marketingPublication } from "./publication";
export async function generateMetadata(): Promise<Metadata> { return {...publicPageMetadata(await getRequestLocale(),marketingPublication),...localizePublicData(marketingEn.metadata,await getRequestLocale())}; }
export default async function MarketingPage() { return <MarketingAudience/>; }
