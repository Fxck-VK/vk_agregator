"use client";
import {useState} from 'react';
import {LandingDialog} from '@/components/public/landing/LandingDialog';
import Link from '@/i18n/Link';
import {businessEn as t} from '@/i18n/public/business-en';
import styles from './Business.module.css';
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText,translatePublicMessage} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export type BusinessDialog={kind:'search'}|{kind:'guide'}|{kind:'quote';seats:number;annual:boolean};
export function BusinessInformation({dialog,onClose}:{dialog:Exclude<BusinessDialog,{kind:'search'}>;onClose:()=>void}){
  const publicCopyLocale=useLocale();

  const {t} = usePublicCopy(publicCopySourcesForLocale);

 const[state,setState]=useState<'idle'|'copied'|'error'>('idle'),quote=dialog.kind==='quote'?translatePublicMessage('Business enquiry for Epifera\nTeam size: {seats} people\nBilling preference: {period}\nPrice to be agreed\nPlease discuss tools, access, shared capacity, review responsibilities and support requirements.',publicCopyLocale,{seats:dialog.seats,period:translatePublicText(dialog.annual?'Annual':'Monthly',publicCopyLocale)}):'';
 const copy=async()=>{try{await navigator.clipboard.writeText(quote);setState('copied');}catch{setState('error');}};
 return <LandingDialog title={dialog.kind==='quote'?t.quote.title:t.pricing.guideTitle} closeLabel={t.close} onClose={onClose}><div className={styles.dialogCopy}><span className={styles.dialogBadge}>{t.label}</span>{dialog.kind==='quote'?<><p>{t.quote.description}</p><p><strong>{dialog.seats} {translatePublicText(" people",publicCopyLocale)}</strong> · {t.pricing.price}</p><pre>{quote}</pre><button type="button" className={styles.dialogButton} onClick={()=>void copy()}>{t.quote.copy}</button><p role="status">{state==='copied'?t.quote.copied:state==='error'?t.quote.copyError:''}</p><div><a href="mailto:support@epifera.com">{t.quote.contact}</a><Link href="/legal/paid-access">{t.quote.terms}</Link></div></>:<><p>{t.pricing.guideNote}</p><ul>{t.features.map(feature=><li key={feature.id}><strong>{feature.label} · {feature.status}</strong><p>{feature.description}</p></li>)}</ul><div><Link href="/app/models">{t.controls.models}</Link><Link href="/pricing">{t.controls.personalPricing}</Link><Link href="/mcp">{t.controls.integration}</Link></div></>}</div></LandingDialog>;
}

const publicCopySourcesForLocale={t};
