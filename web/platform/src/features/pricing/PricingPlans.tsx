"use client";
import {useId,useRef} from "react";
import {CreditAmount} from "@/components/ui/CreditAmount/CreditAmount";
import {keyboardTab} from "@/features/public-home/HomeSuite";
import {pricingEn as t} from "@/i18n/public/pricing-en";
import {subscriptionPlanPreview,type SubscriptionPreviewId} from "./subscription-preview";
import {budgetFor,formatPricingAmount,parseTeamSeats,type PricingPeriod} from "./pricing-model";
import styles from "./Pricing.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



export function PricingPeriodSwitch({period,onChange,comparison=false}:{period:PricingPeriod;onChange:(period:PricingPeriod)=>void;comparison?:boolean}){
  const publicCopyLocale=useLocale();
  const {t} = usePublicCopy(publicCopySourcesForLocale);
return <div className={styles.period}><span>{t.monthly}</span><button type="button" role="switch" aria-label={comparison?`${t.periodLabel}${translatePublicText(" in comparison",publicCopyLocale)}`:t.periodLabel} aria-checked={period==="yearly"} onClick={()=>onChange(period==="monthly"?"yearly":"monthly")}><span/></button><span>{t.yearly}</span></div>;}
export function PricingAudience({audience,onChange}:{audience:"individual"|"teams";onChange:(value:"individual"|"teams")=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

 const tabs=useRef<(HTMLButtonElement|null)[]>([]),id=useId(),items=[{id:"individual" as const,label:t.individual},{id:"teams" as const,label:t.teams}];
 return <div className={styles.audience} role="tablist" aria-label={t.audience}>{items.map((item,index)=><button type="button" role="tab" id={`${id}-${item.id}`} key={item.id} aria-controls="pricing-options" aria-selected={audience===item.id} tabIndex={audience===item.id?0:-1} ref={node=>{tabs.current[index]=node;}} onClick={()=>onChange(item.id)} onKeyDown={event=>{const next=keyboardTab(event,index,items.length);if(next!==null){onChange(items[next].id);tabs.current[next]?.focus();}}}>{item.label}</button>)}</div>;
}
function PlanCard({id,period,onPlan,onUnits,capacity,onCapacity}:{id:SubscriptionPreviewId;period:PricingPeriod;onPlan:(id:SubscriptionPreviewId)=>void;onUnits:()=>void;capacity?:boolean;onCapacity?:(id:"ultima"|"elite")=>void}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale);

 const plan=subscriptionPlanPreview(id),amount=budgetFor(plan,period)!;
 return <article className={styles.planCard} data-pricing-plan={id} data-featured={id==="pro"||undefined}><div className={styles.planTitle}><h2>{plan.name}</h2><span>{t.preview}</span></div><p className={styles.planDescription}>{t.planDescriptions[id]}</p>{capacity?<label className={styles.capacity}><span>{t.capacity}</span><select aria-label={t.capacity} value={id} onChange={event=>onCapacity?.(event.target.value as "ultima"|"elite")}><option value="ultima">{translatePublicText("Ultima",publicCopyLocale)}</option><option value="elite">{translatePublicText("Elite",publicCopyLocale)}</option></select></label>:null}<p className={styles.price}><strong><span className={styles.currency}>₽</span>{formatPricingAmount(amount,publicCopyLocale)}</strong><span>/{translatePublicText(period==="yearly"?"year":"month",publicCopyLocale)}<small>{period==="yearly"?t.yearBudget:t.monthlyPreview}</small></span></p><div className={styles.allowance}><p>{t.monthlyStars}</p><div><CreditAmount value={plan.stars}/><span>{translatePublicText("/month",publicCopyLocale)}</span></div><p className={styles.allowanceNote}>{t.allocationNote}</p></div><button type="button" className={styles.planAction} aria-label={translatePublicText("Explore {plan} preview",publicCopyLocale).replace("{plan}",plan.name)} onClick={()=>onPlan(id)}>{translatePublicText("Explore ",publicCopyLocale)}{plan.name}</button><ul className={styles.planPoints}>{t.planPoints.map(point=><li key={point}><span aria-hidden="true">✓</span>{point}</li>)}</ul><button type="button" className={styles.unitsLink} onClick={onUnits}>{translatePublicText("Review model costs ",publicCopyLocale)}<span aria-hidden="true">↗</span></button></article>;
}
export function PersonalPlans({period,capacity,onCapacity,onPlan,onUnits}:{period:PricingPeriod;capacity:"ultima"|"elite";onCapacity:(id:"ultima"|"elite")=>void;onPlan:(id:SubscriptionPreviewId)=>void;onUnits:()=>void}){return <div className={styles.personalGrid}><PlanCard id="start" period={period} onPlan={onPlan} onUnits={onUnits}/><PlanCard id="pro" period={period} onPlan={onPlan} onUnits={onUnits}/><PlanCard id={capacity} period={period} onPlan={onPlan} onUnits={onUnits} capacity onCapacity={onCapacity}/></div>;}
export function TeamPlans({seats,onSeats,onQuote}:{seats:string;onSeats:(value:string)=>void;onQuote:(enterprise:boolean)=>void}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale);

 const count=parseTeamSeats(seats),id=useId();
 return <div className={styles.teamGrid}><article className={styles.planCard} data-team-plan><h2>{t.team.title}</h2><p className={styles.planDescription}>{t.team.description}</p><p className={styles.customPrice}>{t.team.price}</p><span className={styles.teamStatus}>{t.team.status}</span><label className={styles.seats} htmlFor={id}>{t.team.seats}<input id={id} type="number" min="2" max="100" step="1" aria-label={t.team.seats} aria-invalid={count===null} aria-describedby={`${id}-hint`} value={seats} onChange={event=>onSeats(event.target.value)}/></label><p id={`${id}-hint`} className={styles.seatHint}>{count===null?t.team.seatsError:t.team.seatsHint}</p><button type="button" className={styles.planAction} disabled={count===null} onClick={()=>onQuote(false)}>{t.team.action}</button><ul className={styles.planPoints}>{t.team.points.map(point=><li key={point}><span aria-hidden="true">✓</span>{point}</li>)}</ul></article><article className={styles.planCard} data-team-plan><h2>{t.team.enterprise}</h2><p className={styles.planDescription}>{t.team.enterpriseDescription}</p><p className={styles.customPrice}>{translatePublicText("Let’s talk",publicCopyLocale)}</p><span className={styles.teamStatus}>{t.team.status}</span><p className={styles.teamNote}>{t.team.note}</p><button type="button" className={styles.planAction} disabled={count===null} onClick={()=>onQuote(true)}>{t.team.enterpriseAction}</button><ul className={styles.planPoints}>{t.team.points.map(point=><li key={point}><span aria-hidden="true">✓</span>{point}</li>)}</ul></article></div>;
}

const publicCopySourcesForLocale={t};
