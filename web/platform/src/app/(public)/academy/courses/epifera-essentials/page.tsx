import {localizePublicData} from "@/i18n/public/public-copy";
import type {Metadata} from "next";
import {AcademyCourse} from "@/features/academy/AcademyCourse";
import {courseEn} from "@/i18n/public/course-en";
import {getRequestLocale} from "@/i18n/server";
import {publicPageMetadata} from "@/i18n/seo";
import {coursePublication} from "./publication";
export async function generateMetadata():Promise<Metadata>{return {...publicPageMetadata(await getRequestLocale(),coursePublication),...localizePublicData(courseEn.metadata,await getRequestLocale())};}
export default async function CoursePage(){return <AcademyCourse/>;}
