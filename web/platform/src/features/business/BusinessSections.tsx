"use client";
import Image from 'next/image';
import {useState} from 'react';
import {BrandLogo} from '@/components/brand/BrandLogo/BrandLogo';
import {LandingArrow,LandingToolIcon} from '@/components/public/landing/LandingPrimitives';
import Link from '@/i18n/Link';
import {businessEn as t} from '@/i18n/public/business-en';
import {businessReference as media} from '@/i18n/public/business-reference-en';
import styles from './Business.module.css';
import {translatePublicMessage} from '@/i18n/public/public-copy';
import {useLocale} from '@/i18n/LocaleProvider';
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function BusinessTrust(){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"businessEn","media":"businessReference"});
return <section className={styles.trust} aria-labelledby="business-trust"><div className={styles.container}><span className={styles.tag}>{t.trust.label}</span><h2 id="business-trust">{t.trust.title}</h2><p className={styles.sectionIntro}>{t.trust.description}</p><div className={styles.trustGrid}>{t.trust.cards.map((card,index)=><article key={card.title} data-business-trust><span className={styles.trustIcon} aria-hidden="true"><LandingToolIcon name={index===1?'text':index===3?'voice':'spark'}/></span><h3>{card.title}</h3><p>{card.description}</p><Link href={card.href}>{card.action}<LandingArrow/></Link>{index===0?<div className={styles.trustRings} aria-hidden="true"><span><BrandLogo size={58}/></span></div>:null}</article>)}</div></div></section>;}
export function BusinessCredits(){
  const {t} = usePublicCopy(publicCopySourcesForLocale,{"t":"businessEn","media":"businessReference"});
return <section className={styles.credits} aria-labelledby="business-credits"><div className={styles.pool}><div className={styles.poolDots} aria-hidden="true"/><span className={styles.tag}>{t.credits.label}</span><h2 id="business-credits">{t.credits.title}</h2><p>{t.credits.description}</p><div className={styles.poolCenter}><span>{t.credits.centerLabel}</span><strong>{t.credits.amount}</strong><small>{t.credits.status}</small></div><div className={styles.poolRoles} aria-hidden="true">{t.credits.roles.map((role,index)=><span key={role} style={{'--role-index':index} as React.CSSProperties}><i>✧</i>{role}</span>)}</div></div><div className={styles.creditCards}>{t.credits.cards.map((card,index)=><article key={card.title}><LandingToolIcon name={index===1?'image':'spark'}/><h3>{card.title}</h3><p>{card.description}</p></article>)}</div></section>;}

export function BusinessSetup(){
  const publicCopyLocale=useLocale();
  const {t,media} = usePublicCopy(publicCopySourcesForLocale,{"t":"businessEn","media":"businessReference"});
  const steps=[media['step-1'],media['step-2'],media['step-3'],media['step-4']];
const[selected,setSelected]=useState(0);return <section className={styles.setup} aria-labelledby="business-setup"><div className={styles.setupIntro}><span className={styles.tag}>{t.setup.label}</span><h2 id="business-setup">{t.setup.title}</h2><p className={styles.sectionIntro}>{t.setup.description}</p></div><ol>{t.setup.steps.map((step,index)=><li key={step.title} data-selected={selected===index||undefined}><span className={styles.stepMarker} aria-hidden="true">{index+1}</span><div><button type="button" aria-pressed={selected===index} aria-label={translatePublicMessage('Step {index}: {title}',publicCopyLocale,{index:index+1,title:step.title})} onClick={()=>setSelected(index)}><Image src={steps[index].src} fill alt="" sizes="(max-width: 1000px) 80vw, 50vw"/><span><strong>{step.title}</strong><span>{step.description}</span></span></button>{selected===index?<Link href={step.href}>{step.action}<LandingArrow/></Link>:null}</div></li>)}</ol></section>;}

const publicCopySourcesForLocale={t,media};
