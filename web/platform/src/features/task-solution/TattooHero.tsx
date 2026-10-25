"use client";

import { useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { stageImagePrompt } from "@/features/image-generation/image-prompt-draft";
import { tattooEn as copy } from "@/i18n/public/tattoo-en";
import { tattooReferenceEn as media } from "@/i18n/public/tattoo-reference-en";
import Link from "@/i18n/Link";
import { useRouter } from "@/i18n/navigation";
import { composeTattooPrompt } from "./tattoo-prompt";
import { TattooPicture, type TattooImage } from "./TattooPicture";
import styles from "./TattooSolution.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";
import {useLocale} from "@/i18n/LocaleProvider";


export function TattooHero({ onPreview }: { onPreview: (image: TattooImage) => void }) {
  const {copy,media} = usePublicCopy(publicCopySourcesForLocale);
  const locale = useLocale();

  const [idea, setIdea] = useState("");
  const [style, setStyle] = useState("");
  const [placement, setPlacement] = useState("");
  const [error, setError] = useState(false);
  const [selected, setSelected] = useState(0);
  const ideaInput = useRef<HTMLInputElement>(null);
  const router = useRouter();
  return <section id="top" className={styles.hero} aria-labelledby="tattoo-title"><div className={styles.heroInner}>
    <nav className={styles.breadcrumb} aria-label={copy.breadcrumb}><Link href="/">{copy.home}</Link><span aria-hidden="true">/</span><Link href="/ai">{copy.tools}</Link><span aria-hidden="true">/</span><span>{copy.hero.title}</span></nav>
    <div className={styles.intro}><h1 id="tattoo-title">{copy.hero.title}</h1><p>{copy.hero.description}</p></div>
    <div className={styles.gallery}><button type="button" className={styles.mainExample} aria-label={copy.hero.enlarge} onClick={() => onPreview(media.examples[selected])}><TattooPicture image={media.examples[selected]} sizes={selected === 0 ? "(max-width: 900px) 135vw, 70vw" : "(max-width: 900px) 92vw, 46vw"} preload /></button>
      <div className={styles.thumbnails}>{media.examples.map((image, index) => <button key={image.file} type="button" aria-label={`${copy.hero.example} ${index + 1}`} aria-pressed={selected === index} onClick={() => setSelected(index)}><TattooPicture image={image} sizes="(max-width: 900px) 17vw, 9vw" /></button>)}</div>
      <span className={styles.referenceCredit}>{copy.hero.attribution}</span>
    </div>
    <form className={styles.form} aria-label={copy.hero.form} noValidate onSubmit={event => {
      event.preventDefault();
      const prompt = composeTattooPrompt(idea, style, placement, locale);
      if (!prompt) { setError(true); ideaInput.current?.focus(); return; }
      setError(false); stageImagePrompt(prompt); router.push("/app/image");
    }}>
      <p className={styles.formTitle}>{copy.hero.inputs}</p>
      <InputSurface className={styles.field}><label htmlFor="tattoo-idea">{copy.hero.idea}</label><input id="tattoo-idea" ref={ideaInput} value={idea} maxLength={4000} aria-required="true" aria-invalid={error || undefined} aria-describedby={error ? "tattoo-error" : "tattoo-idea-help"} onChange={event => { setIdea(event.target.value); if (event.target.value.trim()) setError(false); }} placeholder={copy.hero.ideaPlaceholder} /><span id="tattoo-idea-help">{copy.hero.ideaHelp}</span></InputSurface>
      <InputSurface className={styles.field}><label htmlFor="tattoo-style">{copy.hero.style}</label><input id="tattoo-style" value={style} maxLength={240} onChange={event => setStyle(event.target.value)} placeholder={copy.hero.stylePlaceholder} /><div className={styles.presets}>{copy.hero.styles.map(value => <button key={value} type="button" aria-pressed={style === value} onClick={() => setStyle(value)}>{value}</button>)}</div></InputSurface>
      <InputSurface className={styles.field}><label htmlFor="tattoo-placement">{copy.hero.placement}</label><input id="tattoo-placement" value={placement} maxLength={160} onChange={event => setPlacement(event.target.value)} placeholder={copy.hero.placementPlaceholder} /><div className={styles.presets}>{copy.hero.placements.map(value => <button key={value} type="button" aria-pressed={placement === value} onClick={() => setPlacement(value)}>{value}</button>)}</div></InputSurface>
      {error ? <p id="tattoo-error" role="alert" className={styles.error}>{copy.hero.error}</p> : null}
      <div className={styles.submitRow}><Button type="submit" className={styles.submit}>{copy.hero.submit}<LandingArrow /></Button></div><p className={styles.formNote}>{copy.hero.note}</p>
    </form>
  </div></section>;
}

const publicCopySourcesForLocale={copy,media};
