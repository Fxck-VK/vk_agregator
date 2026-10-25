import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {PricingLanding} from "@/features/pricing/PricingLanding";
import {pricingEn} from "@/i18n/public/pricing-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {pricingPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),pricingPublication),...localizePublicData(pricingEn.metadata,await getRequestLocale())};}
export default async function Page(){return <PricingLanding/>;}
