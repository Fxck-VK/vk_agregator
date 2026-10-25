import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {MaterialCategoryPage} from "@/features/material-category/MaterialCategoryPage";
import {materialCategoryEn} from "@/i18n/public/material-category-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {natureCategoryPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),natureCategoryPublication),...localizePublicData(materialCategoryEn.metadata,await getRequestLocale())};}
export default async function CategoryPage(){return <MaterialCategoryPage/>;}
