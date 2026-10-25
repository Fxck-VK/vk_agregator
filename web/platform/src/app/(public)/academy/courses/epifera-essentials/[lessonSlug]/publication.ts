import "server-only";
import type {PublicPagePublication} from "@/content/publication/types";
import {courseLessons,coursePath} from "@/features/academy/course-model";
export const publications:readonly PublicPagePublication[]=courseLessons.map(lesson=>({id:`epifera-lesson-${lesson.slug}`,status:"draft",indexing:"noindex",paths:{en:`${coursePath}/${lesson.slug}`,ru:`${coursePath}/${lesson.slug}`}}));
