"use client";
import {useState} from "react";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {CreditAmount} from "@/components/ui/CreditAmount/CreditAmount";
import {pricingEn as t} from "@/i18n/public/pricing-en";
import Link from "@/i18n/Link";
import {subscriptionPlanPreview,type SubscriptionPreviewId} from "./subscription-preview";
import {formatPricingAmount,buildPricingEnquiry,type PricingPeriod} from "./pricing-model";
import styles from "./Pricing.module.css";
import {SubscriptionCheckout} from "@/features/subscriptions/SubscriptionCheckout";
import {subscriptionCopy} from "@/features/subscriptions/copy";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export type PricingDialog={kind:"search"}|{kind:"units"}|{kind:"plan";id:SubscriptionPreviewId}|{kind:"quote";seats:number;enterprise:boolean};
export function PricingInformation({dialog,onClose}:{dialog:Exclude<PricingDialog,{kind:"search"}>;period:PricingPeriod;onClose:()=>void}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"pricingEn"});

 const[copyState,setCopyState]=useState<"idle"|"copied"|"error">("idle"),plan=dialog.kind==='plan'?subscriptionPlanPreview(dialog.id):null,trial=subscriptionPlanPreview('lite'),quote=dialog.kind==='quote'?buildPricingEnquiry(dialog.seats,dialog.enterprise,publicCopyLocale):"";
 const copy=async()=>{try{await navigator.clipboard.writeText(quote);setCopyState('copied');}catch{setCopyState('error');}};
 const title=dialog.kind==='quote'?(dialog.enterprise?t.team.enterpriseDialog:t.team.dialog):dialog.kind==='plan'?`${plan!.name}${translatePublicText(" subscription preview",publicCopyLocale)}`:t.modelsTitle;
 return <LandingDialog title={title} closeLabel={t.close} onClose={onClose}><div className={styles.dialogContent}>{dialog.kind==='plan'&&plan?<><SubscriptionCheckout key={plan.id} planID={plan.id}/><div className={styles.dialogLinks}><Link href="/app">{t.openWorkspace}</Link><Link href="/legal/paid-access">{t.terms}</Link></div></>:dialog.kind==='units'?<><p>{subscriptionCopy[publicCopyLocale].costs}</p><h3>{trial.name}{translatePublicText(" trial preview",publicCopyLocale)}</h3><p>₽{formatPricingAmount(trial.amount,publicCopyLocale)} / {trial.period_days} {subscriptionCopy[publicCopyLocale].days} · <CreditAmount value={trial.stars}/></p><p>{subscriptionCopy[publicCopyLocale].trial}</p><p>{t.finalAuthority}</p><div className={styles.dialogLinks}><Link href="/app/models">{translatePublicText("Explore available models",publicCopyLocale)}</Link><Link href="/legal/paid-access">{t.terms}</Link></div></>:dialog.kind==='quote'?<><span className={styles.previewBadge}>{t.team.status}</span><p><strong>{new Intl.NumberFormat(publicCopyLocale).format(dialog.seats)}{translatePublicText(" people",publicCopyLocale)}</strong> · {t.team.price}</p><p>{t.team.note}</p><pre className={styles.quoteText}>{quote}</pre><button type="button" className={styles.darkButton} onClick={()=>void copy()}>{t.team.copy}</button><p role="status">{copyState==='copied'?t.team.copied:copyState==='error'?t.team.copyError:""}</p><div className={styles.dialogLinks}><a href="mailto:support@epifera.com">{t.team.contact}</a><Link href="/legal/payment-details">{translatePublicText("Payment details",publicCopyLocale)}</Link></div></>:null}</div></LandingDialog>;
}

const publicCopySourcesForLocale={t};
