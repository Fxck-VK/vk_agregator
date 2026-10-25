import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from 'next';
import {AuthorLanding} from '@/features/author/AuthorLanding';
import {authorMaterials} from '@/features/author/author-catalog';
import {paginateAuthorMaterials} from '@/features/author/author-model';
import {authorEn} from '@/i18n/public/author-en';
import {getRequestLocale} from '@/i18n/server';
import {publicPageMetadata} from '@/i18n/seo';
import {authorPublication} from './publication';
type Props={searchParams:Promise<{page?:string|string[]}>};

export async function generateMetadata():Promise<Metadata>{
  return {...publicPageMetadata(await getRequestLocale(),authorPublication),...localizePublicData(authorEn.metadata,await getRequestLocale())};
}
export default async function Page({searchParams}:Props){
  const {page}=await searchParams;
  return <AuthorLanding items={localizePublicData(authorMaterials,await getRequestLocale())} page={paginateAuthorMaterials(authorMaterials,page).page}/>;
}
