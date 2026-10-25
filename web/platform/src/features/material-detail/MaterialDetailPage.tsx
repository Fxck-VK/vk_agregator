"use client";
import {useEffect,useLayoutEffect,useRef,useState} from "react";
import {ReferenceChrome,ReferenceTools} from "@/components/public/reference/ReferenceChrome";
import {deviceBookmarks} from "@/components/public/reference/device-bookmarks";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {HomeSearch} from "@/features/public-home/HomeSearch";
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {publicExampleEn as common,type PublicExampleImage} from "@/i18n/public/public-example-en";
import {materialDetailEn as t,type StockItem} from "@/i18n/public/material-detail-en";
import {ExampleArtwork} from "@/features/public-example/ExampleArtwork";
import {ExampleActions} from "@/features/public-example/ExampleActions";
import {ExampleViewer} from "@/features/public-example/ExampleDialogs";
import {RelatedRow} from "@/features/public-example/ExampleRelated";
import Link from "@/i18n/Link";
import shared from "@/features/public-example/PublicExample.module.css";
import styles from "./MaterialDetail.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";
import {publicTagLabel} from "@/features/collection/tag-labels";


const bookmarks=deviceBookmarks("epifera.saved-materials");
type Viewer={items:readonly PublicExampleImage[];index:number};
export function MaterialDetailPage({material:sourceMaterial}:{material:StockItem}){
  const {t,common,aiToolsEn,homeEn} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale(),material=usePublicCopy(sourceMaterial),text=(source:string)=>translatePublicText(source,locale);

 const[dialog,setDialog]=useState<"information"|"search"|"tools"|null>(null),[viewer,setViewer]=useState<Viewer|null>(null),[notice,setNotice]=useState(""),[shareLink,setShareLink]=useState(""),[tag,setTag]=useState<string|null>(null);
 const root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),related=useRef<HTMLElement|null>(null),open=dialog!==null||viewer!==null;
 const launch=(action:()=>void,trigger?:HTMLElement)=>{opener.current=trigger??(document.activeElement instanceof HTMLElement?document.activeElement:null);action();};
 useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
 useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
 const view=(items:readonly PublicExampleImage[],index:number)=>launch(()=>setViewer({items,index}));
 const share=async()=>{const url=`${window.location.origin}/${locale}${material.path}`;try{await navigator.clipboard.writeText(url);setNotice(text("Photo link copied."));setShareLink("");}catch{setNotice(text("Could not copy the link. Copy it below."));setShareLink(url);}};
 const photos=material.photos.filter(item=>tag===null||item.tags.includes(tag));
 return <><div ref={root} className={`${shared.page} ${styles.page}`} data-material-detail><a href="#public-main" className={shared.skip}>{t.skip}</a><ReferenceChrome current={{label:t.current,mobileLabel:t.currentShort,searchLabel:common.search,searchButtonLabel:common.searchButton}} onSearch={()=>launch(()=>setDialog("search"))} onTools={()=>launch(()=>setDialog("tools"))} search={<button type="button" className={shared.globalSearch} onClick={()=>launch(()=>setDialog("search"))}><HelpIcon kind="search"/><span>{common.searchPlaceholder}</span><HelpIcon kind="grid"/></button>}/><div className={shared.workspace}><main id="public-main" className={shared.main}><div className={shared.detail}>
 <div className={styles.visualWrap}><ExampleArtwork example={material} showBadge={false} presets={t.presets} onEnlarge={()=>view([material.media,...material.photos],0)} originNote={<>{t.originNote} <Link href="/ai/image-generator">{t.generator}</Link>.</>}/><p className={styles.hint}>{t.hint} <span>{t.hintDetail}</span></p></div>
 <aside className={shared.panel} data-material-detail-panel><a className={styles.creator} href={material.creator.url} aria-label={`${material.creator.name} — ${text("source creator")}`} target="_blank" rel="noreferrer"><span aria-hidden="true">▣</span><strong>{material.creator.name}</strong></a><h1>{material.title}</h1><ExampleActions example={material} bookmarkStore={bookmarks} labels={{save:"Save photo",saved:"Saved photo",removedNotice:"Removed from saved photos on this device.",share:"Share photo",viewPrompt:t.license,moreTitle:"More photo actions"}} onInformation={()=>launch(()=>setDialog("information"))} onPrompt={trigger=>launch(()=>setDialog("information"),trigger)} onDiscover={()=>{related.current?.scrollIntoView({block:"start",behavior:window.matchMedia?.("(prefers-reduced-motion: reduce)").matches?"instant":"smooth"});related.current?.focus({preventScroll:true});}} onShare={()=>void share()} onNotice={setNotice}/><p role="status" className={shared.notice}>{notice}</p>{shareLink?<input className={shared.shareLink} aria-label={text("Photo link")} readOnly value={shareLink} onFocus={event=>event.target.select()}/>:null}</aside></div>
 <section ref={related} tabIndex={-1} className={shared.related} aria-label={t.photos}><RelatedRow label={t.photos} items={photos} kind="similar" onView={view}/><div className={shared.tags} role="group" aria-label={t.filter}>{material.taxonIds.map(value=><button type="button" key={value} aria-label={`${common.filterPrefix} ${publicTagLabel(value,locale)}`} aria-pressed={tag===value} onClick={()=>setTag(current=>current===value?null:value)}>{publicTagLabel(value,locale)}</button>)}</div>{tag?<div className={shared.filtered}><span aria-live="polite">{photos.length} {text("matching photos")}</span><button type="button" onClick={()=>setTag(null)}>{t.all}</button></div>:null}<RelatedRow label={t.psd} items={material.designs.slice(0,1)} kind="designs" onView={view}/><RelatedRow label={t.designs} items={material.designs.slice(1)} kind="designs" onView={view}/><p className={styles.hint}>{t.designNote}</p><RelatedRow label={t.videos} items={material.videoStills} kind="stills" onView={view}/><p className={styles.hint}>{t.videoNote}</p><Link className={styles.category} href="/photos/nature">{t.category}<span aria-hidden="true"> ↗</span></Link></section>
 </main><LandingFooter {...aiToolsEn.footer}/></div></div>
 {dialog==="information"?<MaterialInformation material={material} onClose={()=>setDialog(null)}/>:null}{dialog==="search"?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:null}{dialog==="tools"?<ReferenceTools onClose={()=>setDialog(null)}/>:null}{viewer?<ExampleViewer items={viewer.items} index={viewer.index} onSelect={index=>setViewer(current=>current?{...current,index}:null)} onClose={()=>setViewer(null)}/>:null}</>;
}
function MaterialInformation({material,onClose}:{material:StockItem;onClose:()=>void}){
  const {t,common} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale(),text=(source:string)=>translatePublicText(source,locale);

 return <LandingDialog title={t.about} closeLabel={common.close} onClose={onClose}><div className={shared.information}><dl><div><dt>{t.source}</dt><dd>{material.creator.name}</dd></div><div><dt>{t.dimensions}</dt><dd>{material.media.width} × {material.media.height}</dd></div><div><dt>{t.format}</dt><dd>{"WebP"} · {(material.fileDescriptor.bytes/1024).toFixed(0)} {text("KB")}</dd></div><div><dt>{text("Source type")}</dt><dd>{text("Photograph (source metadata)")}</dd></div></dl><p>{t.original}</p><h3>{t.license}</h3><p>{material.rights.note}</p><p><a href={material.licenseRef} target="_blank" rel="noreferrer">{text("Check source license")}</a></p><a href={material.provenance.url} target="_blank" rel="noreferrer">{common.sourceLink} ↗</a></div></LandingDialog>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,common,t};
