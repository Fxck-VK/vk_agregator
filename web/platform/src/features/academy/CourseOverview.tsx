"use client";
import {useState} from "react";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import Link from "@/i18n/Link";
import {useLocale} from "@/i18n/LocaleProvider";
import {courseEn as copy} from "@/i18n/public/course-en";
import {getCourseLessons,getCourseReadingMinutes,coursePath,lessonReadingMinutes} from "./course-model";
import {CourseCover} from "./CourseCover";
import styles from "./Academy.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function CourseOverview({onOverview}:{onOverview:()=>void}){
  const {copy,coursePath} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale(),courseLessons=getCourseLessons(locale),courseReadingMinutes=getCourseReadingMinutes(locale);

  const [expanded,setExpanded]=useState(false);
  return <><CourseCover onOverview={onOverview}/><div className={styles.overviewGrid}>
    <div className={styles.description}><div id="course-description" className={expanded?styles.descriptionExpanded:styles.descriptionCollapsed}>{copy.description.map(text=><p key={text}>{text}</p>)}</div><button type="button" aria-expanded={expanded} aria-controls="course-description" onClick={()=>setExpanded(!expanded)}>{expanded?copy.labels.seeLess:copy.labels.seeMore}</button></div>
    <aside className={styles.facts} aria-label={copy.labels.details}><Link className={styles.start} href={`${coursePath}/${courseLessons[0].slug}`}><HelpIcon kind="play"/>{copy.labels.start}</Link><section className={styles.detailCard}><h2>{copy.labels.details}</h2><p><HelpIcon kind="book"/>{courseLessons.length} {copy.lessonUnit}</p><p><HelpIcon kind="clock"/>~{courseReadingMinutes} {copy.readingUnit}</p><p><HelpIcon kind="level"/>{copy.level}</p><p>{copy.format}</p><small>{copy.readingNote}</small></section><section className={styles.tags}><h2>{copy.labels.tags}</h2><ul>{copy.tags.map(tag=><li key={tag}>{tag}</li>)}</ul></section></aside>
    <div className={styles.curriculum}>{copy.modules.map((module,index)=><section key={module.id} id={`module-${module.id}`} data-course-module><h2>{String(index+1).padStart(2,'0')}. {module.title}</h2><ul>{module.lessons.map(lesson=><li key={lesson.slug} data-course-lesson><Link href={`${coursePath}/${lesson.slug}`}><span className={styles.lessonCircle} aria-hidden="true"/><span>{lesson.title}</span><span className={styles.lessonTime}><HelpIcon kind="clock"/>{lessonReadingMinutes({...lesson,moduleId:module.id,moduleTitle:module.title})} {copy.readingUnit}</span></Link></li>)}</ul></section>)}</div>
  </div></>;
}

const publicCopySourcesForLocale={copy,coursePath};
