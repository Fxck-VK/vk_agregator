import {courseEn} from "@/i18n/public/course-en";
import type {Locale} from "@/i18n/locales";
import {localizePublicData} from "@/i18n/public/public-copy";
export const coursePath=`/academy/courses/${courseEn.slug}`;
export const courseLessons=courseEn.modules.flatMap(module=>module.lessons.map(lesson=>({...lesson,moduleId:module.id,moduleTitle:module.title})));
export const getCourseLessons=(locale:Locale="en")=>localizePublicData(courseLessons,locale);
export const findCourseLesson=(slug:string,locale:Locale="en")=>getCourseLessons(locale).find(lesson=>lesson.slug===slug);
export type CourseLessonRecord=typeof courseLessons[number];
export const lessonReadingMinutes=(lesson:CourseLessonRecord)=>Math.max(1,Math.ceil([lesson.goal,lesson.introduction,...lesson.steps,lesson.exercise,lesson.result].join(' ').trim().split(/\s+/).length/200));
export const courseReadingMinutes=courseLessons.reduce((sum,lesson)=>sum+lessonReadingMinutes(lesson),0);
export const getCourseReadingMinutes=(locale:Locale="en")=>getCourseLessons(locale).reduce((sum,lesson)=>sum+lessonReadingMinutes(lesson),0);
export function searchCourseModules(query:string,locale:Locale="en"){
  const normalized=query.trim().toLocaleLowerCase(locale);
  return localizePublicData(courseEn.modules,locale).map(module=>({...module,lessons:module.lessons.filter(lesson=>!normalized||`${module.title} ${lesson.title}`.toLocaleLowerCase(locale).includes(normalized))})).filter(module=>module.lessons.length);
}
