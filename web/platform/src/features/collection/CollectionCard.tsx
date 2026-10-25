"use client";
import {useCallback,useId,useRef,useState,type ReactNode} from "react";
import {MediaImage} from "@/components/media/MediaImage/MediaImage";
import {PopoverPanel} from "@/components/ui/PopoverPanel/PopoverPanel";
import {stageImagePrompt} from "@/features/image-generation/image-prompt-draft";
import {collectionEn as t,type CollectionMember} from "@/i18n/public/collection-en";
import Link from "@/i18n/Link";
import styles from "./Collection.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function CollectionCard({member,priority,onView,className,actions}:{member:CollectionMember;priority:boolean;onView:(opener?:HTMLElement)=>void;className?:string;actions?:ReactNode}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  const [menu,setMenu]=useState(false),anchor=useRef<HTMLButtonElement>(null),id=useId();
  const close=useCallback(()=>setMenu(false),[]);
  return <figure className={[styles.card,className].filter(Boolean).join(" ")}><button type="button" aria-label={`${t.view} ${member.alt}`} className={styles.view} onClick={()=>onView()}><MediaImage src={member.src} alt={member.alt} width={member.width} height={member.height} fit="cover" loading={priority?"eager":"lazy"} passive showLoading={false}/><span className={styles.cardShade}/></button><button ref={anchor} type="button" className={styles.use} aria-label={`${t.use} ${member.alt}`} aria-haspopup="dialog" aria-expanded={menu} aria-controls={menu?id:undefined} data-open={menu||undefined} onClick={()=>setMenu(value=>!value)}>{t.use} <span aria-hidden="true">⌄</span></button><PopoverPanel id={id} anchorRef={anchor} align="end" width={250} isOpen={menu} onClose={close} label={t.exampleActions} itemVariant="action"><div className={styles.cardMenu}><button type="button" onClick={()=>{onView(anchor.current??undefined);close();}}>{t.viewPrompt}</button><Link href="/app/image" onClick={()=>{stageImagePrompt(member.prompt);close();}}>{t.usePrompt}<span aria-hidden="true">↗</span></Link></div></PopoverPanel>{actions}</figure>;
}

const publicCopySourcesForLocale={t};
