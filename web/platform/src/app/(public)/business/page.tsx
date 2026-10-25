import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from 'next';
import {BusinessLanding} from '@/features/business/BusinessLanding';
import {businessEn} from '@/i18n/public/business-en';
import {getRequestLocale} from '@/i18n/server';
import {publicPageMetadata} from '@/i18n/seo';
import {businessPublication} from './publication';
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),businessPublication),...localizePublicData(businessEn.metadata,await getRequestLocale())};}
export default async function Page(){return <BusinessLanding/>;}
