"use client";
import {useCallback,useState} from "react";
import {BrandLogo} from "@/components/brand/BrandLogo/BrandLogo";
import {LandingHeader} from "@/components/public/landing/LandingHeader";
import {LandingFooter} from "@/components/public/landing/LandingFooter";
import {Tooltip} from "@/components/ui/Tooltip/Tooltip";
import {HomeSearch} from "@/features/public-home/HomeSearch";
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import Link from "@/i18n/Link";
import {EditorialBackToTop,EditorialIcon,EditorialListen,EditorialOutline,EditorialShare} from "./EditorialControls";
import {EditorialPicture} from "./EditorialPicture";
import type {EditorialPageProps} from "./types";
import styles from "./Editorial.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";

export function EditorialPage({page:sourcePage,children}:EditorialPageProps) {
  const {aiToolsEn,homeEn} = usePublicCopy(publicCopySourcesForLocale);
  const page=usePublicCopy(sourcePage);

  const navigation=[aiToolsEn.navigation[0],{label:page.controls.tools,href:"/ai"},aiToolsEn.navigation[2],aiToolsEn.navigation[4]];
  const [search,setSearch]=useState(false),[dialog,setDialog]=useState<"share"|"listen"|null>(null);
  const openSearch=useCallback(()=>{if(!dialog)setSearch(true);},[dialog]);
  const titleId=`${page.id}-title`,relatedId=`${page.id}-related`;
  return <div className={styles.page} data-editorial-page={page.kind} data-comparison-page={page.kind==="comparison"||undefined}>
    <a href="#public-main" className={styles.skip}>{page.skip}</a>
    <LandingHeader variant="editorial" siteAction={{label:page.controls.home,href:"/"}} navigation={navigation} labels={{...homeEn.controls,search:page.controls.search,login:homeEn.login,signup:page.controls.openWorkspace}} onSearch={openSearch}/>
    <main id="public-main"><section className={styles.hero} aria-labelledby={titleId}>
      <div className={styles.heroContent}><div className={styles.heroHeading}>
        <nav aria-label={page.controls.breadcrumb} className={styles.breadcrumb}>{page.breadcrumbs.map((crumb,index)=><span key={`${index}-${crumb.label}`} className={styles.crumb}>{index?<span aria-hidden="true">/</span>:null}{crumb.href?<Link href={crumb.href}>{crumb.label}</Link>:<span aria-current={index===page.breadcrumbs.length-1?"page":undefined}>{crumb.label}</span>}</span>)}</nav>
        <h1 id={titleId}>{page.title}</h1></div>
        <div className={styles.byline}><div className={styles.author}><span className={styles.authorIcon}><BrandLogo size={30}/></span><div><Link href={page.author.href}>{page.author.label}</Link><time dateTime={page.author.date}>{page.author.dateLabel}</time></div></div>
          <div className={styles.reading}><span>{page.minutes} {page.readTime}</span><Tooltip label={page.controls.comingSoon} placement="bottom"><button type="button" aria-label={page.controls.listen} onClick={()=>setDialog("listen")}><EditorialIcon kind="listen"/></button></Tooltip><button type="button" aria-label={page.controls.share} onClick={()=>setDialog("share")}><EditorialIcon kind="share"/></button></div></div>
      </div><div className={styles.heroMedia}><figure><EditorialPicture image={page.hero} sizes="(max-width:767px) 95vw,45vw" preload/><figcaption>{page.source}</figcaption></figure></div>
    </section>
      {page.intro?<div className={styles.intro}><p>{page.intro}</p></div>:null}
      <div className={styles.layout}><aside className={styles.sidebar}><EditorialOutline entries={page.sections} label={page.onPage}/><Link href={page.inspiration.href} className={styles.inspiration}><EditorialPicture image={page.inspiration.image} sizes="300px" decorative/><span><strong>{page.inspiration.title}</strong><span>{page.inspiration.action}</span></span></Link></aside>{children}</div>
      <div className={styles.shareBar}><button type="button" aria-label={page.controls.shareLink} onClick={()=>setDialog("share")}><EditorialIcon kind="share"/>{page.controls.shareLink}</button></div>
      <section className={styles.related} aria-labelledby={relatedId}><h2 id={relatedId}>{page.related.title}</h2><div>{page.related.items.map(item=><Link href={item.href} key={item.href} className={styles.relatedCard} data-related-guide><span className={styles.relatedImage}><EditorialPicture image={item.image} sizes="(max-width:767px) 90vw,30vw" decorative/></span><span className={styles.relatedCategory}>{item.category}</span><h3>{item.title}</h3><p>{item.description}</p></Link>)}</div><p className={styles.sourceNote}>{page.source}</p></section>
    </main><LandingFooter {...aiToolsEn.footer} layout="centered"/>
    {search?<HomeSearch copy={homeEn} onClose={()=>setSearch(false)}/>:null}
    {dialog==="share"?<EditorialShare labels={page.controls} onClose={()=>setDialog(null)}/>:dialog==="listen"?<EditorialListen labels={page.controls} onClose={()=>setDialog(null)}/>:null}
    <EditorialBackToTop label={page.controls.backTop}/>
  </div>;
}

const publicCopySourcesForLocale={homeEn,aiToolsEn};
