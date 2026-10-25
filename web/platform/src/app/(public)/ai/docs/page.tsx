import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {brandName} from "@/config/brand";
import {DocumentationCenter} from "@/features/documentation/DocumentationCenter";
import {documentationEn} from "@/i18n/public/documentation-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {documentationPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),documentationPublication),title:`${localizePublicData(documentationEn,await getRequestLocale()).title} | ${brandName}`,description:localizePublicData(documentationEn,await getRequestLocale()).description};}
export default async function HelpCenterPage(){return <DocumentationCenter/>;}
