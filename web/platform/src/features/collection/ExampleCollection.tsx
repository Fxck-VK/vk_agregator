"use client";
import {useEffect,useLayoutEffect,useRef,useState,useSyncExternalStore,type CSSProperties} from "react";
import {CollectionCard} from "./CollectionCard";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {collectionEn as t,type ExampleCollection as CollectionData,type CollectionMember} from "@/i18n/public/collection-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import Link from "@/i18n/Link";
import {CollectionChrome,CollectionSearchField,CollectionTools} from "./CollectionChrome";
import {CollectionPreview} from "./CollectionPreview";
import {isCollectionSaved,subscribeCollections,toggleCollectionSaved} from "./collection-bookmarks";
import styles from "./Collection.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {publicTagLabel} from "./tag-labels";


function CollectionActionIcon({share=false}:{share?:boolean}){return <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">{share?<><circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="m9 10 6-4M9 14l6 4"/></>:<><path d="M3 7V5h7l2 2h9v6M3 7v13h10M3 10h18M16 17l2 2 4-4"/></>}</svg>;}
function rowRatio(members:readonly CollectionMember[],index:number,columns:number){const start=Math.floor(index/columns)*columns;return Math.min(...members.slice(start,start+columns).map(item=>item.width/item.height));}
export function ExampleCollection({collection:sourceCollection}:{collection:CollectionData}){
  const {t,aiToolsEn,canonicalPath} = usePublicCopy(publicCopySourcesForLocale,{"canonicalPath":"__route.path","t":"collectionEn"});
  const locale=useLocale(),collection=usePublicCopy(sourceCollection);

  const [query,setQuery]=useState(""),[overlay,setOverlay]=useState<"search"|"tools"|null>(null),[selected,setSelected]=useState<number|null>(null),[notice,setNotice]=useState(""),[shareLink,setShareLink]=useState("");
  const root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),input=useRef<HTMLInputElement>(null),dialogInput=useRef<HTMLInputElement>(null),open=overlay!==null||selected!==null;
  const saved=useSyncExternalStore(subscribeCollections,()=>isCollectionSaved(collection.id),()=>false);
  const words=query.trim().toLowerCase().split(/\s+/).filter(Boolean),members=collection.members.filter(item=>words.every(word=>`${item.alt} ${item.tags.map(tag=>publicTagLabel(tag,locale)).join(" ")} ${item.prompt}`.toLowerCase().includes(word)));
  const launch=(action:()=>void,trigger?:HTMLElement)=>{opener.current=trigger??(document.activeElement instanceof HTMLElement?document.activeElement:null);action();};
  useLayoutEffect(()=>{const page=root.current;if(!page)return;const before=page.inert;page.inert=open;return()=>{page.inert=before;};},[open]);
  useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
  const save=()=>{const result=toggleCollectionSaved(collection.id);setNotice(result.ok?(result.saved?t.savedNotice:t.removedNotice):t.saveError);};
  const share=async()=>{const url=`${window.location.origin}/${locale}${canonicalPath}`;try{await navigator.clipboard.writeText(url);setNotice(t.shared);setShareLink("");}catch{setNotice(t.shareError);setShareLink(url);}};
  return <><div ref={root} className={styles.page} data-collection-page><a href="#public-main" className={styles.skip}>{t.skip}</a><CollectionChrome query={query} onQuery={setQuery} inputRef={input} onSearch={()=>launch(()=>setOverlay("search"))} onTools={()=>launch(()=>setOverlay("tools"))}/><div className={styles.workspace}><main id="public-main" className={styles.main} tabIndex={-1}><Link href="/app/inspiration" className={styles.back}>‹ <span>{t.back}</span></Link><section className={styles.intro} aria-labelledby="collection-title"><h1 id="collection-title">{collection.title}</h1><p>{collection.description}</p><span className={styles.count}><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"><rect x="6" y="4" width="14" height="14" rx="2"/><path d="M3 8v12h13M8 13l3-3 3 3 2-2 2 2"/></svg>{collection.members.length} {t.examples}</span><span>{t.by} <strong>{collection.curator}</strong></span><div className={styles.actions}><button type="button" aria-label={saved?t.saved:t.save} aria-pressed={saved} onClick={save}><CollectionActionIcon/><span>{saved?t.saved:t.save}</span></button><button type="button" aria-label={t.share} onClick={()=>void share()}><CollectionActionIcon share/><span>{t.shareShort}</span></button></div><div className={styles.notice} role="status">{notice}</div>{shareLink?<input className={styles.shareLink} aria-label={t.link} readOnly value={shareLink} onFocus={event=>event.target.select()}/>:null}</section>
    {query?<div className={styles.results} aria-live="polite"><p>{members.length} {t.results}</p><button type="button" onClick={()=>{setQuery("");input.current?.focus();}}>{t.all}</button></div>:null}
    {members.length?<ol className={styles.grid} aria-label={t.gallery}>{members.map((item,index)=><li key={item.id} data-collection-member={item.id} style={{"--image-ratio":item.width/item.height,"--row-two":rowRatio(members,index,2),"--row-three":rowRatio(members,index,3),"--row-four":rowRatio(members,index,4)} as CSSProperties}><CollectionCard member={item} priority={index<3} onView={trigger=>launch(()=>setSelected(index),trigger)}/></li>)}</ol>:<div className={styles.empty}><h2>{t.empty}</h2><p>{t.emptyHint}</p></div>}
    <section className={styles.context}><h2>{t.contextTitle}</h2><p>{collection.curatorNote}</p><h3>{t.relatedTitle}</h3><div>{t.related.map(item=><Link href={item.href} key={item.href}>{item.label} <span aria-hidden="true">↗</span></Link>)}</div><small>{t.source}</small></section>
  </main><LandingFooter {...aiToolsEn.footer}/></div></div>
  {selected!==null?<CollectionPreview members={members} index={selected} onSelect={setSelected} onClose={()=>setSelected(null)}/>:null}
  {overlay==="search"?<LandingDialog title={t.search} closeLabel={t.close} onClose={()=>setOverlay(null)}><div className={styles.searchDialog}><CollectionSearchField query={query} onQuery={setQuery} inputRef={dialogInput}/><p aria-live="polite">{members.length} {t.results}</p><button type="button" onClick={()=>setOverlay(null)}>{t.showResults}</button></div></LandingDialog>:null}
  {overlay==="tools"?<CollectionTools onClose={()=>setOverlay(null)}/>:null}</>;
}

const publicCopySourcesForLocale={canonicalPath:"/inspiration/cinematic-scenes",t,aiToolsEn};
