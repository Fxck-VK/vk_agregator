"use client";
import {useId,useState} from "react";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import Link from "@/i18n/Link";
import {useLocale} from "@/i18n/LocaleProvider";
import {documentationArticles,documentationEn as copy,documentationGroups} from "@/i18n/public/documentation-en";
import {documentationArticlePath,searchDocumentation} from "./documentation-model";
import styles from "./Documentation.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function DocumentationNavigation({currentSlug,onNavigate}:{currentSlug?:string;onNavigate?:()=>void}){
  const {copy,documentationGroups,documentationArticles,documentationPath} = usePublicCopy(publicCopySourcesForLocale,{"copy":"documentationEn","documentationArticles":"documentationArticles","documentationPath":"__route.basePath","documentationGroups":"documentationGroups"});

  const locale=useLocale(),[query,setQuery]=useState(""),searchId=useId(),normalized=query.trim().toLocaleLowerCase(locale);
  const matches=searchDocumentation(query,locale,documentationArticles);
  return <nav className={styles.navigation} aria-label={copy.labels.sectionTitle}>
    <div className={styles.search}><HelpIcon kind="search"/><input id={searchId} type="search" aria-label={copy.labels.search} placeholder={copy.labels.searchPlaceholder} value={query} onChange={e=>setQuery(e.target.value)}/></div>
    <a className={styles.navigationLink} href="mailto:support@epifera.com" onClick={onNavigate}><HelpIcon kind="support"/>{copy.labels.support}</a>
    <Link className={styles.navigationLink} href="/academy/courses/epifera-essentials" onClick={onNavigate}><HelpIcon kind="book"/>{copy.labels.academy}</Link>
    <div className={styles.navigationLink}><HelpIcon kind="file"/>{copy.labels.articles}</div>
    <div className={styles.tree} key={normalized}>{documentationGroups.map(group=>{
      const articles=matches.filter(article=>article.group===group.id);if(!articles.length)return null;
      return <details key={group.id} open={!!normalized||articles.some(a=>a.slug===currentSlug)||undefined}><summary><span aria-hidden="true">›</span>{group.title}</summary><div className={styles.articleLinks}>{articles.map(article=><Link href={documentationArticlePath(article.slug,documentationPath)} key={article.slug} aria-current={article.slug===currentSlug?"page":undefined} onClick={onNavigate}>{article.title}</Link>)}</div></details>;
    })}{!matches.length?<p role="status" className={styles.noResults}>{copy.labels.noResults}</p>:null}</div>
  </nav>;
}

const publicCopySourcesForLocale={copy,documentationGroups,documentationArticles,documentationPath:"/ai/docs"};
