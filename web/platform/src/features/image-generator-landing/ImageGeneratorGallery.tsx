"use client";

import { useEffect, useId, useRef } from "react";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { imageGeneratorEn as copy } from "@/i18n/public/image-generator-en";
import { imageGeneratorReferenceEn as reference } from "@/i18n/public/image-generator-reference-en";
import Link from "@/i18n/Link";
import { GeneratorPicture, type ReferenceImage } from "./ImageGeneratorMedia";
import styles from "@/components/public/generator-landing/GeneratorLanding.module.css";
import {usePublicCopy} from "@/i18n/public/use-public-copy";


export function ImageGeneratorGallery({ onOpen, paused }: { onOpen: (image: ReferenceImage) => void; paused: boolean }) {
  const {copy,reference} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  return <section id="examples" className={styles.galleryStrip} aria-labelledby="gallery-title"><h2 id="gallery-title" className={styles.galleryTitle}>{copy.galleryTitle}</h2><div className={styles.marquee} data-paused={paused || undefined}>
    {[0, 1].map(track => <div className={styles.marqueeTrack} key={track} aria-hidden={track === 1 || undefined}>{reference.gallery.map(image => <button key={image.file} type="button" tabIndex={track ? -1 : 0} className={styles.galleryCard} aria-label={`${image.alt}. ${copy.controls.openDetails}`} onClick={() => onOpen(image)}><GeneratorPicture image={image} className={styles.galleryImg} sizes="(max-width: 600px) 180px, 210px" decorative={track === 1} /><span className={styles.galleryZoom} aria-hidden="true"><span className={styles.galleryZoomBadge}>+</span></span></button>)}</div>)}
  </div></section>;
}

function GallerySurface({ image, close }: { image: ReferenceImage; close: () => void }) {
  const {copy} = usePublicCopy(publicCopySourcesForLocale,{"copy":"imageGeneratorEn","reference":"imageGeneratorReferenceEn"});

  const ref = useRef<HTMLElement>(null); const title = useId();
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const page = document.querySelector<HTMLElement>("[data-image-generator-page]");
    const inert = page?.inert ?? false; if (page) page.inert = true;
    ref.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const trap = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const elements = [...(ref.current?.querySelectorAll<HTMLElement>("button,a[href]") ?? [])];
      const first = elements[0], last = elements.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    const node = ref.current; node?.addEventListener("keydown", trap);
    return () => { node?.removeEventListener("keydown", trap); if (page) page.inert = inert; if (previous?.isConnected) previous.focus(); };
  }, []);
  return <section ref={ref} role="dialog" aria-modal="true" aria-labelledby={title} className={`${styles.theme} ${styles.sheetContent}`}><h2 id={title} className={styles.srOnly}>{copy.controls.example}</h2><ModalCloseButton className={styles.sheetClose} aria-label={copy.controls.close} onClick={close} /><div className={styles.sheetMedia}><GeneratorPicture image={image} sizes="(max-width: 600px) 90vw, 460px" /></div><div className={styles.sheetActions}><Link href="/app/image" className={styles.sheetTry}>{copy.controls.tryImage}<span aria-hidden="true">→</span></Link></div></section>;
}
export function ImageGeneratorGalleryDialog({ image, onClose }: { image: ReferenceImage; onClose: () => void }) {
  return <ModalBackdrop onClose={onClose} testId="image-gallery-backdrop">{close => <GallerySurface image={image} close={close} />}</ModalBackdrop>;
}

const publicCopySourcesForLocale={copy,reference};
