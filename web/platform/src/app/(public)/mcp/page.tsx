import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {IntegrationLanding} from "@/features/integration/IntegrationLanding";
import {mcpEn} from "@/i18n/public/integration-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {mcpPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),mcpPublication),...localizePublicData(mcpEn.metadata,await getRequestLocale())};}
export default async function Page(){return <IntegrationLanding/>;}
