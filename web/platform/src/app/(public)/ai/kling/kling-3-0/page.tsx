import type { Metadata } from "next";
import { Kling30Landing } from "@/features/model-version/Kling30Landing";
import { kling30En } from "@/i18n/public/kling30-en";
import { getRequestLocale } from "@/i18n/server";
import { publicPageMetadata } from "@/i18n/seo";
import { kling30Publication } from "./publication";

export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),kling30Publication),...kling30En.metadata};}
export default async function VersionPage(){return <Kling30Landing/>;}
