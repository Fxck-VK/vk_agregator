"use client";
import {useRef,useState} from 'react';
import {LandingDialog} from '@/components/public/landing/LandingDialog';
import {InputSurface} from '@/components/ui/InputSurface/InputSurface';
import {ScrollArea} from '@/components/ui/ScrollArea/ScrollArea';
import {authorEn as t,type AuthorMaterial} from '@/i18n/public/author-en';
import Link from '@/i18n/Link';
import {searchAuthorMaterials} from './author-model';
import styles from './Author.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale,useMessages} from '@/i18n/LocaleProvider';
import {countLabel} from '@/i18n/counts';


export function AuthorSearch({items,onClose}:{items:readonly AuthorMaterial[];onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale();
  const messages=useMessages();

  const[query,setQuery]=useState('');
  const input=useRef<HTMLInputElement>(null);
  const found=searchAuthorMaterials(items,query,locale);
  return <LandingDialog title={t.searchTitle} closeLabel={t.close} onClose={onClose}>
    <InputSurface className={styles.searchSurface}>
      <input ref={input} type="search" aria-label={t.searchPlaceholder} placeholder={t.searchPlaceholder} value={query} onChange={event=>setQuery(event.target.value)} autoComplete="off" maxLength={200}/>
      {query?<button type="button" aria-label={t.clear} onClick={()=>{setQuery('');input.current?.focus();}}>×</button>:null}
    </InputSurface>
    <p className={styles.searchCount} role="status">{countLabel(messages,'materials',found.length)}</p>
    {found.length?<ScrollArea className={styles.searchResults} viewportClassName={styles.searchViewport} viewportProps={{'aria-label':t.searchTitle}}>
      {found.map(item=><Link key={item.id} href={item.href} onClick={onClose}><span>{item.category}</span>{' '}<strong>{item.title}</strong></Link>)}
    </ScrollArea>:<p className={styles.noResults}>{t.noResults}</p>}
  </LandingDialog>;
}

const publicCopySourcesForLocale={t};
