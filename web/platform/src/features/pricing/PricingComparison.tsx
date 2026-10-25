"use client";
import {useCallback,useId,useRef,useState} from "react";
import {PopoverPanel} from "@/components/ui/PopoverPanel/PopoverPanel";
import {ScrollArea} from "@/components/ui/ScrollArea/ScrollArea";
import {CreditAmount} from "@/components/ui/CreditAmount/CreditAmount";
import {pricingEn as t} from "@/i18n/public/pricing-en";
import {subscriptionPreviewPlans,subscriptionPlanPreview,type SubscriptionPreviewId} from "./subscription-preview";
import {budgetFor,formatPricingAmount,type PricingPeriod} from "./pricing-model";
import {PricingPeriodSwitch} from "./PricingPlans";
import styles from "./Pricing.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export function PricingComparison({period,onPeriod,selected,onSelected,onPlan,teams}:{period:PricingPeriod;onPeriod:(period:PricingPeriod)=>void;selected:readonly SubscriptionPreviewId[];onSelected:(ids:SubscriptionPreviewId[])=>void;onPlan:(id:SubscriptionPreviewId)=>void;teams:boolean}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale);

 const[open,setOpen]=useState(false),anchor=useRef<HTMLButtonElement>(null),id=useId(),close=useCallback(()=>setOpen(false),[setOpen]),plans=selected.map(subscriptionPlanPreview);
 const toggle=(key:SubscriptionPreviewId)=>{if(selected.includes(key)){if(selected.length>1)onSelected(selected.filter(value=>value!==key));}else onSelected([...selected,key]);};
 return <section className={styles.comparison} aria-labelledby="pricing-comparison-title"><div className={styles.comparisonHeading}><div><h2 id="pricing-comparison-title">{t.comparison.title}</h2><p>{teams?t.comparison.teamDescription:t.comparison.description}</p></div><PricingPeriodSwitch period={period} onChange={onPeriod} comparison/></div>{!teams?<div className={styles.compareActions}><button type="button" ref={anchor} onClick={()=>setOpen(value=>!value)} aria-expanded={open} aria-controls={open?id:undefined} aria-haspopup="dialog">{t.comparison.choose}<span aria-hidden="true">⌄</span></button><button type="button" onClick={()=>onSelected(['start','pro','ultima'])}>{t.comparison.restore}</button></div>:null}<ScrollArea orientation="horizontal" className={styles.tableScroll} viewportClassName={styles.tableViewport} viewportProps={{role:"region","aria-label":t.comparison.label,tabIndex:0}}><table className={styles.comparisonTable} style={{minWidth:teams?660:240+plans.length*210}}><caption className={styles.srOnly}>{t.comparison.label}</caption><thead><tr><th scope="col">{t.planDetails}</th>{teams?<><th scope="col">{t.team.title}<small>{t.team.price}</small></th><th scope="col">{t.team.enterprise}<small>{t.team.price}</small></th></>:plans.map(plan=><th scope="col" key={plan.id}><div><span>{plan.name}</span><button type="button" aria-label={translatePublicText("Remove {plan} from comparison",publicCopyLocale).replace("{plan}",plan.name)} disabled={plans.length===1} onClick={()=>toggle(plan.id)}>×</button></div><small>₽{formatPricingAmount(budgetFor(plan,period)!,publicCopyLocale)} /{translatePublicText(period==="yearly"?"year":"month",publicCopyLocale)}</small><button type="button" className={styles.tablePlanAction} aria-label={translatePublicText("View {plan} details",publicCopyLocale).replace("{plan}",plan.name)} onClick={()=>onPlan(plan.id)}>{translatePublicText("View preview",publicCopyLocale)}</button></th>)}</tr></thead><tbody><tr className={styles.groupRow}><th colSpan={teams?3:plans.length+1}>{translatePublicText("Stars",publicCopyLocale)}</th></tr><tr><th scope="row">{t.comparison.starRow}</th>{teams?<><td>{translatePublicText("Custom agreement",publicCopyLocale)}</td><td>{translatePublicText("Custom agreement",publicCopyLocale)}</td></>:plans.map(plan=><td key={plan.id}><CreditAmount value={plan.stars}/></td>)}</tr>{t.comparison.groups.map(group=><ComparisonGroup key={group.title} title={group.title} rows={group.rows} columns={teams?2:plans.length} teams={teams}/>)}</tbody></table></ScrollArea><PopoverPanel anchorRef={anchor} id={id} isOpen={open} onClose={close} label={t.comparison.picker} width={260} align="start" itemVariant="action"><div className={styles.planPicker}>{subscriptionPreviewPlans.filter(plan=>plan.period==='month').map(plan=><label key={plan.id}><input type="checkbox" checked={selected.includes(plan.id)} disabled={selected.includes(plan.id)&&selected.length===1} onChange={()=>toggle(plan.id)}/>{plan.name}</label>)}</div></PopoverPanel></section>;
}
function ComparisonGroup({title,rows,columns,teams}:{title:string;rows:readonly {label:string;value:string}[];columns:number;teams:boolean}){const publicCopyLocale=useLocale();return <><tr className={styles.groupRow}><th colSpan={columns+1}>{title}</th></tr>{rows.map(row=><tr key={row.label}><th scope="row">{row.label}</th>{Array.from({length:columns},(_,index)=><td key={index}>{translatePublicText(teams?"To be discussed":row.value,publicCopyLocale)}</td>)}</tr>)}</>;}



const publicCopySourcesForLocale={t};
