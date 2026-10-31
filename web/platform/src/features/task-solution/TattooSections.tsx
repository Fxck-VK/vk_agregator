"use client";

import { useRef, useState } from "react";
import { FAQ } from "@/components/ui/FAQ/FAQ";
import { LandingCarousel } from "@/components/public/landing/LandingCarousel";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { LandingPlanCards } from "@/components/public/landing/LandingPlanCards";
import { tattooEn as copy } from "@/i18n/public/tattoo-en";
import { tattooReferenceEn as media } from "@/i18n/public/tattoo-reference-en";
import type { OverviewAction } from "@/i18n/public/ai-tools-en";
import Link from "@/i18n/Link";
import { TattooPicture } from "./TattooPicture";
import styles from "./TattooSolution.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function TattooHowTo() {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  const [selected, setSelected] = useState(0);
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  return <section id="how-it-works" className={styles.how}><h2>{copy.how.title}</h2><div className={styles.howBoard}>
    <div className={styles.steps}>{copy.how.steps.map((step, index) => <div key={step.title} className={styles.step}><h3><button ref={element => { refs.current[index] = element; }} type="button" id={`tattoo-step-${index}`} aria-expanded={selected === index} aria-controls={`tattoo-step-panel-${index}`} onClick={() => setSelected(index)} onKeyDown={event => {
      const last = copy.how.steps.length - 1;
      const target = event.key === "Home" ? 0 : event.key === "End" ? last : event.key === "ArrowDown" ? (index + 1) % (last + 1) : event.key === "ArrowUp" ? (index + last) % (last + 1) : null;
      if (target !== null) { event.preventDefault(); refs.current[target]?.focus(); }
    }}><span>{index + 1}. {step.title}</span><svg aria-hidden="true" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="m4 6 4 4 4-4" /></svg></button></h3><div role="region" id={`tattoo-step-panel-${index}`} aria-labelledby={`tattoo-step-${index}`} hidden={selected !== index}><p>{step.description}</p></div></div>)}</div>
    <div className={styles.howImage}><TattooPicture image={media.steps[0]} sizes="(max-width: 767px) 90vw, 60vw" /></div>
  </div></section>;
}

export function TattooBenefits() {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  return <section id="benefits" className={styles.benefits} aria-label={copy.hero.title}>{copy.features.map((feature, index) => <div className={styles.feature} key={feature.title}><div className={styles.featureText}><h2>{feature.title}</h2><p>{feature.description}</p></div><div className={styles.featureImage}><TattooPicture image={media.features[index]} sizes="(max-width: 767px) 90vw, 46vw" /></div></div>)}</section>;
}

export function TattooFlows({ onPreview }: { onPreview: (item: OverviewAction) => void }) {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  return <section id="more-flows" className={styles.flows}><h2>{copy.flows.title}</h2><LandingCarousel label={copy.flows.label} previous={copy.flows.previous} next={copy.flows.next} className={styles.flowViewport} bounded>{copy.flows.items.map((item, index) => {
    const children = <><TattooPicture image={media.flows[index]} sizes="(max-width: 767px) 80vw, 26vw" decorative /><div className={styles.flowText}><h3>{item.title}</h3><p>{item.description}</p>{"planned" in item ? <small>{copy.comingSoon}</small> : null}<LandingArrow /></div></>;
    return item.href ? <Link key={item.title} href={item.href} className={styles.flowCard} data-flow-card>{children}</Link> : <button key={item.title} type="button" className={styles.flowCard} data-flow-card onClick={() => onPreview(item)}>{children}</button>;
  })}</LandingCarousel></section>;
}

export function TattooDirectory() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  return <section id="directory" className={styles.directory}><h2>{copy.directory.title}</h2><p>{copy.directory.description}</p><div className={styles.directoryLinks}>{copy.directory.items.map(item => <Link href={`/app?model=${encodeURIComponent(item.id)}`} key={item.id}>{item.label}<span aria-hidden="true">↗</span></Link>)}{copy.directory.tools.map(item => <Link href={item.href} key={item.href}>{item.label}<span aria-hidden="true">↗</span></Link>)}</div></section>;
}

export function TattooPlans() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  return <LandingPlanCards {...copy.plans} />;
}

export function TattooWorkflow() {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  return <section id="workflow" className={styles.workflow}><div className={styles.centerHeading}><h2>{copy.workflow.title}</h2><p>{copy.workflow.description}</p></div><div className={styles.workflowGrid}>{copy.workflow.items.map((item, index) => <article key={item.title} className={styles.workflowCard} data-workflow-card><TattooPicture image={media.workflow[index]} sizes={index === 0 ? "(max-width: 767px) 90vw, 62vw" : "(max-width: 767px) 90vw, 30vw"} decorative /><div><span className={styles.workflowNumber}>{String(index + 1).padStart(2, "0")}</span><h3>{item.title}</h3><p>{item.description}</p></div></article>)}</div><p className={styles.artworkNote}>{copy.hero.attribution}</p></section>;
}

export function TattooQuestions() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"tattooEn","media":"tattooReferenceEn"});

  return <section id="questions" className={styles.questions}><h2>{copy.faq.title}</h2><FAQ items={copy.faq.items} name="tattoo-faq" /></section>;
}

const publicCopySourcesForLocale={copy,media};
