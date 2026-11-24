"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { Button } from "@/components/ui/Button/Button";
import { HomePicture } from "@/features/landing-variants/third/HomeMedia";
import { homePhotoSource } from "@/features/landing-variants/third/photos";
import { presentationDeckCopy } from "@/i18n/presentation-deck";
import { downloadFile, type Brief, type DeckSlide } from "./deck-content";
import { powerpoint } from "./deck-powerpoint";
import styles from "./Studio.module.css";

function Slide({ slide, index, brief }: { slide: DeckSlide; index: number; brief: Brief }) {
  return <article className={styles.sheet} data-cover={index === 0} data-style={brief.style}>
    {index === 0 && <div className={styles.slidePhoto}><HomePicture photo="workflowStart" original sizes="(max-width: 680px) 40vw, 460px" /></div>}
    <span className={styles.folio}>{brief.style} · {String(index + 1).padStart(2, "0")} / {String(brief.count).padStart(2, "0")}</span>
    <h3>{slide.title}</h3>
    <ul>{slide.points.map((point, i) => <li key={i}>{point}</li>)}</ul>
  </article>;
}

async function coverJpeg() {
  const image = new Image();
  image.src = homePhotoSource("workflowStart");
  await image.decode();
  const canvas = document.createElement("canvas");
  canvas.width = 650;
  canvas.height = 778;
  const context = canvas.getContext("2d");
  if (!context) throw new Error("cover unavailable");
  const scale = Math.max(canvas.width / image.naturalWidth, canvas.height / image.naturalHeight);
  const width = image.naturalWidth * scale, height = image.naturalHeight * scale;
  context.drawImage(image, (canvas.width - width) * .55, (canvas.height - height) * .67, width, height);
  const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, "image/jpeg", .85));
  if (!blob) throw new Error("cover unavailable");
  return new Uint8Array(await blob.arrayBuffer());
}

export function DeckResult({ slides, brief, onChange, onNew }: { slides: DeckSlide[]; brief: Brief; onChange: (slides: DeckSlide[]) => void; onNew: () => void }) {
  const [index, setIndex] = useState(0);
  const [show, setShow] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportNotice, setExportNotice] = useState("");
  const [draft, setDraft] = useState<{ title: string; body: string }>();
  const [editError, setEditError] = useState("");
  function editSlide() {
    setDraft({ title: slides[index].title, body: slides[index].points.join("\n") });
    setEditError("");
  }
  async function save() {
    if (exporting) return;
    setExporting(true);
    try {
      const jpeg = await coverJpeg().catch(() => undefined);
      downloadFile(powerpoint(slides, brief.style, jpeg), "epifera-slides.pptx");
      setExportNotice(jpeg ? presentationDeckCopy.result.exportReady : presentationDeckCopy.result.exportReadyWithoutPhoto);
    } finally { setExporting(false); }
  }
  useEffect(() => {
    if (!show) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      setIndex(value => Math.min(slides.length - 1, Math.max(0, value + (event.key === "ArrowRight" ? 1 : -1))));
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [show, slides.length]);
  const navigation = <div className={styles.pager}>
    <Button variant="outline" aria-label={presentationDeckCopy.result.previousSlide} disabled={!index} onClick={() => setIndex(index - 1)}><LandingArrow direction="left" /></Button>
    <span role="status">{index + 1} / {slides.length}</span>
    <Button variant="outline" aria-label={presentationDeckCopy.result.nextSlide} disabled={index === slides.length - 1} onClick={() => setIndex(index + 1)}><LandingArrow /></Button>
  </div>;
  const page = <Slide slide={slides[index]} index={index} brief={brief} />;
  return <div className={styles.result}>
    <div className={styles.resultHeading}><h2>{presentationDeckCopy.result.heading}</h2><Button variant="outline" onClick={onNew}>{presentationDeckCopy.result.newPresentation}</Button></div>
    <div className={styles.resultGrid}>
      <nav className={styles.outline} aria-label={presentationDeckCopy.result.outlineLabel}>{slides.map((slide, i) =>
        <button key={i} disabled={!!draft} aria-current={index === i ? "step" : undefined} onClick={() => setIndex(i)}><span>{String(i + 1).padStart(2, "0")}</span>{slide.title}</button>)}</nav>
      <div className={styles.resultStage}>{draft ? <form className={styles.more} onSubmit={event => {
        event.preventDefault();
        const title = draft.title.trim(), points = draft.body.split("\n").map(line => line.trim()).filter(Boolean);
        if (!title || !points.length || points.length > 5 || points.some(point => point.length > 300)) {
          setEditError(presentationDeckCopy.result.invalidEdit);
          return;
        }
        onChange(slides.map((slide, i) => i === index ? { title, points } : slide));
        setExportNotice(presentationDeckCopy.result.editSaved); setDraft(undefined);
      }}>
        <label>{presentationDeckCopy.result.titleLabel}<input required maxLength={160} value={draft.title} onChange={event => setDraft({ ...draft, title: event.target.value })} /></label>
        <label>{presentationDeckCopy.result.pointsLabel}<textarea required rows={5} value={draft.body} maxLength={1504} onChange={event => setDraft({ ...draft, body: event.target.value })} /></label>
        {editError && <p className={styles.error} role="alert">{editError}</p>}
        <div className={styles.actions}><Button className={styles.primary} type="submit">{presentationDeckCopy.result.saveSlide}</Button><Button variant="outline" onClick={() => setDraft(undefined)}>{presentationDeckCopy.result.cancel}</Button></div>
      </form> : <>{page}{!show && navigation}</>}</div>
    </div>
    {!draft && <div className={styles.actions}>
      <Button className={styles.primary} onClick={save} disabled={exporting}>{exporting ? presentationDeckCopy.result.exportPending : presentationDeckCopy.result.downloadPowerPoint}</Button>
      <Button variant="outline" onClick={() => setShow(true)}>{presentationDeckCopy.result.startShow}</Button>
      <Button variant="outline" onClick={() => window.print()}>{presentationDeckCopy.result.printPdf}</Button>
      <Button variant="outline" onClick={editSlide}>{presentationDeckCopy.result.editSlide}</Button>
    </div>}
    <p className={styles.feedback} role="status">{exportNotice}</p>
    {show && <LandingDialog title={brief.topic} closeLabel={presentationDeckCopy.result.closeShow} wide className={styles.showDialog} onClose={() => setShow(false)}>{page}{navigation}</LandingDialog>}
    {createPortal(<div className={styles.printDeck}>{slides.map((slide, i) => <Slide key={i} slide={slide} index={i} brief={brief} />)}</div>, document.body)}
  </div>;
}
