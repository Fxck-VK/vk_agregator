import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {MaterialDetailPage} from "@/features/material-detail/MaterialDetailPage";
import {gypsophilaMaterial,materialDetailEn} from "@/i18n/public/material-detail-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {materialDetailPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),materialDetailPublication),...localizePublicData(materialDetailEn.metadata,await getRequestLocale())};}
export default async function Page(){return <MaterialDetailPage material={gypsophilaMaterial}/>;}
