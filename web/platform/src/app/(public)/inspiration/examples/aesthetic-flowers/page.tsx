import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {PublicExamplePage} from "@/features/public-example/PublicExamplePage";
import {flowerExample,publicExampleEn} from "@/i18n/public/public-example-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {publicExamplePublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),publicExamplePublication),...localizePublicData(publicExampleEn.metadata,await getRequestLocale())};}
export default async function ExamplePage(){return <PublicExamplePage example={flowerExample}/>;}
