"use client";
import {courseEn as copy} from "@/i18n/public/course-en";
import Link from "@/i18n/Link";
import {useLocale} from "@/i18n/LocaleProvider";
import {getCourseLessons,coursePath,lessonReadingMinutes,type CourseLessonRecord} from "./course-model";
import styles from "./Academy.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function CourseLesson({lesson:sourceLesson}:{lesson:CourseLessonRecord}){
  const {coursePath,copy} = usePublicCopy(publicCopySourcesForLocale);
  const lesson=usePublicCopy(sourceLesson),courseLessons=getCourseLessons(useLocale());

  const index=courseLessons.findIndex(item=>item.slug===lesson.slug),previous=courseLessons[index-1],next=courseLessons[index+1];
  return <article className={styles.lessonArticle}><Link href={coursePath} className={styles.back}>{copy.labels.back}</Link>
    <header className={styles.lessonHeader}><p>{copy.labels.lesson} {index+1} / {courseLessons.length} · {lesson.moduleTitle}</p><h1>{lesson.title}</h1><span>{copy.format} · {lessonReadingMinutes(lesson)} {copy.readingUnit}</span></header>
    <section><h2>{copy.labels.goal}</h2><p>{lesson.goal}</p><p>{lesson.introduction}</p></section>
    <section><h2>{copy.labels.steps}</h2><ol>{lesson.steps.map(step=><li key={step}>{step}</li>)}</ol></section>
    <section className={styles.exercise}><h2>{copy.labels.exercise}</h2><p>{lesson.exercise}</p><Link href={lesson.workspace}>{copy.labels.practice}<span aria-hidden="true"> ↗</span></Link></section>
    <section><h2>{copy.labels.result}</h2><p>{lesson.result}</p></section>
    <nav aria-label={copy.title} className={styles.lessonPager}>{previous?<Link href={`${coursePath}/${previous.slug}`} aria-label={copy.labels.previous}><span>{copy.labels.previous}</span>{previous.title}</Link>:<span/>}{next?<Link href={`${coursePath}/${next.slug}`} aria-label={copy.labels.next}><span>{copy.labels.next}</span>{next.title}</Link>:<Link href={coursePath}>{copy.labels.back}</Link>}</nav>
  </article>;
}

const publicCopySourcesForLocale={copy,coursePath};
