"use client";
import {useEffect,useState} from "react";
import {HelpCenterFrame} from "@/components/public/help-center/HelpCenterFrame";
import type {HelpRailItem} from "@/components/public/help-center/types";
import {EditorialIcon,EditorialShare} from "@/components/public/editorial/EditorialControls";
import Link from "@/i18n/Link";
import {documentationArticles,documentationEn as copy,documentationGroups,type DocumentationArticle as Article} from "@/i18n/public/documentation-en";
import {DocumentationArticle,HelpSupport} from "./DocumentationArticle";
import {DocumentationNavigation} from "./DocumentationNavigation";
import {documentationArticlePath,documentationPath} from "./documentation-model";
import frame from "@/components/public/help-center/HelpCenter.module.css";
import styles from "./Documentation.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function DocumentationCenter({article:sourceArticle}:{article?:Article}){
  const {documentationGroups,copy,documentationArticles} = usePublicCopy(publicCopySourcesForLocale);
  const article=usePublicCopy(sourceArticle);

  const [sharing,setSharing]=useState(false),group=documentationGroups.find(g=>g.id===article?.group);
  useEffect(()=>{if(!article)return;const target=article.sections.find(section=>`#${section.id}`===window.location.hash);if(!target)return;const id=requestAnimationFrame(()=>document.getElementById(target.id)?.scrollIntoView?.({block:"start"}));return()=>cancelAnimationFrame(id);},[article]);
  const rail:readonly HelpRailItem[]=[{href:"/app",label:copy.labels.newChat,icon:"plus"},{href:"/",label:copy.labels.home,icon:"home"},{href:"/app/models",label:copy.labels.models,icon:"grid"},{href:"/app/files",label:copy.labels.files,icon:"file"},{href:documentationPath,label:copy.title,icon:"support",current:true,bottom:true},{href:"/academy/courses/epifera-essentials",label:copy.labels.academy,icon:"book",bottom:true},{href:"/legal",label:copy.labels.legal,icon:"file",bottom:true}];
  return <><HelpCenterFrame kind="documentation" labels={copy.labels} rail={rail} readingKey={article?.slug??"index"} mainClassName={styles.main} breadcrumbs={[{label:copy.title,href:documentationPath},...(article?[{label:group!.title},{label:article.title}]:[])]} renderNavigation={onNavigate=><DocumentationNavigation key={article?.slug??"index"} currentSlug={article?.slug} onNavigate={onNavigate}/>} utilityAction={<button className={`${frame.iconButton} ${frame.supportButton}`} type="button" aria-label={copy.share.share} onClick={()=>setSharing(true)}><EditorialIcon kind="share"/></button>}>
    {article?<DocumentationArticle article={article}/>:<div className={styles.article}><div className={styles.hero}><h1>{copy.title}</h1><p>{copy.description}</p></div><p className={styles.introduction}>{copy.directoryIntro}</p><div className={styles.directory}>{documentationGroups.map(group=><section key={group.id}><h2>{group.title}</h2><ul>{documentationArticles.filter(a=>a.group===group.id).map(a=><li key={a.slug}><Link href={documentationArticlePath(a.slug)}><span>{a.title}</span>{" "}<small>{a.description}</small></Link></li>)}</ul></section>)}</div><HelpSupport/></div>}
  </HelpCenterFrame>{sharing?<EditorialShare labels={copy.share} onClose={()=>setSharing(false)}/>:null}</>;
}



const publicCopySourcesForLocale={documentationArticles,copy,documentationGroups};
