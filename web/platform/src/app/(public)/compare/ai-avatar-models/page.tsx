import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {AvatarComparison} from "@/features/comparison/AvatarComparison";
import {comparisonEn} from "@/i18n/public/comparison-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {comparisonPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return{...publicPageMetadata(await getRequestLocale(),comparisonPublication),...localizePublicData(comparisonEn.metadata,await getRequestLocale())};}
export default async function ComparisonPage(){return <AvatarComparison/>;}
