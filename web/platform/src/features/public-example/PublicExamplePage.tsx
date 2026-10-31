"use client";
import {useEffect,useLayoutEffect,useRef,useState} from "react";
import {BrandLogo} from "@/components/brand/BrandLogo/BrandLogo";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {ReferenceChrome,ReferenceTools} from "@/components/public/reference/ReferenceChrome";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {HomeSearch} from "@/features/public-home/HomeSearch";
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {publicExampleEn as t,type PublicExampleRecord,type PublicExampleImage} from "@/i18n/public/public-example-en";
import {ExampleArtwork} from "./ExampleArtwork";
import {ExampleActions} from "./ExampleActions";
import {ExampleInformation,ExamplePrompt,ExampleViewer} from "./ExampleDialogs";
import {ExampleRelated} from "./ExampleRelated";
import styles from "./PublicExample.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


type Viewer={items:readonly PublicExampleImage[];index:number};
export function PublicExamplePage({example:sourceExample}:{example:PublicExampleRecord}){
  const {t,aiToolsEn,homeEn,canonicalPath} = usePublicCopy(publicCopySourcesForLocale,{"canonicalPath":"__route.path","t":"publicExampleEn"});
  const locale=useLocale(),example=usePublicCopy(sourceExample);

  const [dialog,setDialog]=useState<"information"|"prompt"|"search"|"tools"|null>(null),[viewer,setViewer]=useState<Viewer|null>(null),[notice,setNotice]=useState(""),[shareLink,setShareLink]=useState(""),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),related=useRef<HTMLElement|null>(null),open=dialog!==null||viewer!==null;
  const launch=(action:()=>void,trigger?:HTMLElement)=>{opener.current=trigger??(document.activeElement instanceof HTMLElement?document.activeElement:null);action();};
  useLayoutEffect(()=>{const page=root.current;if(!page)return;const before=page.inert;page.inert=open;return()=>{page.inert=before;};},[open]);
  useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
  const view=(items:readonly PublicExampleImage[],index:number)=>launch(()=>setViewer({items,index}));
  const share=async()=>{const url=`${window.location.origin}/${locale}${canonicalPath}`;try{await navigator.clipboard.writeText(url);setNotice(t.shared);setShareLink("");}catch{setNotice(t.shareError);setShareLink(url);}};
  return <><div className={styles.page} data-public-example ref={root}><a className={styles.skip} href="#public-main">{t.skip}</a><ReferenceChrome current={{label:t.current,mobileLabel:t.currentShort,searchLabel:t.search,searchButtonLabel:t.searchButton}} onSearch={()=>launch(()=>setDialog("search"))} onTools={()=>launch(()=>setDialog("tools"))} search={<button type="button" className={styles.globalSearch} onClick={()=>launch(()=>setDialog("search"))}><HelpIcon kind="search"/><span>{t.searchPlaceholder}</span><HelpIcon kind="grid"/></button>}/><div className={styles.workspace}><main id="public-main" className={styles.main}><div className={styles.detail}><ExampleArtwork example={example} onEnlarge={()=>view([example.media,...example.series,...example.similar],0)}/><aside className={styles.panel}><div className={styles.identity}><BrandLogo size={24}/><strong>{t.curator}</strong><span>{t.curatorNote}</span></div><h1>{example.title}</h1><ExampleActions example={example} onInformation={()=>launch(()=>setDialog("information"))} onPrompt={trigger=>launch(()=>setDialog("prompt"),trigger)} onDiscover={()=>{related.current?.scrollIntoView({behavior:window.matchMedia?.("(prefers-reduced-motion: reduce)").matches?"instant":"smooth",block:"start"});related.current?.focus({preventScroll:true});}} onShare={()=>void share()} onNotice={setNotice}/><p role="status" className={styles.notice}>{notice}</p>{shareLink?<input className={styles.shareLink} readOnly aria-label={t.link} value={shareLink} onFocus={event=>event.target.select()}/>:null}</aside></div><ExampleRelated example={example} sectionRef={related} onView={view}/></main><LandingFooter {...aiToolsEn.footer}/></div></div>
    {dialog==="information"?<ExampleInformation example={example} onClose={()=>setDialog(null)}/>:null}{dialog==="prompt"?<ExamplePrompt example={example} onClose={()=>setDialog(null)}/>:null}{dialog==="search"?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:null}{dialog==="tools"?<ReferenceTools onClose={()=>setDialog(null)}/>:null}{viewer?<ExampleViewer items={viewer.items} index={viewer.index} onSelect={index=>setViewer(current=>current?{...current,index}:null)} onClose={()=>setViewer(null)}/>:null}</>;
}

const publicCopySourcesForLocale={canonicalPath:"/inspiration/examples/aesthetic-flowers",homeEn,aiToolsEn,t};
