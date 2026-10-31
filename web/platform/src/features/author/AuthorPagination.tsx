"use client";
import {authorEn as t,authorPath,type AuthorMaterial} from '@/i18n/public/author-en';
import Link from '@/i18n/Link';
import {authorPageWindow,paginateAuthorMaterials} from './author-model';
import styles from './Author.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function AuthorPagination({items,page}:{items:readonly AuthorMaterial[];page:number}){
  const {authorPath,t} = usePublicCopy(publicCopySourcesForLocale,{"t":"authorEn","authorPath":"authorPath"});

  const result=paginateAuthorMaterials(items,page);
  const target=(value:number)=>`${authorPath}${value>1?`?page=${value}`:''}#author-materials`;
  const action=(label:string,value:number,disabled:boolean,content:string,edge=false)=>disabled?
    <button type="button" aria-label={label} disabled data-edge={edge||undefined}>{content}</button>:
    <Link href={target(value)} aria-label={label} data-edge={edge||undefined}>{content}</Link>;
  return <nav className={styles.pagination} aria-label={t.pagination}>
    <span className={styles.srOnly} role="status">{t.showing} {result.start}–{result.end} {t.of} {result.total} {t.resultUnit}</span>
    {action(t.first,1,result.page===1,'«',true)}{action(t.previous,result.page-1,result.page===1,'‹')}
    {authorPageWindow(result.page,result.pages).map(value=><Link key={value} href={target(value)} aria-label={`${t.page} ${value}`} aria-current={value===result.page?'page':undefined}>{value}</Link>)}
    {action(t.next,result.page+1,result.page===result.pages,'›')}{action(t.last,result.pages,result.page===result.pages,'»',true)}
  </nav>;
}

const publicCopySourcesForLocale={t,authorPath};
