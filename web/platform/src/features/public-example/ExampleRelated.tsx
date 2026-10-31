"use client";
import {useState,type CSSProperties,type RefObject} from "react";
import {LandingCarousel} from "@/components/public/landing/LandingCarousel";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {publicExampleEn as t,type PublicExampleRecord,type PublicExampleImage} from "@/i18n/public/public-example-en";
import styles from "./PublicExample.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {publicTagLabel} from "@/features/collection/tag-labels";


export function RelatedRow({label,items,kind,onView}:{label:string;items:readonly PublicExampleImage[];kind:"series"|"similar"|"designs"|"stills";onView:(items:readonly PublicExampleImage[],index:number)=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"publicExampleEn"});

  return <section className={styles.relatedRow}><div className={styles.sectionHeading}><h2>{label}</h2>{kind==="series"?<button type="button" onClick={()=>onView(items,0)}>{t.seeAll}</button>:null}</div><LandingCarousel label={label} previous={t.previous} next={t.next} bounded controls="overlay" className={styles.carouselItems}>{items.map((item,index)=><figure key={item.id} data-example-related={kind} style={{"--thumbnail-ratio":item.width/item.height} as CSSProperties}><button type="button" aria-label={`${t.viewRelated} ${item.alt}`} onClick={()=>onView(items,index)}><MediaImage src={item.src} alt={item.alt} width={item.width} height={item.height} fit="cover" loading="lazy" passive showLoading={false}/></button></figure>)}</LandingCarousel></section>;
}
export function ExampleRelated({example,onView,sectionRef}:{example:PublicExampleRecord;onView:(items:readonly PublicExampleImage[],index:number)=>void;sectionRef:RefObject<HTMLElement|null>}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"publicExampleEn"});
  const locale=useLocale();

  const [tag,setTag]=useState<string|null>(null),similar=example.similar.filter(item=>tag===null||item.tags.includes(tag));
  return <section ref={sectionRef} id="related-examples" className={styles.related} tabIndex={-1} aria-label={t.similarTitle}><RelatedRow label={t.series} items={example.series} kind="series" onView={onView}/><RelatedRow label={t.similarTitle} items={similar} kind="similar" onView={onView}/><div className={styles.tags} role="group" aria-label={t.filterLabel}>{example.taxonIds.map(value=><button type="button" aria-label={`${t.filterPrefix} ${publicTagLabel(value,locale)}`} aria-pressed={tag===value} key={value} onClick={()=>setTag(current=>current===value?null:value)}>{publicTagLabel(value,locale)}</button>)}</div>{tag?<div className={styles.filtered}><span aria-live="polite">{similar.length} {t.filtered}</span><button type="button" onClick={()=>setTag(null)}>{t.all}</button></div>:null}</section>;
}

const publicCopySourcesForLocale={t};
