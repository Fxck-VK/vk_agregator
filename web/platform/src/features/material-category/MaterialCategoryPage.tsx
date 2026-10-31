"use client";
import {useEffect,useLayoutEffect,useRef,useState} from "react";
import {ReferenceChrome,ReferenceTools} from "@/components/public/reference/ReferenceChrome";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {CollectionPreview} from "@/features/collection/CollectionPreview";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {materialCategoryEn as t,natureCategory,natureMaterials,type MaterialFilterKey} from "@/i18n/public/material-category-en";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {gypsophilaMaterial} from "@/i18n/public/material-detail-en";
import Link from "@/i18n/Link";
import {filterMaterials,initialMaterialFilters,paginateMaterials,type MaterialLayout,type MaterialSort} from "./category-model";
import {MaterialFilterBar,MaterialFilterDialog} from "./MaterialFilters";
import {MaterialGrid} from "./MaterialGrid";
import styles from "./MaterialCategory.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";
import {publicTagLabel} from "@/features/collection/tag-labels";


function CategorySearch({query,onQuery,inputRef}:{query:string;onQuery:(query:string)=>void;inputRef?:React.RefObject<HTMLInputElement|null>}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"materialCategoryEn","initialMaterialFilters":"initialMaterialFilters","natureCategory":"natureCategory","natureMaterials":"natureMaterials","gypsophilaMaterial":"gypsophilaMaterial"});

  return <form className={styles.search} role="search" onSubmit={event=>event.preventDefault()}><HelpIcon kind="search"/><input ref={inputRef} type="search" aria-label={t.search} value={query} placeholder={t.searchPlaceholder} onChange={event=>onQuery(event.target.value)}/>{query?<button type="button" aria-label={t.clearSearch} onClick={()=>{onQuery("");inputRef?.current?.focus();}}>×</button>:null}</form>;
}
export function MaterialCategoryPage(){
  const {natureCategory,t,gypsophilaMaterial,aiToolsEn,natureMaterials,initialMaterialFilters} = usePublicCopy(publicCopySourcesForLocale,{"t":"materialCategoryEn","initialMaterialFilters":"initialMaterialFilters","natureCategory":"natureCategory","natureMaterials":"natureMaterials","gypsophilaMaterial":"gypsophilaMaterial"});
  const locale=useLocale(),localizedMaterials=usePublicCopy(natureMaterials),initialFilters={...initialMaterialFilters,query:usePublicCopy(initialMaterialFilters.query)},text=(source:string)=>translatePublicText(source,locale);

  const [filters,setFilters]=useState({...initialFilters}),[layout,setLayout]=useState<MaterialLayout>("masonry"),[page,setPage]=useState(1),[overlay,setOverlay]=useState<"search"|"filters"|"tools"|null>(null),[preview,setPreview]=useState<number|null>(null),[notice,setNotice]=useState(""),[banner,setBanner]=useState(true),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),input=useRef<HTMLInputElement>(null),dialogInput=useRef<HTMLInputElement>(null),resultsRef=useRef<HTMLDivElement>(null),open=overlay!==null||preview!==null;
  const found=usePublicCopy(filterMaterials(natureMaterials,filters,item=>`${item.alt} ${localizedMaterials.find(candidate=>candidate.id===item.id)?.alt??""} ${item.tags.join(" ")} ${item.tags.map(tag=>publicTagLabel(tag,locale)).join(" ")}`)),pagination=paginateMaterials(found,page),active=natureCategory.filterDefinitions.filter(definition=>filters[definition.key]!==initialFilters[definition.key]).length+(filters.topic!=="all"?1:0)+(filters.query!==initialFilters.query?1:0);
  const launch=(action:()=>void,trigger?:HTMLElement)=>{opener.current=trigger??(document.activeElement instanceof HTMLElement?document.activeElement:null);action();};
  useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
  useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
  const changeFilter=(key:MaterialFilterKey,value:string)=>{if(!natureCategory.filterDefinitions.find(definition=>definition.key===key)?.options.some(option=>option.value===value))return;setFilters(current=>({...current,[key]:value}));setPage(1);};
  const query=(value:string)=>{setFilters(current=>({...current,query:value}));setPage(1);};
  const reset=()=>{setFilters({...initialFilters});setPage(1);};
  const changePage=(value:number)=>{setPage(value);requestAnimationFrame(()=>resultsRef.current?.scrollIntoView?.({block:"start",behavior:window.matchMedia?.("(prefers-reduced-motion: reduce)").matches?"instant":"smooth"}));};
  return <><div className={styles.page} ref={root} data-material-category><a href="#public-main" className={styles.skip}>{t.skip}</a><ReferenceChrome current={{label:t.current,mobileLabel:t.currentShort,searchLabel:t.search,searchButtonLabel:t.searchButton}} search={<CategorySearch query={filters.query} onQuery={query} inputRef={input}/>} onSearch={()=>launch(()=>setOverlay("search"))} onTools={()=>launch(()=>setOverlay("tools"))}/><div className={styles.workspace}><main id="public-main" className={styles.main} tabIndex={-1}>
    <nav className={styles.topics} aria-label={text("Related nature topics")}><Link href="/app/image">✧ {t.createImage}</Link><Link href="/app/models">▹ {t.createVideo}</Link>{t.topics.map(topic=><button type="button" key={topic.id} aria-label={`${t.filterTopic} ${topic.label}`} aria-pressed={filters.topic===topic.id} onClick={()=>{setFilters(current=>({...current,topic:current.topic===topic.id?"all":topic.id}));setPage(1);}}>{topic.label}</button>)}</nav>
    <section className={styles.intro}><div className={styles.heading}><h1>{t.heading}</h1><label className={styles.sort}><span className={styles.srOnly}>{t.sort}</span><select aria-label={t.sort} value={filters.sort} onChange={event=>{const value=event.target.value as MaterialSort;if(natureCategory.sortPolicy.includes(value)){setFilters(current=>({...current,sort:value}));setPage(1);}}}>{t.sorts.map(option=><option key={option.value} value={option.value}>{option.label}</option>)}</select></label></div><p>{t.intro} <Link href="/ai/image-generator">{t.generator}</Link>. <em>{t.introEnd}</em></p></section>
    <MaterialFilterBar values={filters} onChange={changeFilter} layout={layout} onLayout={setLayout} onAllFilters={()=>launch(()=>setOverlay("filters"))}/>
    {banner?<aside className={styles.banner}><strong>{t.banner}</strong><div>{t.bannerLinks.map(link=><Link href={link.href} key={link.href}>{link.label}<span aria-hidden="true">↗</span></Link>)}</div><button type="button" aria-label={t.bannerClose} onClick={()=>{setBanner(false);requestAnimationFrame(()=>resultsRef.current?.focus({preventScroll:true}));}}>×</button></aside>:null}
    <div className={styles.resultSummary} aria-live="polite"><span>{locale==='ru'?`${t.results}: ${found.length}`:`${found.length} ${t.results}`}</span>{active?<button type="button" onClick={reset}>{t.reset} ({active})</button>:null}</div><p role="status" className={styles.notice}>{notice}</p><div ref={resultsRef} className={styles.results} tabIndex={-1}>{found.length?<MaterialGrid items={pagination.items} layout={layout} onView={(index,trigger)=>launch(()=>setPreview(index),trigger)} onNotice={setNotice}/>:<section className={styles.empty}><h2>{t.empty}</h2><p>{t.emptyHint}</p><button type="button" onClick={reset}>{t.reset}</button></section>}</div>
    {found.length?<nav className={styles.pagination} aria-label={t.pagination}><span>{t.showing} {pagination.start}–{pagination.end} {t.of} {found.length}</span><div><button type="button" aria-label={t.previous} disabled={pagination.page===1} onClick={()=>changePage(pagination.page-1)}>‹</button>{Array.from({length:pagination.pages},(_,index)=><button type="button" key={index} aria-label={`${t.page} ${index+1}`} aria-current={pagination.page===index+1?"page":undefined} onClick={()=>changePage(index+1)}>{index+1}</button>)}<button type="button" aria-label={t.next} disabled={pagination.page===pagination.pages} onClick={()=>changePage(pagination.page+1)}>›</button></div></nav>:null}
    <section className={styles.featured}><h2>{text("Featured botanical photo")}</h2><Link href={gypsophilaMaterial.path} aria-label={`${text("View photo details:")} ${gypsophilaMaterial.title}`}><MediaImage src={gypsophilaMaterial.media.src} alt="" width={gypsophilaMaterial.media.width} height={gypsophilaMaterial.media.height} fit="cover" passive loading="lazy"/><span>{gypsophilaMaterial.title} <span aria-hidden="true">↗</span></span></Link></section>
    <section className={styles.related}><h2>{t.relatedTitle}</h2><div>{t.related.map(link=><Link key={link.href} href={link.href}>{link.label}<span aria-hidden="true">↗</span></Link>)}</div></section>
  </main><LandingFooter {...aiToolsEn.footer}/></div></div>
  {overlay==="filters"?<MaterialFilterDialog values={filters} onChange={changeFilter} onApply={()=>setOverlay(null)} onReset={reset} onClose={()=>setOverlay(null)} count={found.length}/>:null}{overlay==="search"?<LandingDialog title={t.searchDialogTitle} closeLabel={t.close} onClose={()=>setOverlay(null)}><div className={styles.searchDialog}><CategorySearch query={filters.query} onQuery={query} inputRef={dialogInput}/><p>{locale==='ru'?`${t.results}: ${found.length}`:`${found.length} ${t.results}`}</p><button type="button" onClick={()=>setOverlay(null)}>{t.showResults}</button></div></LandingDialog>:null}{overlay==="tools"?<ReferenceTools onClose={()=>setOverlay(null)}/>:null}{preview!==null?<CollectionPreview members={pagination.items} index={preview} onSelect={setPreview} onClose={()=>setPreview(null)}/>:null}</>;
}

const publicCopySourcesForLocale={aiToolsEn,t,natureCategory,natureMaterials,gypsophilaMaterial,initialMaterialFilters};
