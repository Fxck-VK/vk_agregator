"use client";

import { Fragment, type CSSProperties } from "react";
import { imageGeneratorEn } from "@/i18n/public/image-generator-en";
import { imageGeneratorReferenceEn as reference } from "@/i18n/public/image-generator-reference-en";
import Link from "@/i18n/Link";
import styles from "@/components/public/generator-landing/GeneratorLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {translatePublicText} from "@/i18n/public/public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


const copy = imageGeneratorEn.study;
const source = "https://www.magnific.com/ai/image-generator#numbers";
type Chart = NonNullable<(typeof reference.stats)[number]["chart"]>;

function ResearchChart({ chart, grouped }: { chart: Chart; grouped: boolean }) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"imageGeneratorEn":"imageGeneratorEn","reference":"imageGeneratorReferenceEn","copy":"imageGeneratorEn.study"});

  const max = Math.max(...chart.rows.map(row => parseFloat(row.value)));
  return <figure className={styles.studyFigure}><figcaption className={styles.studyFigCaption}>{chart.title}</figcaption>{grouped ? <div className={styles.studySeriesLegend}><span className={styles.studySeriesLegendItem}><span className={`${styles.studySeriesDot} ${styles.studySeriesDotTeam}`} />{copy.team}</span><span className={styles.studySeriesLegendItem}><span className={`${styles.studySeriesDot} ${styles.studySeriesDotIndividual}`} />{copy.individual}</span></div> : null}
    <div className={`${styles.studyBarGrid} ${grouped ? styles.studyBarGridGrouped : ""}`}>{chart.labels.map((label,index) => <Fragment key={label}><div className={styles.studyBarLabel}>{label}</div><div className={styles.studyBarStack}>{chart.rows.slice(grouped ? index*2 : index, grouped ? index*2+2 : index+1).map((row,rowIndex) => <div className={styles.studyBarRow} key={rowIndex}><div className={`${styles.studyBarTrack} ${grouped ? styles.studyBarTrackThin : ""}`}><span data-revealed="true" className={`${styles.studyBarFill} ${rowIndex ? styles.studyBarFillMuted : ""}`} style={{ "--stats-bar-width": `${parseFloat(row.value)/max*100}%` } as CSSProperties} /></div><span className={styles.studyBarValue}>{row.value}</span></div>)}</div></Fragment>)}</div><p className={styles.studyChartNote}>{chart.note}</p>
  </figure>;
}
function ResearchBanner() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"imageGeneratorEn":"imageGeneratorEn","reference":"imageGeneratorReferenceEn","copy":"imageGeneratorEn.study"});

  return <aside className={styles.studyBlockInline}><div><p className={styles.studyBlockEyebrow}>{copy.business.eyebrow}</p><p className={styles.studyBlockInlineHeadline}>{copy.business.title}</p><ul className={styles.studyBlockPoints}>{copy.business.points.map(point => <li key={point}><span className={styles.studyBlockDot} />{point}</li>)}</ul><p className={styles.studyBlockMicro}>{copy.business.note}</p></div><div className={styles.studyBlockInlineCtas}><a href={source} target="_blank" rel="noreferrer" className={styles.studyCtaOutlineDark}>{copy.business.action}</a><Link href="/app/models" className={styles.studyCtaTextDark}>{copy.business.secondary}</Link></div></aside>;
}
function CanvasBanner() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"imageGeneratorEn":"imageGeneratorEn","reference":"imageGeneratorReferenceEn","copy":"imageGeneratorEn.study"});

  return <aside className={styles.studyBlockFeature}><div><p className={styles.studyBlockEyebrowLight}>{copy.canvas.eyebrow}</p><p className={styles.studyBlockFeatureHeadline}>{copy.canvas.title}</p><p className={styles.studyBlockFeatureBody}>{copy.canvas.body}</p><p className={styles.studyBlockFeatureBodyStrong}>{copy.canvas.strong}</p><div className={styles.studyBlockFeatureCtas}><a href={source} target="_blank" rel="noreferrer" className={styles.studyCtaMaroon}>{copy.canvas.action}</a><Link href="#models" className={styles.studyCtaTextMaroon}>{copy.canvas.secondary}</Link></div></div><div className={styles.studyBlockFeatureVisual} aria-hidden="true">{["studyCanvasThumbA","studyCanvasThumbB","studyCanvasThumbHero","studyCanvasThumbC","studyCanvasThumbD","studyCanvasAvatarPurple","studyCanvasAvatarMaroon","studyCanvasAvatarAmber"].map(name => <span key={name} className={styles[name]} />)}</div></aside>;
}
export function ImageGeneratorStudy() {
  const publicCopyLocale=useLocale();

  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"imageGeneratorEn":"imageGeneratorEn","reference":"imageGeneratorReferenceEn","copy":"imageGeneratorEn.study"});

  return <section id="numbers" className={styles.chapter} aria-labelledby="numbers-title"><p className={styles.eyebrow}>{copy.eyebrow}</p><h2 id="numbers-title" className={styles.chapterTitle}>{copy.title}</h2><p className={styles.sourceAttribution}>{copy.source} <a href={source} target="_blank" rel="noreferrer">{copy.link}</a></p><div className={styles.numbersBlock}><div className={`${styles.studyChapter} ${styles.studyChapterEmbedded}`}><div className={styles.studyInner}>
    <div className={styles.studyOpening}><p className={styles.studyHeroFigure}>+60<span className={styles.studyHeroFigureUnit}>%</span></p><div className={styles.studyOpeningBody}><p className={styles.studyKicker}>{copy.title}</p><h3 className={styles.studyTitle}>{copy.title2}</h3></div></div><p className={styles.studyLead}>{copy.lead}</p><p className={styles.studyLead}>{copy.lead2}</p><div className={styles.studyLegend}><span className={`${styles.studyLegendItem} ${styles.studyLegendMeasured}`}>{copy.measured}</span><span className={`${styles.studyLegendItem} ${styles.studyLegendEditorial}`}>{copy.editorial}</span></div><ResearchBanner />
    {reference.stats.map((slot,index) => <Fragment key={slot.number}>{index===0 || index===2 || index===6 ? <p className={styles.studyGroupKicker}>{index===0 ? copy.group1 : index===2 ? copy.group2 : copy.group3}</p> : null}<div className={`${styles.studySlot} ${slot.chart ? styles.studySlotWithChart : ""} ${index===3 ? styles.studySlotPaired : ""}`}><p className={styles.studySlotNumber}>{slot.number}</p><div className={index===3 ? styles.studySlotFigureLayout : undefined}>{index===3 ? <p className={styles.studyInlineFigure}>1<span className={styles.studyInlineFigureConnector}>{translatePublicText("in",publicCopyLocale)}</span>5</p> : null}<div>{index===4 ? <p className={styles.studyBlockFigure}>3×</p> : null}<h3 className={`${styles.studySlotHeading} ${index===1 ? styles.studySlotHeadingLarge : ""}`}>{slot.title}</h3>{slot.claim ? <p className={styles.studyClaim}>{slot.claim}</p> : null}{index===5 ? <div className={styles.studyFigureRow}>{copy.metrics.map(metric => <div key={metric.label}><p className={styles.studyStatValue}>{metric.value}</p><p className={styles.studyStatLabel}>{metric.label}<span className={styles.studyStatUnit}>{metric.unit}</span></p></div>)}</div> : null}<div className={styles.studyAnnotation}>{slot.interpretation.map((paragraph,i) => <Fragment key={paragraph}><p className={styles.studyAnnotationLabel}>{i ? copy.advice : copy.meaning}</p><p className={styles.studyAnnotationText}>{paragraph}</p></Fragment>)}</div>{index===2 ? <p className={styles.studyModelsUpLink}><Link href="#models">{copy.business.secondary}</Link></p> : null}</div></div>{slot.chart ? <ResearchChart chart={slot.chart} grouped={index===0} /> : null}</div>{index===0 ? <CanvasBanner /> : null}</Fragment>)}
  </div></div></div></section>;
}

const publicCopySourcesForLocale={imageGeneratorEn,reference,copy};
