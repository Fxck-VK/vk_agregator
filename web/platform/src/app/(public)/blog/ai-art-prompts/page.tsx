import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {ArtPromptsArticle} from "@/features/editorial-article/ArtPromptsArticle";
import {artPromptsEn} from "@/i18n/public/art-prompts-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {artPromptsPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),artPromptsPublication),...localizePublicData(artPromptsEn.metadata,await getRequestLocale())};}
export default async function ArticlePage(){return <ArtPromptsArticle/>;}
