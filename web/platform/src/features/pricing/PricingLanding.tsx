"use client";
import {useCallback,useEffect,useLayoutEffect,useRef,useState} from "react";
import {LandingHeader} from "@/components/public/landing/LandingHeader";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {LandingArrow} from "@/components/public/landing/LandingPrimitives";
import {FAQ} from "@/components/ui/FAQ/FAQ";
import {HomeSearch} from "@/features/public-home/HomeSearch";
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {pricingEn as t} from "@/i18n/public/pricing-en";
import Link from "@/i18n/Link";
import {PricingAudience,PricingPeriodSwitch,PersonalPlans,TeamPlans} from "./PricingPlans";
import {PricingComparison} from "./PricingComparison";
import {PricingInformation,type PricingDialog} from "./PricingDialogs";
import {PricingBanner,PricingStories,PricingTools} from "./PricingSections";
import {parseTeamSeats,type PricingPeriod} from "./pricing-model";
import type {SubscriptionPreviewId} from "./subscription-preview";
import styles from "./Pricing.module.css";
import {useLocale} from "@/i18n/LocaleProvider";
import {subscriptionCopy} from "@/features/subscriptions/copy";
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function PricingLanding(){
  const {t,aiToolsEn,homeEn} = usePublicCopy(publicCopySourcesForLocale,{"t":"pricingEn"});
 const locale=useLocale(),copy=subscriptionCopy[locale];
 const faq=t.faq.filter((_,index)=>![1,2,3].includes(index));

 const[period,setPeriod]=useState<PricingPeriod>('monthly'),[audience,setAudience]=useState<'individual'|'teams'>('individual'),[capacity,setCapacity]=useState<'ultima'|'elite'>('ultima'),[seats,setSeats]=useState('2'),[comparison,setComparison]=useState<SubscriptionPreviewId[]>(['start','pro','ultima']),[dialog,setDialog]=useState<PricingDialog|null>(null),root=useRef<HTMLDivElement>(null),opener=useRef<HTMLElement|null>(null),open=dialog!==null;
 const launch=(value:PricingDialog)=>{opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog(value);};
 const search=useCallback(()=>{if(!dialog){opener.current=document.activeElement instanceof HTMLElement?document.activeElement:null;setDialog({kind:'search'});}},[dialog]);
 useLayoutEffect(()=>{const element=root.current;if(!element)return;const before=element.inert;element.inert=open;return()=>{element.inert=before;};},[open]);
 useEffect(()=>{if(open)return;const target=opener.current;opener.current=null;if(target?.isConnected)target.focus();},[open]);
 const changeCapacity=(id:'ultima'|'elite')=>{setComparison(ids=>[...new Set(ids.map(value=>value===capacity?id:value))]);setCapacity(id);};
 const openPlan=(id:SubscriptionPreviewId)=>launch({kind:'plan',id});const openQuote=(enterprise:boolean)=>{const count=parseTeamSeats(seats);if(count!==null)launch({kind:'quote',seats:count,enterprise});};
 return <><div className={styles.page} ref={root} data-pricing-page><a href="#public-main" className={styles.skip}>{t.skip}</a><Link href="/ai" className={styles.announcement}><span>{t.announcementBadge}</span>{t.announcement}<LandingArrow/></Link><LandingHeader navigation={[...aiToolsEn.navigation.slice(0,-1),{label:t.pricingLink,href:"/pricing"}]} labels={{...homeEn.controls,login:homeEn.login,signup:homeEn.signup}} variant="editorial" onSearch={search}/><main id="public-main" className={styles.main}><section className={styles.hero}><h1>{t.title}</h1><p className={styles.intro}>{t.description}</p><div className={styles.controls}><PricingAudience audience={audience} onChange={setAudience}/><PricingPeriodSwitch period={period} onChange={setPeriod}/></div><p className={styles.previewNote}>{copy.pricingNote}</p><div id="pricing-options" role="tabpanel" aria-label={audience==='individual'?t.individual:t.teams}>{audience==='individual'?<PersonalPlans period={period} capacity={capacity} onCapacity={changeCapacity} onPlan={openPlan} onUnits={()=>launch({kind:'units'})}/>:<TeamPlans seats={seats} onSeats={setSeats} onQuote={openQuote}/>}</div><div className={styles.planFootnote}><button type="button" onClick={()=>launch({kind:'units'})}>{t.models}<span aria-hidden="true">↗</span></button><Link href="/legal/paid-access">{t.terms}<span aria-hidden="true">↗</span></Link></div></section><PricingBanner/><PricingComparison period={period} onPeriod={setPeriod} selected={comparison} onSelected={setComparison} onPlan={openPlan} teams={audience==='teams'}/><PricingTools/><PricingStories/><section className={styles.questions} id="questions"><h2>{t.faqTitle}</h2><FAQ items={faq} name="pricing-faq"/></section><section className={styles.help}><h2>{t.help.title}</h2><p>{t.help.description}</p><a href="mailto:support@epifera.com" className={styles.darkButton}>{t.help.action}<span aria-hidden="true">↗</span></a></section></main><LandingFooter {...aiToolsEn.footer} layout="centered"/></div>{dialog?.kind==='search'?<HomeSearch copy={homeEn} onClose={()=>setDialog(null)}/>:dialog?<PricingInformation key={dialog.kind==='plan'?dialog.id:dialog.kind} dialog={dialog} period={period} onClose={()=>setDialog(null)}/>:null}</>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn,t};
