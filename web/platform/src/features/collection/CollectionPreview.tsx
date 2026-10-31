"use client";
import {useState} from "react";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {MediaPreviewDialogTemplate,MediaPreviewPromptHeader} from "@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate";
import {stageImagePrompt} from "@/features/image-generation/image-prompt-draft";
import {collectionEn as t,type CollectionMember} from "@/i18n/public/collection-en";
import Link from "@/i18n/Link";
import styles from "./Collection.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


function Prompt({member}:{member:CollectionMember}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"collectionEn"});

  const [status,setStatus]=useState<"idle"|"copied"|"error">("idle");
  const copy=async()=>{try{await navigator.clipboard.writeText(member.prompt);setStatus("copied");}catch{setStatus("error");}};
  return <div className={styles.prompt}><h2>{member.alt}</h2><MediaPreviewPromptHeader title={t.prompt} copyLabel={t.copy} copiedLabel={t.copied} isCopied={status==="copied"} onCopy={()=>void copy()}/><p className={styles.promptText}>{member.prompt}</p><p className={styles.promptNote}>{t.promptNote}</p>{status==="error"?<p role="status">{t.copyError}</p>:null}<Link className={styles.promptAction} href="/app/image" onClick={()=>stageImagePrompt(member.prompt)}>{t.usePrompt}<span aria-hidden="true">↗</span></Link></div>;
}
export function CollectionPreview({members,index,onSelect,onClose}:{members:readonly CollectionMember[];index:number;onSelect:(index:number)=>void;onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"collectionEn"});

  return <MediaPreviewDialogTemplate ariaLabel={t.preview} backdropTestId="collection-preview-backdrop" closeLabel={t.close} items={members} selectedIndex={index} onSelect={onSelect} onClose={onClose} previousLabel={t.previous} nextLabel={t.next} testIdPrefix="collection-preview" thumbnailRailLabel={t.thumbnails} getItemKey={item=>item.id} getPreviewDimensions={item=>item} getThumbnailLabel={item=>item.alt} infoPanel={item=><Prompt key={item.id} member={item}/>} renderPreview={(item,classes)=><MediaImage src={item.src} alt={item.alt} width={item.width} height={item.height} fit="contain" className={classes.previewMedia}/>} renderThumbnail={(item,classes)=><MediaImage src={item.src} alt="" loading="lazy" width={item.width} height={item.height} passive fit="cover" className={classes.thumbnailMedia}/>}/>;
}

const publicCopySourcesForLocale={t};
