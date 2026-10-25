"use client";
import {BrandLogo} from "@/components/brand/BrandLogo/BrandLogo";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {courseEn as copy} from "@/i18n/public/course-en";
import {useLocale} from "@/i18n/LocaleProvider";
import {getCourseLessons,getCourseReadingMinutes} from "./course-model";
import styles from "./Academy.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function CourseCover({onOverview}:{onOverview:()=>void}){
  const {copy} = usePublicCopy(publicCopySourcesForLocale);
  const locale=useLocale(),courseLessons=getCourseLessons(locale),courseReadingMinutes=getCourseReadingMinutes(locale);

  return <section className={styles.cover} aria-labelledby="course-title">
    <div className={styles.coverArt} aria-hidden="true"><span><HelpIcon kind="grid"/></span><span><HelpIcon kind="file"/></span><span className={styles.brandTile}><BrandLogo size={104}/></span><span><HelpIcon kind="level"/></span><span><HelpIcon kind="book"/></span></div>
    <span className={styles.coverBook} aria-hidden="true"><HelpIcon kind="book"/></span>
    <div className={styles.coverContent}><h1 id="course-title">{copy.title}</h1><p>{copy.subtitle}</p><div className={styles.coverFacts}><span><HelpIcon kind="level"/>{copy.level}</span><i aria-hidden="true">|</i><span><HelpIcon kind="book"/>{courseLessons.length} {copy.lessonUnit}</span><span><HelpIcon kind="clock"/>~{courseReadingMinutes} {copy.readingUnit}</span></div></div>
    <button type="button" className={styles.coverOverview} onClick={onOverview}><HelpIcon kind="book"/>{copy.labels.overview}</button>
  </section>;
}

const publicCopySourcesForLocale={copy};
