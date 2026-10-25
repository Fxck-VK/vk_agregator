"use client";
import {useCallback,useId,useRef,useState,useSyncExternalStore} from "react";
import {CopyIcon} from "@/components/icons/CopyIcon";
import {MoreIcon} from "@/components/icons/MoreIcon";
import {PopoverPanel} from "@/components/ui/PopoverPanel/PopoverPanel";
import {deviceBookmarks} from "@/components/public/reference/device-bookmarks";
import {publicExampleEn as defaults,type PublicExampleRecord} from "@/i18n/public/public-example-en";
import {ExampleIcon} from "./ExampleIcon";
import styles from "./PublicExample.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


const defaultBookmarks=deviceBookmarks("epifera.saved-examples");
type ActionLabelKey="download"|"formats"|"formatsTitle"|"information"|"copyImage"|"imageCopied"|"imageCopyError"|"similar"|"save"|"saved"|"savedNotice"|"removedNotice"|"saveError"|"share"|"more"|"moreTitle"|"viewPrompt"|"sourceLink"|"contact";
export type ReferenceActionLabels=Partial<Record<ActionLabelKey,string>>;
export function ExampleActions({example,onInformation,onPrompt,onDiscover,onShare,onNotice,labels,bookmarkStore=defaultBookmarks}:{example:Pick<PublicExampleRecord,"id"|"media"|"downloads"|"provenance">;labels?:ReferenceActionLabels;bookmarkStore?:ReturnType<typeof deviceBookmarks>;onInformation:()=>void;onPrompt:(trigger?:HTMLElement)=>void;onDiscover:()=>void;onShare:()=>void;onNotice:(message:string)=>void}){
  const {defaults} = usePublicCopy(publicCopySourcesForLocale);

  const t=usePublicCopy({...defaults,...labels}),bookmarks=bookmarkStore;
  const [menu,setMenu]=useState<"formats"|"more"|null>(null),[copying,setCopying]=useState(false),formats=useRef<HTMLButtonElement>(null),more=useRef<HTMLButtonElement>(null),id=useId();
  const saved=useSyncExternalStore(bookmarks.subscribe,()=>bookmarks.isSaved(example.id),()=>false),close=useCallback(()=>setMenu(null),[]);
  const copyImage=async()=>{if(copying)return;setCopying(true);try{if(!navigator.clipboard?.write||typeof ClipboardItem==="undefined")throw Error("Clipboard unavailable");const png=example.downloads.find(item=>item.mime==="image/png");if(!png)throw Error("Preview unavailable");const response=await fetch(png.src);if(!response.ok)throw Error("Preview unavailable");const blob=await response.blob();if(blob.type!=="image/png")throw Error("Invalid preview format");await navigator.clipboard.write([new ClipboardItem({"image/png":blob})]);onNotice(t.imageCopied);}catch{onNotice(t.imageCopyError);}finally{setCopying(false);}};
  return <div className={styles.panelBody}><div className={styles.downloadGroup}><a className={styles.download} href={example.downloads[0].src} download={example.downloads[0].filename} aria-label={t.download}><ExampleIcon kind="download"/>{t.download}</a><button type="button" ref={formats} className={styles.formatTrigger} aria-label={t.formats} aria-haspopup="dialog" aria-expanded={menu==="formats"} aria-controls={menu==="formats"?`${id}-formats`:undefined} onClick={()=>setMenu(value=>value==="formats"?null:"formats")}>⌄</button></div>
    <div className={styles.actionRow}><button type="button" aria-label={t.information} title={t.information} onClick={onInformation}><ExampleIcon kind="information"/></button><button type="button" aria-label={t.copyImage} title={t.copyImage} disabled={copying} aria-busy={copying} onClick={()=>void copyImage()}><CopyIcon/></button><button type="button" aria-label={t.similar} title={t.similar} onClick={onDiscover}><ExampleIcon kind="similar"/></button><button type="button" aria-label={saved?t.saved:t.save} title={saved?t.saved:t.save} aria-pressed={saved} onClick={()=>{const result=bookmarks.toggle(example.id);onNotice(result.ok?(result.saved?t.savedNotice:t.removedNotice):t.saveError);}}><ExampleIcon kind="save"/></button><button type="button" aria-label={t.share} title={t.share} onClick={onShare}><ExampleIcon kind="share"/></button><button type="button" ref={more} aria-label={t.more} title={t.more} aria-haspopup="dialog" aria-expanded={menu==="more"} aria-controls={menu==="more"?`${id}-more`:undefined} onClick={()=>setMenu(value=>value==="more"?null:"more")}><MoreIcon/></button></div>
    <button type="button" className={styles.promptTrigger} onClick={()=>onPrompt()}>{t.viewPrompt} <span aria-hidden="true">↗</span></button>
    <PopoverPanel anchorRef={formats} id={`${id}-formats`} isOpen={menu==="formats"} onClose={close} label={t.formatsTitle} width={280} align="end" itemVariant="action"><div className={styles.optionList}>{example.downloads.map(format=><a key={format.src} href={format.src} download={format.filename} aria-label={format.label} onClick={close}><span>{format.label}</span><small>{example.media.width} × {example.media.height}</small></a>)}</div></PopoverPanel>
    <PopoverPanel anchorRef={more} id={`${id}-more`} isOpen={menu==="more"} onClose={close} label={t.moreTitle} width={260} align="end" itemVariant="action"><div className={styles.optionList}><button type="button" onClick={()=>{onPrompt(more.current??undefined);close();}}>{t.viewPrompt}</button><a href={example.provenance.url} target="_blank" rel="noreferrer">{t.sourceLink} ↗</a><a href="mailto:support@epifera.com">{t.contact}</a></div></PopoverPanel>
  </div>;
}

const publicCopySourcesForLocale={defaults};
