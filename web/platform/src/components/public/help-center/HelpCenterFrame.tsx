"use client";
import {Fragment,useEffect,useId,useRef,useState} from "react";
import {BrandLogo} from "@/components/brand/BrandLogo/BrandLogo";
import {MenuIcon} from "@/components/icons/MenuIcon";
import {brandName} from "@/config/brand";
import Link from "@/i18n/Link";
import {HelpCenterDrawer} from "./HelpCenterDrawer";
import {HelpIcon} from "./HelpIcon";
import type {HelpCenterFrameProps,HelpRailItem} from "./types";
import styles from "./HelpCenter.module.css";

function RailLinks({items,label,rail=false,onNavigate}:{items:readonly HelpRailItem[];label:string;rail?:boolean;onNavigate?:()=>void}){
  return <nav className={rail?styles.railLinks:styles.drawerLinks} aria-label={label}>{items.map((item,index)=><Fragment key={`${item.href}-${index}`}>{item.bottom&&!items[index-1]?.bottom?<div className={styles.railSpacer}/>:null}<Link href={item.href} aria-label={item.label} title={rail?item.label:undefined} aria-current={item.current?"location":undefined} className={item.icon==="plus"?styles.createLink:undefined} onClick={onNavigate}>{item.icon==="plus"?<span className={styles.plus} aria-hidden="true"/>:<HelpIcon kind={item.icon}/>}<span className={styles.railLabel}>{item.label}</span></Link></Fragment>)}</nav>;
}
export function HelpCenterFrame({kind,labels:t,rail,breadcrumbs,readingKey,mainClassName,hideMobileBreadcrumb=false,utilityAction,renderNavigation,children}:HelpCenterFrameProps){
  const [drawer,setDrawer]=useState<"global"|"section"|null>(null),drawerId=useId();
  const reading=useRef<HTMLDivElement>(null);
  useEffect(()=>{reading.current?.scrollTo?.({top:0});},[readingKey]);
  useEffect(()=>{const media=window.matchMedia?.("(min-width: 1024px)");const close=()=>{if(media?.matches)setDrawer(null);};media?.addEventListener("change",close);return()=>media?.removeEventListener("change",close);},[]);
  return <><div className={styles.page} data-help-center={kind} data-legal-page={kind==="legal"||undefined}>
    <a className={styles.skip} href="#public-main">{t.skip}</a><aside className={styles.rail}><Link href="/" className={styles.brand} aria-label={brandName}><BrandLogo size={32}/></Link><RailLinks items={rail} label={t.navigation} rail/></aside>
    <div className={styles.workspace}><header className={styles.topbar}>
      <button type="button" className={`${styles.iconButton} ${styles.mobileMenu}`} aria-label={t.openNavigation} aria-expanded={drawer==="global"} aria-controls={drawer==="global"?drawerId:undefined} onClick={()=>setDrawer("global")}><MenuIcon/></button>
      <nav className={`${styles.breadcrumb} ${hideMobileBreadcrumb?styles.hideMobileBreadcrumb:""}`} aria-label={t.breadcrumb}>{breadcrumbs.map((crumb,index)=><Fragment key={`${index}-${crumb.label}`}>{index?<span className={styles.breadcrumbArrow} aria-hidden="true">›</span>:null}{crumb.href?<Link href={crumb.href}>{crumb.label}</Link>:<span className={styles.currentCrumb} aria-current={index===breadcrumbs.length-1?"page":undefined}>{crumb.label}</span>}</Fragment>)}</nav>
      <div className={styles.topbarActions}><Link href="/app" className={styles.workspaceLink}>{t.workspace}</Link>{utilityAction??<a className={`${styles.iconButton} ${styles.supportButton}`} href="mailto:support@epifera.com" aria-label={t.support}><HelpIcon kind="support"/></a>}<Link href="/login" className={styles.login}>{t.login}</Link><Link href="/login?mode=register" className={styles.signup}>{t.signup}</Link></div>
    </header><div className={styles.body}><aside className={styles.sidebar}>{renderNavigation()}</aside><div className={styles.readingPanel} ref={reading} data-help-reading>
      <div className={styles.documentToolbar}><button type="button" className={styles.iconButton} aria-label={t.openSectionMenu} aria-expanded={drawer==="section"} aria-controls={drawer==="section"?drawerId:undefined} onClick={()=>setDrawer("section")}><MenuIcon/></button></div>
      <main className={`${styles.main} ${mainClassName??""}`} id="public-main" tabIndex={-1}>{children}</main>
    </div></div></div>
  </div>{drawer?<HelpCenterDrawer id={drawerId} title={drawer==="section"?t.sectionTitle:t.navigation} closeLabel={t.closeMenu} testId={`${kind}-navigation-backdrop`} onClose={()=>setDrawer(null)}>{onNavigate=>drawer==="section"?<>{renderNavigation(onNavigate)}<a className={styles.drawerSupport} href="mailto:support@epifera.com" onClick={onNavigate}>{t.support}</a></>:<RailLinks items={rail} label={t.navigation} onNavigate={onNavigate}/>}</HelpCenterDrawer>:null}</>;
}
