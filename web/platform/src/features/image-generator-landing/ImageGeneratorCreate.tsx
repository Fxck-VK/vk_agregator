"use client";

import { useRef, useState } from "react";
import { imageGeneratorEn as copy } from "@/i18n/public/image-generator-en";
import { imageGeneratorReferenceEn as reference } from "@/i18n/public/image-generator-reference-en";
import Link from "@/i18n/Link";
import { GeneratorPicture } from "./ImageGeneratorMedia";
import styles from "@/components/public/generator-landing/GeneratorLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function ImageGeneratorCreate() {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  const [selected, setSelected] = useState(0);
  const buttons = useRef<(HTMLButtonElement | null)[]>([]);

  return <section id="create" className={styles.chapter} aria-labelledby="create-title">
    <p className={styles.eyebrow}>{copy.create.eyebrow}</p><h2 id="create-title" className={styles.chapterTitle}>{copy.create.title}</h2><p className={styles.chapterIntro}>{copy.create.intro}</p>
    <div className={styles.createTabs}><ol className={styles.createTabList}>{copy.create.modes.map((mode, index) => <li key={mode.label} className={styles.createTabItem}>
      <button ref={node => { buttons.current[index] = node; }} type="button" id={`creation-mode-${index}`} aria-expanded={selected === index} aria-controls={`creation-panel-${index}`} className={`${styles.createTabButton} ${selected === index ? styles.createTabButtonActive : ""}`} onClick={() => setSelected(index)} onKeyDown={event => {
        const next = event.key === "ArrowDown" ? (index + 1) % 4 : event.key === "ArrowUp" ? (index + 3) % 4 : event.key === "Home" ? 0 : event.key === "End" ? 3 : null;
        if (next !== null) { event.preventDefault(); buttons.current[next]?.focus(); }
      }}><span className={styles.createTabNum}>{`0${index + 1}`}</span>{" "}{mode.label}</button>
      <div id={`creation-panel-${index}`} role="region" aria-labelledby={`creation-mode-${index}`} hidden={selected !== index} className={styles.createTabPanel}><h3 className={styles.createTabHeading}>{mode.title}</h3><p className={styles.createTabBody}>{mode.body}</p><Link href="/app/image" className={styles.createTabLink}>{copy.create.action}</Link></div>
    </li>)}</ol><CreationVisual mode={selected} /></div>
    <ol className={styles.createSteps}>{copy.create.steps.map((step, index) => <li className={styles.createStep} key={step.title}><span className={styles.createStepNum}>{`0${index + 1}`}</span><p className={styles.createStepText}><strong>{step.title}</strong> {step.body}</p></li>)}</ol>
  </section>;
}

function CreationVisual({ mode }: { mode: number }) {
  const {reference,copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  const images = [reference.createImages, reference.referenceImages, reference.consistentImages, reference.editImages][mode];
  const tile = (image: { readonly file: string; readonly alt: string }) => <div className={styles.createTile} key={image.file}><GeneratorPicture image={image} sizes="(max-width: 820px) 45vw, 22vw" /></div>;
  return <div className={styles.createVisualCol} key={mode}>{mode === 1 ? <div className={styles.createGridRef}>{tile(images[0])}<div className={styles.createRefStack}>{images.slice(1).map(tile)}</div></div> : mode === 3 ? <div className={styles.createGridEdit}>{images.map((image,index) => <figure key={image.file} className={styles.createEditItem}>{tile(image)}<figcaption className={styles.createEditCaption}>{copy.create.editCaptions[index]}</figcaption></figure>)}</div> : <div className={mode === 2 ? styles.createGrid3 : styles.createGrid2}>{images.map(tile)}</div>}</div>;
}

const publicCopySourcesForLocale={copy,reference};
