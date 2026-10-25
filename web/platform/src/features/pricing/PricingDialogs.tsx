"use client";
import {useState} from "react";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {CreditAmount} from "@/components/ui/CreditAmount/CreditAmount";
import {pricingEn as t} from "@/i18n/public/pricing-en";
import Link from "@/i18n/Link";
import {subscriptionPlanPreview,type SubscriptionPreviewId} from "./subscription-preview";
import {budgetFor,formatPricingAmount,buildPricingEnquiry,type PricingPeriod} from "./pricing-model";
import styles from "./Pricing.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export type PricingDialog={kind:"search"}|{kind:"units"}|{kind:"plan";id:SubscriptionPreviewId}|{kind:"quote";seats:number;enterprise:boolean};
export function PricingInformation({dialog,period,onClose}:{dialog:Exclude<PricingDialog,{kind:"search"}>;period:PricingPeriod;onClose:()=>void}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale);

 const[copyState,setCopyState]=useState<"idle"|"copied"|"error">("idle"),plan=dialog.kind==='plan'?subscriptionPlanPreview(dialog.id):null,trial=subscriptionPlanPreview('lite'),quote=dialog.kind==='quote'?buildPricingEnquiry(dialog.seats,dialog.enterprise,publicCopyLocale):"";
 const copy=async()=>{try{await navigator.clipboard.writeText(quote);setCopyState('copied');}catch{setCopyState('error');}};
 const title=dialog.kind==='quote'?(dialog.enterprise?t.team.enterpriseDialog:t.team.dialog):dialog.kind==='plan'?`${plan!.name}${translatePublicText(" subscription preview",publicCopyLocale)}`:t.modelsTitle;
 return <LandingDialog title={title} closeLabel={t.close} onClose={onClose}><div className={styles.dialogContent}>{dialog.kind==='plan'&&plan?<><span className={styles.previewBadge}>{t.preview}</span><p className={styles.dialogPrice}>₽{formatPricingAmount(budgetFor(plan,period)!,publicCopyLocale)} /{translatePublicText(period==='yearly'?'year estimate':plan.period,publicCopyLocale)}</p><CreditAmount value={plan.stars}/><p>{t.detailsNote}</p>{period==='yearly'?<p>{t.annualDetails}</p>:null}<p>{t.finalAuthority}</p><div className={styles.dialogLinks}><Link href="/app">{t.openWorkspace}</Link><Link href="/legal/paid-access">{t.terms}</Link></div></>:dialog.kind==='units'?<><p>{t.modelsNote}</p><h3>{trial.name}{translatePublicText(" trial preview",publicCopyLocale)}</h3><p>₽{formatPricingAmount(trial.amount,publicCopyLocale)}{translatePublicText(" /week · ",publicCopyLocale)}<CreditAmount value={trial.stars}/></p><p>{t.trialNote}</p><p>{t.finalAuthority}</p><div className={styles.dialogLinks}><Link href="/app/models">{translatePublicText("Explore available models",publicCopyLocale)}</Link><Link href="/legal/paid-access">{t.terms}</Link></div></>:dialog.kind==='quote'?<><span className={styles.previewBadge}>{t.team.status}</span><p><strong>{new Intl.NumberFormat(publicCopyLocale).format(dialog.seats)}{translatePublicText(" people",publicCopyLocale)}</strong> · {t.team.price}</p><p>{t.team.note}</p><pre className={styles.quoteText}>{quote}</pre><button type="button" className={styles.darkButton} onClick={()=>void copy()}>{t.team.copy}</button><p role="status">{copyState==='copied'?t.team.copied:copyState==='error'?t.team.copyError:""}</p><div className={styles.dialogLinks}><a href="mailto:support@epifera.com">{t.team.contact}</a><Link href="/legal/payment-details">{translatePublicText("Payment details",publicCopyLocale)}</Link></div></>:null}</div></LandingDialog>;
}

const publicCopySourcesForLocale={t};
