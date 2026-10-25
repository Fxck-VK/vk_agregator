import "server-only";
import type {PublicPagePublication} from "@/content/publication/types";
import {coursePath} from "@/features/academy/course-model";
export const coursePublication:PublicPagePublication={id:"epifera-essentials-course",status:"draft",indexing:"noindex",paths:{en:coursePath,ru:coursePath}};
export const publications=[coursePublication];
