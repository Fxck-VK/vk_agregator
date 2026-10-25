"use client";

import { useState } from "react";
import { LandingCarousel } from "@/components/public/landing/LandingCarousel";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { ModelPicture, type ModelImage } from "@/components/public/generator-landing/ModelMedia";
import { OverviewTabs } from "@/features/ai-tools-overview/OverviewPrimitives";
import { marketingEn as copy, type MarketingGuide } from "@/i18n/public/marketing-en";
import { marketingReferenceEn as media } from "@/i18n/public/marketing-reference-en";
import Link from "@/i18n/Link";
import styles from "./MarketingAudience.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export type MarketingPreview = MarketingGuide & { readonly image?: ModelImage };

export function MarketingExamples({onOpen}:{onOpen:(item:MarketingPreview)=>void}) {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);

  const [category,setCategory]=useState(0);
  return <section id="workflows" className={styles.workflows}><div className={styles.heading}><h2>{copy.workflows.title}</h2><p>{copy.workflows.description}</p></div><div className={styles.filterBar}><div role="group" aria-label={copy.workflows.filters} className={styles.filters}>{copy.workflows.categories.map((item,index)=><button key={item} type="button" aria-pressed={category===index} onClick={()=>setCategory(index)}>{item}</button>)}</div><Link href="/app/inspiration" className={styles.viewAll}>{copy.workflows.viewAll}<LandingArrow/></Link></div>
    <div className={styles.examples}>{copy.workflows.examples.map((item,index)=>category===0||copy.workflows.categories[category]===item.category?<button type="button" key={item.title} className={styles.example} onClick={()=>onOpen({...item,image:media.examples[index]})} data-marketing-example><span className={styles.exampleImage}><ModelPicture image={media.examples[index]} assetBase="marketing-audience" sizes="(max-width:767px) 80vw,300px" decorative/></span><h3>{item.title}</h3></button>:null)}</div>
  </section>;
}

export function MarketingNeeds() {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);

  const [selected,setSelected]=useState<string>(copy.needs.items[0].id);
  return <section id="marketing-needs" className={styles.needs}><h2>{copy.needs.title}</h2><OverviewTabs prefix="marketing-need" label={copy.needs.label} items={copy.needs.items} selected={selected} onChange={setSelected} className={styles.tabs}/>
    {copy.needs.items.map((item,index)=><div key={item.id} id={`marketing-need-panel-${item.id}`} role="tabpanel" aria-labelledby={`marketing-need-tab-${item.id}`} hidden={selected!==item.id} tabIndex={0} className={styles.needPanel}><div className={styles.needText}><h3>{item.title}</h3><p>{item.description}</p></div><div className={styles.needImage}><ModelPicture image={media.panels[index]} assetBase="marketing-audience" sizes="(max-width:767px) 90vw,46vw"/></div></div>)}
  </section>;
}

export function MarketingGuides({onOpen}:{onOpen:(item:MarketingPreview)=>void}) {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="marketing-guides" className={styles.guides}><div className={styles.heading}><h2>{copy.guides.title}</h2><p>{copy.guides.description}</p></div><LandingCarousel label={copy.guides.label} previous={copy.guides.previous} next={copy.guides.next} bounded className={styles.cardViewport}>{copy.guides.items.map((item,index)=><button type="button" key={item.title} className={styles.guide} onClick={()=>onOpen({...item,image:media.guides[index]})} data-marketing-guide><span className={styles.guideImage}><ModelPicture image={media.guides[index]} assetBase="marketing-audience" sizes="(max-width:767px) 80vw,300px" decorative/><span className={styles.guideOpen} aria-hidden="true">↗</span></span><h3>{item.title}</h3></button>)}</LandingCarousel></section>;
}

export function MarketingTools({onOpen}:{onOpen:(item:MarketingPreview)=>void}) {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);

  return <section id="marketing-tools" className={styles.tools}><div className={styles.heading}><h2>{copy.tools.title}</h2><p>{copy.tools.description}</p></div><LandingCarousel label={copy.tools.label} previous={copy.tools.previous} next={copy.tools.next} bounded className={styles.cardViewport}>{copy.tools.items.map((item,index)=>{
    const content=<><ModelPicture image={media.tools[index]} assetBase="marketing-audience" sizes="(max-width:767px) 80vw,300px" decorative/><div className={styles.toolText}><h3>{item.title}</h3><p>{item.description}</p>{"planned"in item&&item.planned?<span>{copy.comingSoon}</span>:null}</div></>;
    return "href"in item&&item.href?<Link href={item.href} key={item.title} className={styles.tool} data-marketing-tool>{content}</Link>:<button type="button" key={item.title} className={styles.tool} onClick={()=>onOpen({...item,steps:[],image:media.tools[index]})} data-marketing-tool>{content}</button>;
  })}</LandingCarousel></section>;
}

const publicCopySourcesForLocale={copy,media};
