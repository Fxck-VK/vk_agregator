"use client";
import {useId,useRef,useState} from "react";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {ModelPicture} from "@/components/public/generator-landing/ModelMedia";
import {workflowEn as copy,type WorkflowRecord} from "@/i18n/public/workflow-en";
import Link from "@/i18n/Link";
import styles from "./Workflow.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function WorkflowWalkthrough({workflow,initialStep=0,onClose}:{workflow:WorkflowRecord;initialStep?:number;onClose:()=>void}){
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"workflowEn"});

  const [index,setIndex]=useState(Math.max(0,Math.min(initialStep,workflow.steps.length-1))),id=useId(),tabs=useRef<(HTMLButtonElement|null)[]>([]),step=workflow.steps[index];
  const select=(next:number,focus=false)=>{const bounded=Math.max(0,Math.min(next,workflow.steps.length-1));setIndex(bounded);if(focus)tabs.current[bounded]?.focus();};
  return <LandingDialog title={workflow.title} closeLabel={copy.close} onClose={onClose} wide><div className={styles.walkthrough}>
    <span className={styles.guideLabel}>{copy.guide}</span><p className={styles.guideDescription}>{workflow.description}</p>
    <div className={styles.walkthroughColumns}><aside className={styles.requirements}><h3>{copy.inputs}</h3><ul>{workflow.inputs.map(input=><li key={input}>{input}</li>)}</ul><h3>{copy.outputs}</h3><ul>{workflow.outputs.map(output=><li key={output}>{output}</li>)}</ul><p className={styles.guideNote}>{copy.guideNote}</p></aside>
      <div className={styles.stepBody}><div className={styles.stepTabs} role="tablist" aria-label={copy.steps}>{workflow.steps.map((item,i)=><button type="button" role="tab" key={item.title} ref={node=>{tabs.current[i]=node;}} id={`${id}-step-${i}`} aria-controls={`${id}-panel`} aria-selected={index===i} tabIndex={index===i?0:-1} onClick={()=>select(i)} onKeyDown={event=>{let next:number|undefined;if(event.key==="ArrowRight")next=i+1;else if(event.key==="ArrowLeft")next=i-1;else if(event.key==="Home")next=0;else if(event.key==="End")next=workflow.steps.length-1;if(next!==undefined){event.preventDefault();select(next,true);}}}><span>{i+1}</span>{item.title}</button>)}</div>
      <div id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-step-${index}`} tabIndex={0} className={styles.stepPanel}><div className={styles.stepImage}><ModelPicture assetBase="workflows" image={{file:workflow.image,alt:workflow.title}} sizes="(max-width:767px) 80vw,450px"/></div><small>{copy.step} {index+1} {copy.of} {workflow.steps.length}</small><h3>{step.title}</h3><p>{step.description}</p><h4>{copy.review}</h4><ul>{step.checklist.map(item=><li key={item}>{item}</li>)}</ul><Link href={step.href} className={styles.continue} onClick={onClose}>{copy.continue}<span aria-hidden="true">↗</span></Link></div>
      <div className={styles.stepControls}><button type="button" disabled={index===0} onClick={()=>select(index-1)} aria-label={copy.previousStep}>← {copy.previous}</button><span aria-live="polite">{index+1} / {workflow.steps.length}</span><button type="button" disabled={index===workflow.steps.length-1} onClick={()=>select(index+1)} aria-label={copy.nextStep}>{copy.next} →</button></div></div>
    </div><p className={styles.source}>{copy.source}</p>
  </div></LandingDialog>;
}

const publicCopySourcesForLocale={copy};
