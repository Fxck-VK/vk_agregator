"use client";
import {useState} from "react";
import {HelpCenterFrame} from "@/components/public/help-center/HelpCenterFrame";
import type {HelpRailItem} from "@/components/public/help-center/types";
import {EditorialIcon,EditorialShare} from "@/components/public/editorial/EditorialControls";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import Link from "@/i18n/Link";
import {useLocale} from "@/i18n/LocaleProvider";
import {courseEn as copy} from "@/i18n/public/course-en";
import {AcademyNavigation} from "./AcademyNavigation";
import {CourseOverview} from "./CourseOverview";
import {CourseLesson} from "./CourseLesson";
import {getCourseLessons,coursePath,type CourseLessonRecord} from "./course-model";
import frame from "@/components/public/help-center/HelpCenter.module.css";
import styles from "./Academy.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function AcademyCourse({lesson:sourceLesson}:{lesson?:CourseLessonRecord}){
  const {copy,coursePath} = usePublicCopy(publicCopySourcesForLocale);
  const lesson=usePublicCopy(sourceLesson),courseLessons=getCourseLessons(useLocale());

  const [dialog,setDialog]=useState<"overview"|"share"|null>(null);
  const shareLabels=lesson?{...copy.share,share:copy.labels.shareLesson}:copy.share;
  const rail:readonly HelpRailItem[]=[{href:"/app",label:copy.labels.newChat,icon:"plus"},{href:"/",label:copy.labels.home,icon:"home"},{href:"/app/models",label:copy.labels.models,icon:"grid"},{href:"/app/files",label:copy.labels.files,icon:"file"},{href:coursePath,label:copy.labels.academy,icon:"book",current:true,bottom:true},{href:"/legal",label:copy.labels.legal,icon:"file",bottom:true},{href:"mailto:support@epifera.com",label:copy.labels.support,icon:"support",bottom:true}];
  return <><HelpCenterFrame kind="academy" labels={copy.labels} rail={rail} breadcrumbs={[{label:copy.labels.academy},{label:copy.labels.courses},{label:copy.title,href:coursePath},...(lesson?[{label:lesson.title}]:[])]} readingKey={lesson?.slug} mainClassName={styles.courseMain} hideMobileBreadcrumb renderNavigation={onNavigate=><AcademyNavigation currentSlug={lesson?.slug} onNavigate={onNavigate}/>} utilityAction={<button type="button" className={`${frame.iconButton} ${frame.supportButton}`} aria-label={shareLabels.share} onClick={()=>setDialog("share")}><EditorialIcon kind="share"/></button>}>
    {lesson?<CourseLesson lesson={lesson}/>:<CourseOverview onOverview={()=>setDialog("overview")}/>}
  </HelpCenterFrame>
    {dialog==="share"?<EditorialShare labels={shareLabels} onClose={()=>setDialog(null)}/>:dialog==="overview"?<LandingDialog title={copy.labels.overview} closeLabel={copy.share.close} onClose={()=>setDialog(null)}><div className={styles.overviewDialog}><p>{copy.format}</p><h3>{copy.labels.goal}</h3><p>{copy.goal}</p><h3>{copy.labels.before}</h3><ul>{copy.prerequisites.map(item=><li key={item}>{item}</li>)}</ul><Link href={`${coursePath}/${courseLessons[0].slug}`}>{copy.labels.start}</Link></div></LandingDialog>:null}
  </>;
}


const publicCopySourcesForLocale={copy,coursePath};
