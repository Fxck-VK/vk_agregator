"use client";
import type {ReactNode} from "react";
import {BrandLogo} from "@/components/brand/BrandLogo/BrandLogo";
import {HelpIcon} from "@/components/public/help-center/HelpIcon";
import {LandingDialog} from "@/components/public/landing/LandingDialog";
import {brandName} from "@/config/brand";
import {referenceEn as t} from "@/i18n/public/reference-en";
import Link from "@/i18n/Link";
import styles from "./ReferenceChrome.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export type ReferenceCurrent={label:string;mobileLabel:string;searchLabel:string;searchButtonLabel:string};
export function ReferenceChrome({search,current,onSearch,onTools}:{search:ReactNode;current:ReferenceCurrent;onSearch:()=>void;onTools:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  return <><aside className={styles.rail}><Link className={styles.brand} href="/" aria-label={brandName}><BrandLogo size={32}/></Link><nav aria-label={t.navigation}><Link href="/app/image" className={styles.create} aria-label={t.create}><span aria-hidden="true">+</span></Link><Link href="/" aria-label={t.home} title={t.home}><HelpIcon kind="home"/></Link><button type="button" aria-label={current.searchButtonLabel} title={current.searchLabel} onClick={onSearch}><HelpIcon kind="search"/></button><a href="#public-main" aria-label={current.label} title={current.label} aria-current="page"><HelpIcon kind="grid"/></a><button type="button" aria-label={t.openTools} title={t.tools} onClick={onTools}><HelpIcon kind="grid"/></button><Link href="/workflows/product-campaign" aria-label={t.workflows} title={t.workflows}><HelpIcon kind="file"/></Link><Link href="/app/image" aria-label={t.imageGenerator} title={t.imageGenerator}><HelpIcon kind="grid"/></Link><Link href="/app/models" aria-label={t.videoGenerator} title={t.videoGenerator}><HelpIcon kind="play"/></Link><Link href="/app/music" aria-label={t.musicGenerator} title={t.musicGenerator}><HelpIcon kind="play"/></Link><Link className={styles.railBottom} href="/academy/courses/epifera-essentials" aria-label={t.academy} title={t.academy}><HelpIcon kind="book"/></Link><Link href="/ai/docs" aria-label={t.help} title={t.help}><HelpIcon kind="support"/></Link></nav></aside>
  <header className={styles.topbar}><Link className={styles.mobileBrand} href="/" aria-label={brandName}><BrandLogo size={32}/></Link><div className={styles.desktopSearch}>{search}</div><div className={styles.auth}><Link className={styles.pricing} href="/pricing">{t.pricing}</Link><Link className={styles.login} href="/login">{t.login}</Link><Link className={styles.signup} href="/login?mode=register">{t.signup}</Link></div></header>
  <nav className={styles.mobileNav} aria-label={t.quickNav}><button type="button" onClick={onSearch} aria-label={current.searchLabel}><HelpIcon kind="search"/><span>{t.searchShort}</span></button><a href="#public-main" aria-current="page"><HelpIcon kind="grid"/><span>{current.mobileLabel}</span></a><Link href="/app/files"><HelpIcon kind="file"/><span>{t.files}</span></Link><button type="button" onClick={onTools} aria-label={t.browseTools}><HelpIcon kind="grid"/><span>{t.toolsShort}</span></button><Link href="/pricing"><HelpIcon kind="level"/><span>{t.pricing}</span></Link></nav></>;
}
export function ReferenceTools({onClose}:{onClose:()=>void}){
  const {t} = usePublicCopy(publicCopySourcesForLocale);

  return <LandingDialog title={t.tools} closeLabel={t.close} onClose={onClose}><nav className={styles.toolLinks} aria-label={t.tools}>{t.links.map(item=><Link href={item.href} key={item.label}><HelpIcon kind={item.icon}/><span>{item.label}</span><span aria-hidden="true">↗</span></Link>)}</nav></LandingDialog>;
}

const publicCopySourcesForLocale={t};
