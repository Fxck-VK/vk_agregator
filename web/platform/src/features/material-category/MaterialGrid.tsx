"use client";
import {useLayoutEffect,useRef,useState,useSyncExternalStore} from "react";
import {CollectionCard} from "@/features/collection/CollectionCard";
import {AssetIcon} from "@/components/icons/AssetIcon/AssetIcon";
import {deviceBookmarks} from "@/components/public/reference/device-bookmarks";
import {brandFilePrefix} from "@/config/brand";
import {materialCategoryEn as t,type MaterialRecord} from "@/i18n/public/material-category-en";
import {packMaterials,type MaterialLayout} from "./category-model";
import styles from "./MaterialCategory.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";


const bookmarks=deviceBookmarks("epifera.saved-materials");
function MaterialCardActions({item,onNotice}:{item:MaterialRecord;onNotice:(message:string)=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  const saved=useSyncExternalStore(bookmarks.subscribe,()=>bookmarks.isSaved(item.id),()=>false);
  return <div className={styles.cardActions}><button type="button" aria-label={`${saved?t.saved:t.save} ${item.alt}`} aria-pressed={saved} onClick={()=>{const result=bookmarks.toggle(item.id);onNotice(result.ok?(result.saved?t.savedNotice:t.removedNotice):t.saveError);}}><svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round"><path d="M6 3h12v18l-6-4-6 4Z"/></svg></button><a href={item.src} download={`${brandFilePrefix}-${item.id}.webp`} aria-label={`${t.download} ${item.alt}`}><AssetIcon iconName="download" source="/assets/icons/ui/download-white.svg"/></a></div>;
}
export function MaterialGrid({items,layout,onView,onNotice}:{items:readonly MaterialRecord[];layout:MaterialLayout;onView:(index:number,trigger?:HTMLElement)=>void;onNotice:(message:string)=>void}){
  const locale=useLocale();
  const root=useRef<HTMLDivElement>(null),[size,setSize]=useState({width:0,viewport:1280});
  useLayoutEffect(()=>{const element=root.current;if(!element)return;const update=()=>{const width=element.clientWidth,viewport=window.innerWidth;setSize(current=>current.width===width&&current.viewport===viewport?current:{width,viewport});};update();const observer=typeof ResizeObserver==="undefined"?null:new ResizeObserver(update);observer?.observe(element);window.addEventListener("resize",update);return()=>{observer?.disconnect();window.removeEventListener("resize",update);};},[]);
  const packed=packMaterials(items,size.width,size.viewport,layout,layout==="square"?16:24),ready=size.width>0;
  return <div ref={root} className={styles.gridFrame}><ol className={styles.grid} data-material-grid data-layout={layout} data-ready={ready} data-columns={packed.columns} style={ready?{height:packed.height}:undefined} aria-label={translatePublicText("Nature materials",locale)}>{items.map((item,index)=>{const card=packed.cards[index];return <li key={item.id} data-material-id={item.id} style={ready?{position:"absolute",left:card.x,top:card.y,width:card.width,height:card.height}:{aspectRatio:layout==="square"?1:item.width/item.height}}><CollectionCard member={item} priority={index<3} className={styles.categoryCard} onView={trigger=>onView(index,trigger)} actions={<MaterialCardActions item={item} onNotice={onNotice}/>}/></li>;})}</ol></div>;
}

const publicCopySourcesForLocale={t};
