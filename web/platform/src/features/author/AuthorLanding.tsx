"use client";
import Image from 'next/image';
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from 'react';
import {BrandLogo} from '@/components/brand/BrandLogo/BrandLogo';
import {LandingHeader} from '@/components/public/landing/LandingHeader';
import {LandingFooter} from '@/components/public/landing/LandingFooter';
import {LandingDialog} from '@/components/public/landing/LandingDialog';
import {homeEn} from '@/i18n/public/home-en';
import {aiToolsEn} from '@/i18n/public/ai-tools-en';
import {authorEn as t,authorProfile,type AuthorMaterial} from '@/i18n/public/author-en';
import Link from '@/i18n/Link';
import {paginateAuthorMaterials} from './author-model';
import {AuthorPagination} from './AuthorPagination';
import {AuthorSearch} from './AuthorSearch';
import styles from './Author.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function AuthorLanding({items,page}:{items:readonly AuthorMaterial[];page:number}){
  const {aiToolsEn,t,homeEn,authorProfile} = usePublicCopy(publicCopySourcesForLocale,{"t":"authorEn","authorProfile":"authorProfile"});

  const[dialog,setDialog]=useState<'search'|'source'|null>(null);
  const root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),open=dialog!==null;
  const launchSource=()=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog('source');};
  const search=useCallback(()=>{if(!dialog){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog('search');}},[dialog]);
  useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
  useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
  const result=paginateAuthorMaterials(items,page);
  const navigation=[aiToolsEn.navigation[0],{label:t.tools,href:'/ai'},aiToolsEn.navigation[2],{label:t.pricing,href:'/pricing'}];
  return <>
    <div className={styles.page} ref={root} data-author-page>
      <a href="#public-main" className={styles.skip}>{t.skip}</a>
      <LandingHeader variant="editorial" siteAction={{label:t.home,href:'/'}} navigation={navigation} labels={{...homeEn.controls,search:t.search,login:homeEn.login,signup:homeEn.signup}} onSearch={search}/>
      <main id="public-main" className={styles.frame}>
        <section className={styles.profile} aria-labelledby="author-title"><span className={styles.avatar}><BrandLogo size={64}/></span><h1 id="author-title">{authorProfile.name}</h1></section>
        <section id="author-materials" className={styles.materials} aria-labelledby="author-materials-title" tabIndex={-1}>
          <h2 id="author-materials-title" className={styles.srOnly}>{t.materials}</h2>
          <div className={styles.grid}>{result.items.map((item,index)=><Link href={item.href} key={item.id} className={styles.card} data-author-material>
            <span className={styles.cover}>{item.cover.src?<Image src={item.cover.src} fill alt="" sizes="(max-width:679px) 95vw,(max-width:1000px) 45vw,33vw" preload={index===0}/>:null}</span>
            <span className={styles.category}>{item.category}</span><h3>{item.title}</h3>
            {item.date?<time dateTime={item.date.iso}>{item.date.label}</time>:<span className={styles.draft}>{t.draftPreview}</span>}
          </Link>)}</div>
        </section>
        <AuthorPagination items={items} page={result.page}/>
        <button className={styles.sourceAction} type="button" onClick={launchSource}>{t.source.action}<span aria-hidden="true"> ↗</span></button>
      </main>
      <LandingFooter {...aiToolsEn.footer} layout="centered"/>
    </div>
    {dialog==='search'?<AuthorSearch items={items} onClose={()=>setDialog(null)}/>:dialog==='source'?<LandingDialog title={t.source.title} closeLabel={t.close} onClose={()=>setDialog(null)}><div className={styles.sourceCopy}><p>{t.source.description}</p><a href="https://www.magnific.com/blog/author/magnific/" target="_blank" rel="noopener noreferrer">{t.source.link}<span aria-hidden="true"> ↗</span></a></div></LandingDialog>:null}
  </>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,t,authorProfile};
