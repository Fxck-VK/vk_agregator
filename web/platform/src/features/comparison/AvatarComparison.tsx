"use client";
import {authorPath} from "@/i18n/public/author-en";
import {EditorialPage} from "@/components/public/editorial/EditorialPage";
import type {EditorialImage,EditorialPageData} from "@/components/public/editorial/types";
import {comparisonEn as copy} from "@/i18n/public/comparison-en";
import {comparisonReferenceEn as media} from "@/i18n/public/comparison-reference-en";
import {ComparisonArticle,comparisonReadingMinutes} from "./ComparisonArticle";
import {usePublicCopy} from "@/i18n/public/use-public-copy";

const image=(item:typeof media.hero|typeof media.inspiration|typeof media.related[number]):EditorialImage=>({src:`/assets/images/comparison/${item.file}`,alt:item.alt,width:item.width,height:item.height});
const page:EditorialPageData={
  id:"comparison",kind:"comparison",title:copy.title,skip:copy.skip,
  breadcrumbs:[{label:copy.breadcrumb,href:"/ai"},{label:copy.category}],
  author:{label:copy.author,href:authorPath,date:"2026-10-25",dateLabel:copy.draft},
  minutes:comparisonReadingMinutes,readTime:copy.readTime,controls:copy.controls,onPage:copy.onPage,sections:copy.sections,
  hero:image(media.hero),intro:copy.intro,source:copy.source,inspiration:{...copy.inspiration,image:image(media.inspiration)},
  related:{title:copy.related.title,items:copy.related.items.map((item,index)=>({...item,image:image(media.related[index])}))},
};
export function AvatarComparison(){
  const {page} = usePublicCopy(publicCopySourcesForLocale);
return <EditorialPage page={page}><ComparisonArticle/></EditorialPage>;}

const publicCopySourcesForLocale={authorPath,copy,media,page};
