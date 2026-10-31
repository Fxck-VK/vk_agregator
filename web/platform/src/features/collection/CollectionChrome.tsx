"use client";
import type {RefObject} from "react";
import {ReferenceChrome,ReferenceTools} from "@/components/public/reference/ReferenceChrome";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {collectionEn as t} from "@/i18n/public/collection-en";
import styles from "./Collection.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function CollectionSearchField({query,onQuery,inputRef}:{query:string;onQuery:(query:string)=>void;inputRef?:RefObject<HTMLInputElement|null>}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"collectionEn"});

  return <form className={styles.search} role="search" onSubmit={event=>event.preventDefault()}><HelpIcon kind="search"/><input ref={inputRef} type="search" aria-label={t.search} placeholder={t.searchPlaceholder} value={query} onChange={event=>onQuery(event.target.value)}/>{query?<button type="button" aria-label={t.clear} onClick={()=>{onQuery("");inputRef?.current?.focus();}}>×</button>:null}</form>;
}
export function CollectionChrome({query,onQuery,onSearch,onTools,inputRef}:{query:string;onQuery:(query:string)=>void;onSearch:()=>void;onTools:()=>void;inputRef:RefObject<HTMLInputElement|null>}){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"collectionEn"});

  return <ReferenceChrome search={<CollectionSearchField query={query} onQuery={onQuery} inputRef={inputRef}/>} current={{label:t.current,mobileLabel:t.collection,searchLabel:t.searchExamples,searchButtonLabel:t.openSearch}} onSearch={onSearch} onTools={onTools}/>;
}
export const CollectionTools=ReferenceTools;

const publicCopySourcesForLocale={t};
