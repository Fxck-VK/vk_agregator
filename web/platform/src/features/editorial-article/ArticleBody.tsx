import {Fragment} from "react";
import {EditorialPicture} from "@/components/public/editorial/EditorialPicture";
import type {EditorialImage} from "@/components/public/editorial/types";
import Link from "@/i18n/Link";
import type {ArticleBlock,ArticleInline,ArticleRecord} from "@/i18n/public/article-types";
import styles from "@/components/public/editorial/Editorial.module.css";

function Inline({content}:{content:readonly ArticleInline[]}) {
  return content.map((part,index)=>typeof part==="string"?<Fragment key={index}>{part}</Fragment>:part.href?<Link key={index} href={part.href}>{part.strong?<strong>{part.text}</strong>:part.text}</Link>:part.strong?<strong key={index}>{part.text}</strong>:<Fragment key={index}>{part.text}</Fragment>);
}
function Blocks({blocks,images,source}:{blocks:readonly ArticleBlock[];images:readonly EditorialImage[];source:string}) {
  return blocks.map((block,index)=>{
    if(block.type==="paragraph")return <p key={index}><Inline content={block.content}/></p>;
    if(block.type==="image"){
      const image=images[block.imageIndex];
      return <figure key={index} className={styles.illustration} data-article-illustration><div className={styles.illustrationMedia} style={{aspectRatio:`${image.width}/${image.height}`}}><EditorialPicture image={image} sizes="(max-width:767px) 85vw,65vw"/></div><figcaption>{source}</figcaption></figure>;
    }
    const List=block.ordered?"ol":"ul";
    return <List key={index}>{block.items.map((item,itemIndex)=><li key={itemIndex} data-prompt-example={item.example||undefined}><Inline content={item.content}/></li>)}</List>;
  });
}
export function articleReadingMinutes(article:ArticleRecord){
  const blocks=[...article.lead,...article.sections.flatMap(section=>section.blocks),...article.closing];
  const content=blocks.flatMap(block=>block.type==="paragraph"?block.content:block.type==="list"?block.items.flatMap(item=>item.content):[]);
  return Math.max(1,Math.ceil(content.map(part=>typeof part==="string"?part:part.text).join(" ").trim().split(/\s+/).length/200));
}
export function ArticleBody({article,images,source,notes}:{article:ArticleRecord;images:readonly EditorialImage[];source:string;notes:string}) {
  return <article className={styles.article} aria-label={article.title}>
    <Blocks blocks={article.lead} images={images} source={source}/>
    {article.sections.map(section=><section key={section.id} id={section.id} className={styles.chapter} data-article-section><h2>{section.title}</h2><Blocks blocks={section.blocks} images={images} source={source}/></section>)}
    <Blocks blocks={article.closing} images={images} source={source}/>
    <p id="article-notes" className={styles.articleNotes}>{notes}</p>
  </article>;
}
