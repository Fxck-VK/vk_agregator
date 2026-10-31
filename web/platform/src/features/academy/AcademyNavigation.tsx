"use client";
import {useId,useState} from "react";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {courseEn as copy} from "@/i18n/public/course-en";
import Link from "@/i18n/Link";
import {useLocale} from "@/i18n/LocaleProvider";
import {coursePath,searchCourseModules} from "./course-model";
import styles from "./Academy.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function AcademyNavigation({currentSlug,onNavigate}:{currentSlug?:string;onNavigate?:()=>void}){
  const {copy,coursePath} = usePublicCopy(publicCopySourcesForLocale,{"copy":"courseEn","coursePath":"__route.basePath"});

  const locale=useLocale(),[query,setQuery]=useState(""),searchId=useId(),normalized=query.trim().toLocaleLowerCase(locale);
  const modules=searchCourseModules(query,locale,copy);
  return <nav className={styles.navigation} aria-label={copy.labels.sectionTitle}>
    <div className={styles.search}><HelpIcon kind="search"/><input id={searchId} type="search" aria-label={copy.labels.search} placeholder={copy.labels.search} value={query} onChange={e=>setQuery(e.target.value)}/></div>
    <a className={styles.navigationLink} href="mailto:support@epifera.com" onClick={onNavigate}><HelpIcon kind="support"/>{copy.labels.support}</a>
    <Link className={styles.navigationLink} href={coursePath} onClick={onNavigate}><HelpIcon kind="book"/>{copy.labels.academy}</Link>
    <details key={normalized} className={styles.tree} open><summary>{copy.labels.courses}</summary><details open><summary><Link href={coursePath} aria-current={!currentSlug?"page":undefined} onClick={onNavigate}>{copy.title}</Link></summary>
      {modules.length?modules.map(module=><details key={`${module.id}-${!!normalized}`} open={!!normalized||module.lessons.some(lesson=>lesson.slug===currentSlug)||undefined}><summary>{String(copy.modules.findIndex(m=>m.id===module.id)+1).padStart(2,'0')}. {module.title}</summary><div className={styles.lessonLinks}>{module.lessons.map(lesson=><Link key={lesson.slug} href={`${coursePath}/${lesson.slug}`} aria-current={lesson.slug===currentSlug?"page":undefined} onClick={onNavigate}>{lesson.title}</Link>)}</div></details>):<p role="status" className={styles.noResults}>{copy.labels.noResults}</p>}
    </details></details>
    <Link className={styles.navigationLink} href="/blog/ai-art-prompts" onClick={onNavigate}><HelpIcon kind="file"/>{copy.labels.articles}</Link>
    <Link className={styles.navigationLink} href="/ai/docs" onClick={onNavigate}><HelpIcon kind="support"/>{copy.labels.help}</Link>
  </nav>;
}

const publicCopySourcesForLocale={copy,coursePath};
