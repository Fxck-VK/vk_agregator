"use client";
import {authorPath} from "@/i18n/public/author-en";
import {EditorialPage} from "@/components/public/editorial/EditorialPage";
import type {EditorialImage,EditorialPageData} from "@/components/public/editorial/types";
import {artPromptsEn as copy} from "@/i18n/public/art-prompts-en";
import {artPromptsReferenceEn as media} from "@/i18n/public/art-prompts-reference-en";
import {comparisonReferenceEn as sharedMedia} from "@/i18n/public/comparison-reference-en";
import {ArticleBody,articleReadingMinutes} from "./ArticleBody";
import {usePublicCopy} from "@/i18n/public/use-public-copy";



export function ArtPromptsArticle(){
  const {authorPath,copy,media,sharedMedia}=usePublicCopy(publicCopySourcesForLocale,{"authorPath":"authorPath","copy":"artPromptsEn","media":"artPromptsReferenceEn","sharedMedia":"comparisonReferenceEn"});
const sharedImage=(item:typeof sharedMedia.inspiration|typeof sharedMedia.related[number]):EditorialImage=>({src:`/assets/images/comparison/${item.file}`,alt:item.alt,width:item.width,height:item.height});
const page:EditorialPageData={
  id:"art-prompts",kind:"article",title:copy.title,skip:copy.skip,
  breadcrumbs:[...copy.breadcrumbs,{label:copy.title}],
  author:{label:copy.author,href:authorPath,date:"2026-10-25",dateLabel:copy.draft},
  minutes:articleReadingMinutes(copy),readTime:copy.readTime,controls:copy.controls,onPage:copy.onPage,sections:copy.sections,
  hero:media.hero,source:copy.source,inspiration:{...copy.inspiration,image:sharedImage(sharedMedia.inspiration)},
  related:{title:copy.related.title,items:copy.related.items.map((item,index)=>({...item,image:sharedImage(sharedMedia.related[index])}))},
};
return <EditorialPage page={page}><ArticleBody article={copy} images={media.illustrations} source={copy.source} notes={copy.notes}/></EditorialPage>;}

const publicCopySourcesForLocale={authorPath,copy,media,sharedMedia};
