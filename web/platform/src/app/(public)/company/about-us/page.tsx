import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from 'next';
import {AboutLanding} from '@/features/about/AboutLanding';
import {aboutEn} from '@/i18n/public/about-en';
import {getRequestLocale} from '@/i18n/server';
import {publicPageMetadata} from '@/i18n/seo';
import {aboutPublication} from './publication';
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),aboutPublication),...localizePublicData(aboutEn.metadata,await getRequestLocale())};}
export default async function Page(){return <AboutLanding/>;}
