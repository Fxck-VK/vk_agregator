"use client";

import { useState } from "react";
import { GeneratorChapter as FamilyChapter } from "@/components/public/generator-landing/GeneratorChapter";
export { GeneratorChapter as FamilyChapter } from "@/components/public/generator-landing/GeneratorChapter";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { klingEn as copy } from "@/i18n/public/kling-en";
import { klingCatalogEn as catalog } from "@/i18n/public/kling-catalog-en";
import { klingReferenceEn as reference } from "@/i18n/public/kling-reference-en";
import Link from "@/i18n/Link";
import { FamilyPicture, FamilyVideo } from "./FamilyMedia";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import styles from "@/components/public/generator-landing/ModelLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";



export const modelHref=(id:string)=>`/app?model=${encodeURIComponent(id)}`;
export function FamilyVersions({onPreview}:{onPreview:(title:string)=>void}) {
  const publicCopyLocale=useLocale();

  const {copy,catalog} = usePublicCopy(publicCopySourcesForLocale);

  return <FamilyChapter id="versions" {...copy.versions}><div className={layout.modelsTableCard}><div className={layout.modelsTableScroll} role="region" aria-label={translatePublicText("Scrollable Kling specifications",publicCopyLocale)} tabIndex={0}><table className={`${layout.modelsTable} ${styles.specTable} ${styles.versionsTable}`}><caption className={layout.srOnly}>{copy.versions.caption}</caption><thead><tr>{copy.versions.columns.map((column,index)=><th scope="col" key={column}>{index===7?<span className={layout.srOnly}>{column}</span>:column}</th>)}</tr></thead><tbody>{copy.versions.rows.map(row=>{
    const current=catalog.find(item=>item.id===row.id);
    return <tr key={row.name}><th scope="row">{row.id?<Link href={row.id==="video_kling_v3"?"/ai/kling/kling-3-0":modelHref(row.id)} className={styles.versionName}>{row.name}</Link>:<button type="button" className={`${styles.versionName} ${styles.versionButton}`} aria-label={`${translatePublicText("About ",publicCopyLocale)}${row.name}`} onClick={()=>onPreview(row.name)}>{row.name}</button>}<span className={styles.cellNote}>{row.note}</span></th><td>{current?.inputs??"—"}</td><td className={styles.mono}>{current?.duration??"—"}</td><td className={styles.mono}>{current?.resolution??"—"}</td><td>{current?.audio??"—"}</td><td>{current?<><span className={styles.cellNote}>{translatePublicText("Estimate in workspace",publicCopyLocale)}</span></>:"—"}</td><td><span className={`${styles.status} ${current?styles.statusLatest:""}`}>{translatePublicText(current?"In catalog":"Not listed",publicCopyLocale)}</span></td><td>{row.id?<Link href={modelHref(row.id)} className={styles.tryLink} aria-label={`${translatePublicText("View ",publicCopyLocale)}${row.name}`}>{translatePublicText("View ",publicCopyLocale)}<span aria-hidden="true">→</span></Link>:<button type="button" className={styles.previewLink} aria-label={`${translatePublicText("Details for ",publicCopyLocale)}${row.name}`} onClick={()=>onPreview(row.name)}>{translatePublicText("Details →",publicCopyLocale)}</button>}</td></tr>;
  })}</tbody></table></div></div><p className={styles.tableNote}>{copy.versions.note}</p></FamilyChapter>;
}
export function FamilyChoose() {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <FamilyChapter id="choose" {...copy.choose}><div className={`${layout.useCaseGrid} ${styles.chooseGrid}`}>{copy.choose.cards.map((card,index)=><article className={layout.useCaseCard} key={card.title}><div className={layout.useCaseImgWrap}><FamilyPicture image={reference.choose[index]} className={layout.useCaseImg}/><div className={layout.useCaseChips}><span className={layout.useCaseChip}>{card.model}</span></div></div><div className={layout.useCaseBody}><h3 className={layout.useCaseH3}>{card.title}</h3><p className={layout.useCaseLine}>{card.body}</p><Link href={"href"in card?card.href:modelHref(card.id)} className={styles.chooseCta}>{card.action}<LandingArrow/></Link></div></article>)}</div></FamilyChapter>;
}
export function FamilyTimeline({onPreview}:{onPreview:(title:string)=>void}) {
  const publicCopyLocale=useLocale();

  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <FamilyChapter id="whats-new" {...copy.timeline}><ol className={styles.timeline}>{copy.timeline.items.map(item=><li key={item.name} className={styles.timelineItem}><h3 className={styles.timelineName}>{item.name}</h3><p className={styles.timelineBody}>{item.body}</p>{item.id?<Link href={item.id==="video_kling_v3"?"/ai/kling/kling-3-0":modelHref(item.id)} className={styles.timelineLink}>{translatePublicText("Explore ",publicCopyLocale)}{item.name} →</Link>:<button type="button" className={`${styles.timelineLink} ${styles.previewLink}`} onClick={()=>onPreview(item.name)}>{translatePublicText("About ",publicCopyLocale)}{item.name} →</button>}</li>)}</ol></FamilyChapter>;
}
function ComparisonClip({index,paused}:{index:number;paused:boolean}) {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  const [muted,setMuted]=useState(true);
  const clip=copy.compare.clips[index];
  return <li data-comparison-clip><figure className={styles.output}><div className={styles.outputMedia}><FamilyVideo clip={reference.compare[index]} paused={paused} muted={muted} controls/><span className={`${styles.status} ${styles.outputChip}`}>{clip.name}</span><button type="button" className={styles.soundToggle} aria-label={`${copy.controls.sound} ${clip.name}`} aria-pressed={!muted} onClick={()=>setMuted(!muted)}><span aria-hidden="true">{muted?"♩":"♫"}</span></button></div><figcaption className={styles.outputCaption}>{clip.caption}</figcaption></figure></li>;
}
export function FamilyComparison({paused}:{paused:boolean}) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale);

  return <FamilyChapter id="compare" eyebrow={copy.compare.eyebrow} title={copy.compare.title}><div className={styles.compareBlock}><h3 className={styles.compareTitle}>{copy.compare.subtitle}</h3><p className={styles.compareIntro}>{copy.compare.intro}</p><blockquote className={styles.brief}><span className={styles.briefLabel}>{copy.compare.briefLabel}</span>{copy.compare.brief}</blockquote><ul className={styles.triptych}>{copy.compare.clips.map((clip,index)=><ComparisonClip key={clip.name} index={index} paused={paused}/>)}</ul></div></FamilyChapter>;
}
export function FamilyWorkflow({onPreview}:{onPreview:(title:string)=>void}) {
  const publicCopyLocale=useLocale();
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale);

  return <FamilyChapter id="in-epifera" {...copy.workflow}><div className={layout.workflowCards}>{copy.workflow.cards.map((card,index)=><article className={layout.workflowCard} key={card.title}><div className={layout.workflowCardImg}><FamilyPicture image={reference.workflow[index]} sizes="(max-width: 820px) 90vw, 25vw"/></div><div className={layout.workflowCardBodyWrap}><h3 className={layout.workflowCardTitle}>{card.title}</h3><p className={layout.workflowCardBody}>{card.body}</p><div className={layout.useCaseLinks}>{"href"in card?<Link href={card.href} className={layout.useCaseLink}>{card.action}<LandingArrow/></Link>:<button type="button" className={styles.previewLink} onClick={()=>onPreview(translatePublicText("Epifera connections",publicCopyLocale))}>{card.action} →</button>}</div></div></article>)}</div></FamilyChapter>;
}

const publicCopySourcesForLocale={copy,catalog,reference};
