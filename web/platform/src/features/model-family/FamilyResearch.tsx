"use client";
import { Fragment, type CSSProperties } from "react";
import { klingEn as copy } from "@/i18n/public/kling-en";
import { klingReferenceEn as reference } from "@/i18n/public/kling-reference-en";
import { FamilyChapter } from "./FamilySections";
import layout from "@/components/public/generator-landing/GeneratorLanding.module.css";
import styles from "@/components/public/generator-landing/ModelLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function FamilyResearch() {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"copy":"klingEn","reference":"klingReferenceEn"});

  return <FamilyChapter id="numbers" eyebrow={copy.study.eyebrow} title={copy.study.title}><div className={`${layout.numbersBlock} ${styles.research}`}><p className={styles.researchLead}>{copy.study.intro}</p>{reference.stats.map(slot=><div key={slot.number} className={`${layout.studySlot} ${"chart"in slot?layout.studySlotWithChart:""}`}><p className={layout.studySlotNumber}>{slot.number}</p><div><p className={layout.studyBlockFigure}>{slot.figure}</p><h3 className={layout.studySlotHeading}>{slot.title}</h3><p className={layout.studyClaim}>{slot.claim}</p></div>{"chart"in slot?<figure className={layout.studyFigure}><figcaption className={layout.studyFigCaption}>{slot.chart.title}</figcaption><div className={styles.researchBars}>{slot.chart.rows.map(row=><Fragment key={row.label}><span className={layout.studyBarLabel}>{row.label}</span><span className={layout.studyBarRow}><span className={layout.studyBarTrack}><span className={layout.studyBarFill} data-revealed="true" style={{"--stats-bar-width":row.value}as CSSProperties}/></span><span className={layout.studyBarValue}>{row.value}</span></span></Fragment>)}</div>{slot.chart.note?<p className={layout.studyChartNote}>{slot.chart.note}</p>:null}</figure>:null}</div>)}<p className={styles.numbersSource}>{copy.study.source} <a href="https://www.magnific.com/ai/kling#numbers" target="_blank" rel="noreferrer">{copy.study.link}</a></p></div></FamilyChapter>;
}

const publicCopySourcesForLocale={copy,reference};
