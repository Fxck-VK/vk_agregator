import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from 'next';
import {EventLanding} from '@/features/event-project/EventLanding';
import {eventProjectEn} from '@/i18n/public/event-project-en';
import {getRequestLocale} from '@/i18n/server';
import {publicPageMetadata} from '@/i18n/seo';
import {eventProjectPublication} from './publication';

export async function generateMetadata():Promise<Metadata>{
  return {...publicPageMetadata(await getRequestLocale(),eventProjectPublication),...localizePublicData(eventProjectEn.metadata,await getRequestLocale())};
}
export default async function Page(){
  return <EventLanding/>;
}
