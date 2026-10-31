import {courseEn} from "@/i18n/public/course-en";
import type {Locale} from "@/i18n/locales";
import {localizePublicData} from "@/i18n/public/public-copy";
export const coursePath=`/academy/courses/${courseEn.slug}`;
export const courseLessons=courseEn.modules.flatMap(module=>module.lessons.map(lesson=>({...lesson,moduleId:module.id,moduleTitle:module.title})));
// Course inputs are immutable; retain one projection per source for locale caching.
const lessonsByCourse=new WeakMap<typeof courseEn,typeof courseLessons>([[courseEn,courseLessons]]);
export function getCourseLessons(locale:Locale="en",source:typeof courseEn=courseEn){
  let lessons=lessonsByCourse.get(source);
  if(!lessons){
    lessons=source.modules.flatMap(module=>module.lessons.map(lesson=>({...lesson,moduleId:module.id,moduleTitle:module.title})));
    lessonsByCourse.set(source,lessons);
  }
  return localizePublicData(lessons,locale);
}
export const findCourseLesson=(slug:string,locale:Locale="en")=>getCourseLessons(locale).find(lesson=>lesson.slug===slug);
export type CourseLessonRecord=typeof courseLessons[number];
export const lessonReadingMinutes=(lesson:CourseLessonRecord)=>Math.max(1,Math.ceil([lesson.goal,lesson.introduction,...lesson.steps,lesson.exercise,lesson.result].join(' ').trim().split(/\s+/).length/200));
export const courseReadingMinutes=courseLessons.reduce((sum,lesson)=>sum+lessonReadingMinutes(lesson),0);
export const getCourseReadingMinutes=(locale:Locale="en",source:typeof courseEn=courseEn)=>getCourseLessons(locale,source).reduce((sum,lesson)=>sum+lessonReadingMinutes(lesson),0);
export function searchCourseModules(query:string,locale:Locale="en",source:typeof courseEn=courseEn){
  const normalized=query.trim().toLocaleLowerCase(locale);
  return localizePublicData(source.modules,locale).map(module=>({...module,lessons:module.lessons.filter(lesson=>!normalized||`${module.title} ${lesson.title}`.toLocaleLowerCase(locale).includes(normalized))})).filter(module=>module.lessons.length);
}
