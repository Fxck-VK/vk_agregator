"use client";
import {useState} from "react";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {MediaPreviewDialogTemplate,MediaPreviewPromptHeader} from "@/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate";
import {stageImagePrompt} from "@/features/image-generation/image-prompt-draft";
import {publicExampleEn as t,type PublicExampleRecord,type PublicExampleImage} from "@/i18n/public/public-example-en";
import Link from "@/i18n/Link";
import styles from "./PublicExample.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";
import {translatePublicText} from "@/i18n/public/public-copy";


export function ExampleInformation({example,onClose}:{example:PublicExampleRecord;onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  return <LandingDialog title={t.about} closeLabel={t.close} onClose={onClose}><div className={styles.information}><dl><div><dt>{t.source}</dt><dd>{example.provenance.source}</dd></div><div><dt>{t.dimensions}</dt><dd>{example.media.width} × {example.media.height}</dd></div><div><dt>{t.model}</dt><dd>{example.modelVersionId??t.unknownModel}</dd></div><div><dt>{t.originalPrompt}</dt><dd>{t.unknownPrompt}</dd></div></dl><h3>{t.reproduction}</h3><p>{example.reproductionNotes}</p><a href={example.provenance.url} aria-label={t.sourceLink} target="_blank" rel="noreferrer">{t.sourceLink} ↗</a></div></LandingDialog>;
}
export function ExamplePrompt({example,onClose}:{example:PublicExampleRecord;onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  const [state,setState]=useState<"idle"|"copied"|"error">("idle");
  const copy=async()=>{try{await navigator.clipboard.writeText(example.suggestedPrompt);setState("copied");}catch{setState("error");}};
  return <LandingDialog title={t.promptTitle} closeLabel={t.close} onClose={onClose}><div className={styles.information}><MediaPreviewPromptHeader title={example.title} copyLabel={t.copyPrompt} copiedLabel={t.copied} isCopied={state==="copied"} onCopy={()=>void copy()}/><p className={styles.promptText}>{example.suggestedPrompt}</p><p>{t.promptNote}</p>{state==="error"?<p role="status">{t.promptError}</p>:null}<Link href="/app/image" className={styles.promptLink} onClick={()=>stageImagePrompt(example.suggestedPrompt)}>{t.usePrompt} ↗</Link></div></LandingDialog>;
}
export function ExampleViewer({items,index,onSelect,onClose}:{items:readonly PublicExampleImage[];index:number;onSelect:(index:number)=>void;onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale(),imageLabel=(direction:string)=>translatePublicText("{direction} image",locale).replace("{direction}",direction);

  return <MediaPreviewDialogTemplate ariaLabel={t.viewer} backdropTestId="public-example-preview-backdrop" closeLabel={t.close} items={items} selectedIndex={index} onSelect={onSelect} onClose={onClose} previousLabel={imageLabel(t.previous)} nextLabel={imageLabel(t.next)} thumbnailRailLabel={t.thumbnails} testIdPrefix="public-example-preview" getItemKey={item=>item.id} getPreviewDimensions={item=>item} getThumbnailLabel={item=>item.alt} renderPreview={(item,classes)=><MediaImage src={item.src} alt={item.alt} width={item.width} height={item.height} fit="contain" className={classes.previewMedia}/>} renderThumbnail={(item,classes)=><MediaImage src={item.src} alt="" width={item.width} height={item.height} fit="cover" passive loading="lazy" className={classes.thumbnailMedia}/>}/>;
}

const publicCopySourcesForLocale={t};
