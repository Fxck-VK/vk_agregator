import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {ExampleCollection} from "@/features/collection/ExampleCollection";
import {cinematicCollection,collectionEn} from "@/i18n/public/collection-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {collectionPublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),collectionPublication),...localizePublicData(collectionEn.metadata,await getRequestLocale())};}
export default async function CollectionPage(){return <ExampleCollection collection={cinematicCollection}/>;}
