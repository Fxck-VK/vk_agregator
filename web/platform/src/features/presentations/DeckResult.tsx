"use client";

import NextImage from "next/image";
import dynamic from "next/dynamic";
import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { createPortal } from "react-dom";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { Button } from "@/components/ui/Button/Button";
import { HomePicture } from "@/features/landing-variants/third/HomeMedia";
import { homePhotoSource } from "@/features/landing-variants/third/photos";
import { presentationDeckCopy, type PresentationStyle } from "@/i18n/presentation-deck";
import { browserSessionSnapshot } from "@/lib/web-api/browser-session-state";
import { downloadFile, type Brief, type DeckSlide } from "./deck-content";
import { powerpoint } from "./deck-powerpoint";
import { prepareSlideImage, renderSlideImage, slideImageBytes, slideImageTypes, slideImageUploadLimit } from "./slide-image";
import styles from "./DeckResult.module.css";

const copy = presentationDeckCopy.result;
const maxSlides = 30;
const SlideImageGeneration = dynamic(() => import("./SlideImageGeneration").then(module => module.SlideImageGeneration));

type SlideDraft = {
  kind: "edit" | "new";
  title: string;
  body: string;
};

function slideNumber(index: number) {
  return String(index + 1).padStart(2, "0");
}

function slidePoints(body: string) {
  return body.split("\n").map(line => line.trim().replace(/^[-*•]\s+/, "")).filter(Boolean);
}

function Slide({ slide, index, total, style }: { slide: DeckSlide; index: number; total: number; style: PresentationStyle }) {
  const cover = index === 0;
  const photo = cover || Boolean(slide.image);
  const textLength = slide.title.length + slide.points.reduce((length, point) => length + point.length, 0);
  const dense = textLength > (photo ? 220 : 450) || slide.title.length > 90;
  return (
    <article className={styles.sheet} data-cover={cover} data-photo={photo} data-style={style} data-dense={dense}>
      {photo && (
        <div className={styles.slidePhoto}>
          {slide.image
            ? <NextImage src={slide.image} width={650} height={778} unoptimized alt="" />
            : <HomePicture photo="workflowStart" original sizes="(max-width: 680px) 40vw, 460px" />}
        </div>
      )}
      <div className={styles.slideContent}>
        <span className={styles.folio}>{style} · {slideNumber(index)} / {slideNumber(total - 1)}</span>
        <h3>{slide.title || copy.titleLabel}</h3>
        {slide.points.length > 0 && <ul>{slide.points.map((point, i) => <li key={i}>{point}</li>)}</ul>}
      </div>
    </article>
  );
}

export function DeckResult({ slides, brief, canGenerateImage = false, onChange, onNew }: {
  slides: DeckSlide[];
  brief: Brief;
  canGenerateImage?: boolean;
  onChange: (slides: DeckSlide[]) => boolean | void;
  onNew: () => void;
}) {
  const [selected, setSelected] = useState(0);
  const [show, setShow] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [preparingImage, setPreparingImage] = useState(false);
  const [imagePrompt, setImagePrompt] = useState<string>();
  const [notice, setNotice] = useState("");
  const [draft, setDraft] = useState<SlideDraft>();
  const [editError, setEditError] = useState("");
  const [previous, setPrevious] = useState<{ slides: DeckSlide[]; index: number }>();
  const imageInput = useRef<HTMLInputElement>(null);
  const imageOperation = useRef<AbortController>(null);
  const titleField = useRef<HTMLInputElement>(null);
  const editButton = useRef<HTMLButtonElement>(null);
  const focusEditorButton = useRef(false);
  const index = Math.min(selected, Math.max(0, slides.length - 1));
  const editing = draft !== undefined;
  const imageBusy = preparingImage || imagePrompt !== undefined;
  const current = slides[index];
  const adding = draft?.kind === "new";
  const previewIndex = adding ? index + 1 : index;
  const previewTotal = slides.length + (adding ? 1 : 0);
  const preview = draft
    ? { image: adding ? undefined : current?.image, title: draft.title.trim(), points: slidePoints(draft.body) }
    : current;

  useEffect(() => () => imageOperation.current?.abort(), []);

  useEffect(() => {
    if (editing) titleField.current?.focus();
    else if (focusEditorButton.current) {
      focusEditorButton.current = false;
      editButton.current?.focus();
    }
  }, [editing]);

  useEffect(() => {
    if (!show) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      setSelected(value => Math.min(slides.length - 1, Math.max(0, value + (event.key === "ArrowRight" ? 1 : -1))));
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [show, slides.length]);

  if (!current) return null;

  function beginEdit(kind: SlideDraft["kind"]) {
    if (kind === "new" && slides.length >= maxSlides) return;
    const slide = slides[index];
    setDraft(kind === "new" ? { kind, title: "", body: "" } : { kind, title: slide.title, body: slide.points.join("\n") });
    setEditError("");
    setNotice("");
  }

  function closeEditor() {
    focusEditorButton.current = true;
    setDraft(undefined);
    setEditError("");
  }

  function applySlides(next: DeckSlide[], selectedIndex: number, feedback: string) {
    setPrevious({ slides, index });
    const persisted = onChange(next);
    setSelected(selectedIndex);
    setNotice(persisted === false && (next.some(slide => slide.image) || slides.some(slide => slide.image)) ? copy.imageDraftUnavailable : feedback);
  }

  function saveSlide(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft) return;
    const title = draft.title.trim();
    const points = slidePoints(draft.body);
    if (!title || title.length > 160 || points.length < 1 || points.length > 5 || points.some(point => point.length > 300)) {
      setEditError(copy.invalidEdit);
      return;
    }
    const slide = { title, points };
    const next = draft.kind === "new"
      ? [...slides.slice(0, index + 1), slide, ...slides.slice(index + 1)]
      : slides.map((item, i) => i === index ? { ...item, ...slide } : item);
    applySlides(next, previewIndex, draft.kind === "new" ? copy.added : copy.editSaved);
    closeEditor();
  }

  function moveSlide(offset: -1 | 1) {
    const target = index + offset;
    if (target < 0 || target >= slides.length) return;
    const next = [...slides];
    [next[index], next[target]] = [next[target], next[index]];
    applySlides(next, target, copy.moved);
  }

  function removeSlide() {
    if (slides.length < 2) return;
    applySlides(slides.filter((_, i) => i !== index), Math.min(index, slides.length - 2), copy.removed);
  }

  function undo() {
    if (!previous) return;
    const persisted = onChange(previous.slides);
    setSelected(previous.index);
    setPrevious(undefined);
    setNotice(persisted === false && (previous.slides.some(slide => slide.image) || slides.some(slide => slide.image)) ? copy.imageDraftUnavailable : copy.undone);
  }

  async function attachImage(file: Blob) {
    if (imageOperation.current) return;
    const operation = new AbortController();
    const session = browserSessionSnapshot();
    imageOperation.current = operation;
    setPreparingImage(true);
    try {
      const image = await prepareSlideImage(file);
      if (operation.signal.aborted || session.signal.aborted) return;
      applySlides(slides.map((slide, i) => i === index ? { ...slide, image } : slide), index, copy.imageChanged);
    } finally {
      imageOperation.current = null;
      if (!operation.signal.aborted) setPreparingImage(false);
    }
  }

  async function selectImage(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file || imageBusy) return;
    if (!slideImageTypes.includes(file.type) || file.size > slideImageUploadLimit) {
      setNotice(copy.imageFileLimit);
      return;
    }
    try { await attachImage(file); }
    catch { setNotice(copy.imageFailed); }
  }

  function removeImage() {
    const next = slides.map((slide, i) => i === index ? { title: slide.title, points: slide.points } : slide);
    applySlides(next, index, index === 0 ? copy.coverReset : copy.imageRemoved);
  }

  async function savePowerPoint() {
    if (exporting) return;
    setExporting(true);
    try {
      const images = slides.map(slide => slide.image ? slideImageBytes(slide.image) : undefined);
      if (!images[0]) {
        const cover = await renderSlideImage(homePhotoSource("workflowStart")).catch(() => undefined);
        if (cover) images[0] = new Uint8Array(await cover.arrayBuffer());
      }
      downloadFile(powerpoint(slides, brief.style, images), "epifera-slides.pptx");
      setNotice(images[0] ? copy.exportReady : copy.exportReadyWithoutPhoto);
    } catch {
      setNotice(copy.exportFailed);
    } finally {
      setExporting(false);
    }
  }

  const navigation = <div className={styles.pager}>
    <Button variant="outline" aria-label={copy.previousSlide} disabled={index === 0 || imageBusy} onClick={() => setSelected(index - 1)}><LandingArrow direction="left" /></Button>
    <span aria-live="polite">{slideNumber(index)} / {slideNumber(slides.length - 1)}</span>
    <Button variant="outline" aria-label={copy.nextSlide} disabled={index === slides.length - 1 || imageBusy} onClick={() => setSelected(index + 1)}><LandingArrow /></Button>
  </div>;

  return (
    <div className={styles.result}>
      <input type="file" hidden ref={imageInput} accept={slideImageTypes.join(",")} onChange={selectImage} />
      <div className={styles.resultHeading}>
        <div>
          <h2>{brief.topic || copy.heading}</h2>
          <p>{copy.deckSummary(slides.length, brief.style)}</p>
        </div>
        <Button variant="outline" disabled={editing || imageBusy} onClick={onNew}>{copy.newPresentation}</Button>
      </div>
      <div className={styles.resultGrid}>
        <aside className={styles.sidebar}>
          <nav className={styles.outline} aria-label={copy.outlineLabel}>
            {slides.map((slide, i) => (
              <button type="button" key={i} disabled={editing || imageBusy} aria-current={index === i ? "step" : undefined} onClick={() => setSelected(i)}>
                <span aria-hidden="true">{slideNumber(i)}</span><span>{slide.title}</span>
              </button>
            ))}
          </nav>
          <Button variant="outline" disabled={editing || imageBusy || slides.length >= maxSlides} title={slides.length >= maxSlides ? copy.slideLimit : undefined} onClick={() => beginEdit("new")}>{copy.addSlide}</Button>
        </aside>
        <div className={styles.resultStage}>
          <div className={styles.preview}>
            <Slide slide={preview} index={previewIndex} total={previewTotal} style={brief.style} />
            {!editing && !show && navigation}
          </div>
          {draft ? (
            <form className={styles.editor} onSubmit={saveSlide}>
              <label>{copy.titleLabel}
                <input ref={titleField} required maxLength={160} value={draft.title} onChange={event => { setDraft({ ...draft, title: event.target.value }); setEditError(""); }} />
              </label>
              <label>{copy.pointsLabel}
                <textarea required rows={5} maxLength={1504} value={draft.body} onChange={event => { setDraft({ ...draft, body: event.target.value }); setEditError(""); }} />
              </label>
              {editError && <p className={styles.error} role="alert">{editError}</p>}
              <div className={styles.actions}>
                <Button className={styles.primary} type="submit">{copy.saveSlide}</Button>
                <Button variant="outline" onClick={closeEditor}>{copy.cancel}</Button>
              </div>
            </form>
          ) : (
            <div className={styles.editActions}>
              <div className={styles.actions}>
                <Button ref={editButton} className={styles.primary} disabled={imageBusy} onClick={() => beginEdit("edit")}>{copy.editSlide}</Button>
                <Button variant="outline" aria-label={copy.moveEarlier} disabled={index === 0 || imageBusy} onClick={() => moveSlide(-1)}>{copy.moveEarlierShort}</Button>
                <Button variant="outline" aria-label={copy.moveLater} disabled={index === slides.length - 1 || imageBusy} onClick={() => moveSlide(1)}>{copy.moveLaterShort}</Button>
              </div>
              <div className={styles.actions}>
                {previous && <Button variant="outline" disabled={imageBusy} onClick={undo}>{copy.undo}</Button>}
                <Button variant="outline" className={styles.deleteButton} disabled={slides.length < 2 || imageBusy} onClick={removeSlide}>{copy.deleteSlide}</Button>
              </div>
            </div>
          )}
          {!editing && (
            <div className={styles.actions}>
              <Button variant="outline" disabled={imageBusy} onClick={() => imageInput.current?.click()}>
                {preparingImage ? copy.imagePreparing : index === 0 ? copy.changeCover : current.image ? copy.replaceImage : copy.uploadImage}
              </Button>
              {canGenerateImage && <Button variant="outline" disabled={imageBusy} onClick={() => setImagePrompt(copy.imagePrompt(brief.topic, current.title))}>{copy.generateImage}</Button>}
              {current.image && <Button variant="outline" disabled={imageBusy} onClick={removeImage}>{index === 0 ? copy.resetCover : copy.removeImage}</Button>}
            </div>
          )}
        </div>
      </div>
      <div className={styles.export}>
        <div className={styles.actions}>
          <Button className={styles.primary} onClick={savePowerPoint} disabled={editing || imageBusy || exporting}>{exporting ? copy.exportPending : copy.downloadPowerPoint}</Button>
          <Button variant="outline" disabled={editing || imageBusy} onClick={() => setShow(true)}>{copy.startShow}</Button>
          <Button variant="outline" disabled={editing || imageBusy} onClick={() => window.print()}>{copy.printPdf}</Button>
        </div>
        <p className={styles.feedback} role="status">{notice}</p>
      </div>
      {imagePrompt !== undefined && <SlideImageGeneration prompt={imagePrompt} onImage={attachImage} onClose={() => setImagePrompt(undefined)} />}
      {show && (
        <LandingDialog title={brief.topic || copy.heading} closeLabel={copy.closeShow} wide className={styles.showDialog} onClose={() => setShow(false)}>
          <Slide slide={current} index={index} total={slides.length} style={brief.style} />
          {navigation}
        </LandingDialog>
      )}
      {typeof document !== "undefined" && createPortal(
        <div className={styles.printDeck} aria-hidden="true">
          {slides.map((slide, i) => <Slide key={i} slide={slide} index={i} total={slides.length} style={brief.style} />)}
        </div>, document.body,
      )}
    </div>
  );
}
