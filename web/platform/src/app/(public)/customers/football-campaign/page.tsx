import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from 'next';
import {CustomerStory} from '@/features/customer-story/CustomerStory';
import {customerStoryEn} from '@/i18n/public/customer-story-en';
import {getRequestLocale} from '@/i18n/server';
import {publicPageMetadata} from '@/i18n/seo';
import {customerStoryPublication} from './publication';
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),customerStoryPublication),...localizePublicData(customerStoryEn.metadata,await getRequestLocale())};}
export default async function Page(){return <CustomerStory/>;}
