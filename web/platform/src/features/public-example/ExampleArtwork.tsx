"use client";
import {useId,useRef,useState,type CSSProperties,type ReactNode} from "react";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {stageImagePrompt} from "@/features/image-generation/image-prompt-draft";
import {publicExampleEn as t,type PublicExampleRecord} from "@/i18n/public/public-example-en";
import {useRouter} from "@/i18n/navigation";
import Link from "@/i18n/Link";
import {ExampleIcon} from "./ExampleIcon";
import styles from "./PublicExample.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function ExampleArtwork({example,onEnlarge,showBadge=true,presets:sourcePresets,originNote}:{example:Pick<PublicExampleRecord,"media"|"suggestedPrompt">;onEnlarge:()=>void;showBadge?:boolean;presets?:readonly {label:string;instruction:string}[];originNote?:ReactNode}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
  const presets=usePublicCopy(sourcePresets??t.presets);

  const [instruction,setInstruction]=useState(""),[visible,setVisible]=useState(true),input=useRef<HTMLInputElement>(null),router=useRouter(),id=useId();
  return <div className={styles.visual}><div className={styles.stage} data-reference-stage><div className={styles.artwork} data-reference-artwork style={{"--artwork-ratio":example.media.width/example.media.height} as CSSProperties}><button type="button" className={styles.enlarge} aria-label={t.enlarge} onClick={onEnlarge}><MediaImage src={example.media.src} alt={example.media.alt} width={example.media.width} height={example.media.height} fit="contain" loading="eager" showLoading={false}/><span className={styles.enlargeIcon}><ExampleIcon kind="expand"/></span></button>{showBadge?<span className={styles.badge}><ExampleIcon kind="spark"/>{t.badge}</span>:null}</div>
    {visible?<div className={styles.editOverlay} data-reference-editor><div className={styles.presets}>{presets.map(preset=><button type="button" key={preset.label} onClick={()=>{setInstruction(preset.instruction);input.current?.focus();}}>{preset.label}</button>)}</div><div className={styles.editRow}><form aria-label={t.variation} className={styles.editForm} onSubmit={event=>{event.preventDefault();stageImagePrompt([example.suggestedPrompt,instruction.trim()].filter(Boolean).join("\n\n"));router.push("/app/image");}}><label htmlFor={id}><ExampleIcon kind="spark"/><span>{t.variation}</span></label><input ref={input} id={id} type="text" aria-label={t.variationLabel} placeholder={t.variationPlaceholder} value={instruction} maxLength={2000} onChange={event=>setInstruction(event.target.value)}/><button type="submit" aria-label={t.variationSubmit}>↑</button></form><button type="button" className={styles.hideEdit} aria-label={t.hideVariation} onClick={()=>setVisible(false)}>×</button></div></div>:<button type="button" className={styles.reopenEdit} aria-label={t.variation} onClick={()=>{setVisible(true);requestAnimationFrame(()=>input.current?.focus());}}><ExampleIcon kind="spark"/>{t.variation}</button>}
  </div><p className={styles.originNote}>{originNote??<>{t.originNote} <Link href="/ai/image-generator">{t.generator}</Link>.</>}</p></div>;
}

const publicCopySourcesForLocale={t};
