"use client";
import {Fragment} from "react";
import Link from "@/i18n/Link";
import {useLocale} from "@/i18n/LocaleProvider";
import {documentationArticles,documentationEn as copy,type DocumentationArticle as Article,type DocumentationBlock,type DocumentationInline} from "@/i18n/public/documentation-en";
import {documentationArticlePath,documentationNeighbours} from "./documentation-model";
import styles from "./Documentation.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


function Inline({parts}:{parts:readonly DocumentationInline[]}){
  return parts.map((part,index)=>typeof part==="string"?<Fragment key={index}>{part}</Fragment>:<Link key={index} href={part.href}>{part.text}</Link>);
}
function Blocks({blocks}:{blocks:readonly DocumentationBlock[]}){
  return blocks.map((block,index)=>{
    if(block.type==="paragraph")return <p key={index}><Inline parts={block.content}/></p>;
    if(block.type==="list")return <ul key={index}>{block.items.map(item=><li key={item}>{item}</li>)}</ul>;
    if(block.type==="steps")return <ol className={styles.steps} key={index}>{block.items.map(item=><li key={item.title}><p>{item.title}</p><p>{item.text}</p></li>)}</ol>;
    if(block.type==="questions")return <div key={index}>{block.items.map(item=><section className={styles.question} key={item.question}><h3>{item.question}</h3><p>{item.answer}</p></section>)}</div>;
    return <aside className={styles.callout} key={index}><span aria-hidden="true">ⓘ</span><p>{block.text}</p></aside>;
  });
}
export function DocumentationArticle({article:sourceArticle}:{article:Article}){
  const {copy,documentationArticles,documentationPath} = usePublicCopy(publicCopySourcesForLocale,{"copy":"documentationEn","documentationArticles":"documentationArticles","documentationPath":"__route.basePath"});
  const locale=useLocale(),article=usePublicCopy(sourceArticle);

  const {previous,next}=documentationNeighbours(article.slug,locale,documentationArticles);
  return <article className={styles.article} aria-labelledby="help-article-title">
    <div className={styles.hero}><h1 id="help-article-title">{article.title}</h1><p>{article.description}</p></div>
    <p className={styles.introduction}>{article.introduction}</p>
    <nav className={styles.toc} aria-labelledby="help-article-contents"><h2 id="help-article-contents">{copy.labels.toc}</h2><ul>{article.sections.map(section=><li key={section.id}><a href={`#${section.id}`}>{section.title}</a></li>)}</ul></nav>
    {article.sections.map(section=><section className={styles.section} key={section.id} id={section.id} data-help-section><h2>{section.title}</h2><Blocks blocks={section.blocks}/></section>)}
    <nav className={styles.neighbours} aria-label={copy.labels.articles}>
      {previous?<Link href={documentationArticlePath(previous.slug,documentationPath)} aria-label={`${copy.labels.previous}: ${previous.title}`}><small>← {copy.labels.previous}</small><span>{previous.title}</span></Link>:<span/>}
      {next?<Link href={documentationArticlePath(next.slug,documentationPath)} aria-label={`${copy.labels.next}: ${next.title}`}><small>{copy.labels.next} →</small><span>{next.title}</span></Link>:<span/>}
    </nav>
    <HelpSupport/>
  </article>;
}
export function HelpSupport(){
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"documentationEn","documentationArticles":"documentationArticles","documentationPath":"__route.basePath"});
return <aside className={styles.support}><h2>{copy.labels.supportTitle}</h2><p>{copy.labels.supportDescription}</p><a href="mailto:support@epifera.com">{copy.labels.supportAction}<span aria-hidden="true">↗</span></a></aside>;}


const publicCopySourcesForLocale={copy,documentationArticles,documentationPath:"/ai/docs"};
