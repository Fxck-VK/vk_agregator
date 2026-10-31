"use client";
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from "react";
import {LandingHeader} from "@/components/public/landing/LandingHeader";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {LandingArrow} from "@/components/public/landing/LandingPrimitives";
import {FAQ} from "@/components/ui/FAQ/FAQ";
import {HomeSearch} from "@/features/public-home/HomeSearch";
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {workflowEn as copy,type WorkflowRecord} from "@/i18n/public/workflow-en";
import Link from "@/i18n/Link";
import {WorkflowHero} from "./WorkflowHero";
import {WorkflowAudiences,WorkflowDiscover,WorkflowFeatured,WorkflowFeatures,WorkflowScenarios,WorkflowTools} from "./WorkflowSections";
import {WorkflowWalkthrough} from "./WorkflowWalkthrough";
import styles from "./Workflow.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function WorkflowLanding({workflow:sourceWorkflow}:{workflow:WorkflowRecord}){
  const {copy,aiToolsEn,homeEn} = usePublicCopy(publicCopySourcesForLocale,{"copy":"workflowEn"});
  const workflow=usePublicCopy(sourceWorkflow);

  const [search,setSearch]=useState(false),[preview,setPreview]=useState<{workflow:WorkflowRecord;step:number}|null>(null),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null);
  const openSearch=useCallback(()=>{if(!preview){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setSearch(true);}},[preview]);
  useLayoutEffect(()=>{const page=root.current;if(!page)return;const before=page.inert;page.inert=search||!!preview;return()=>{page.inert=before;};},[search,preview]);
  useEffect(()=>{if(search||preview)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[search,preview]);
  const openWorkflow=(selected:WorkflowRecord,step=0)=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setPreview({workflow:selected,step});};
  const explore=(step=0)=>openWorkflow(workflow,step);
  return <><div className={styles.page} ref={root} data-workflow-page><a className={styles.skip} href="#public-main">{copy.skip}</a><Link className={styles.announcement} href="/">{copy.announcement}<LandingArrow/></Link><LandingHeader navigation={aiToolsEn.navigation} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} onSearch={openSearch}/><main id="public-main"><WorkflowHero workflow={workflow} onExplore={explore}/><div className={styles.sections}>
    <WorkflowDiscover onExplore={()=>explore()}/><WorkflowFeatured onExplore={()=>explore()}/><WorkflowFeatures/><WorkflowAudiences/><WorkflowTools/><WorkflowScenarios onExplore={scenario=>openWorkflow(scenario)}/><section id="questions" className={styles.questions}><h2>{copy.questionsTitle}</h2><FAQ items={copy.faq} name="workflow-faq"/></section><section id="start" className={styles.closing}><h2>{copy.closing.title}</h2><p>{copy.closing.description}</p><Link className={styles.primary} href="/app">{copy.closing.action}</Link></section>
  </div></main><LandingFooter {...aiToolsEn.footer} layout="centered"/></div>{search?<HomeSearch copy={homeEn} onClose={()=>setSearch(false)}/>:null}{preview?<WorkflowWalkthrough key={`${preview.workflow.id}-${preview.step}`} workflow={preview.workflow} initialStep={preview.step} onClose={()=>setPreview(null)}/>:null}</>;
}


const publicCopySourcesForLocale={homeEn,aiToolsEn,copy};
