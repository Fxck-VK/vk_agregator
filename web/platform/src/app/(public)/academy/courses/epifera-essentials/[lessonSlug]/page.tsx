import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {notFound} from "@/i18n/server";
import {AcademyCourse} from "@/features/academy/AcademyCourse";
import {findCourseLesson} from "@/features/academy/course-model";
import {courseEn} from "@/i18n/public/course-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {publications} from "./publication";
type Props={params:Promise<{lessonSlug:string}>};
export async function generateMetadata({params}:Props):Promise<Metadata>{const {lessonSlug}=await params,lesson=localizePublicData(findCourseLesson(lessonSlug),await getRequestLocale()),publication=publications.find(item=>item.id===`epifera-lesson-${lessonSlug}`);if(!lesson||!publication)notFound();return {...publicPageMetadata(await getRequestLocale(),publication),title:`${lesson.title} | ${localizePublicData(courseEn,await getRequestLocale()).title}`,description:lesson.goal};}
export default async function LessonPage({params}:Props){const lesson=findCourseLesson((await params).lessonSlug);if(!lesson)notFound();return <AcademyCourse lesson={lesson}/>;}
