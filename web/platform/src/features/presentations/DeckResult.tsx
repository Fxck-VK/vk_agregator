"use client";

import NextImage from "next/image";
import { useEffect, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { createPortal } from "react-dom";
import { LandingArrow } from "@/components/public/landing/LandingPrimitives";
import { LandingDialog } from "@/components/public/landing/LandingDialog";
import { Button } from "@/components/ui/Button/Button";
import { HomePicture } from "@/features/landing-variants/third/HomeMedia";
import { homePhotoSource } from "@/features/landing-variants/third/photos";
import { presentationDeckCopy, type PresentationStyle } from "@/i18n/presentation-deck";
import { downloadFile, type Brief, type DeckSlide } from "./deck-content";
import { powerpoint } from "./deck-powerpoint";
import styles from "./DeckResult.module.css";

const copy = presentationDeckCopy.result;
const coverTypes = ["image/jpeg", "image/png", "image/webp"];
const maxCoverBytes = 8 * 1024 * 1024;
const maxSlides = 30;

type Cover = { jpeg: Blob; source: string };

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

function Slide({ slide, index, total, style, coverSource }: { slide: DeckSlide; index: number; total: number; style: PresentationStyle; coverSource?: string }) {
  const cover = index === 0;
  const textLength = slide.title.length + slide.points.reduce((length, point) => length + point.length, 0);
  const dense = textLength > (cover ? 220 : 450) || slide.title.length > 90;
  return (
    <article className={styles.sheet} data-cover={cover} data-style={style} data-dense={dense}>
      {cover && (
        <div className={styles.slidePhoto}>
          {coverSource
            ? <NextImage src={coverSource} width={650} height={778} unoptimized alt="" />
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

async function coverJpeg(source: string) {
  const image = new Image();
  image.src = source;
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
  return blob;
}

function coverDataUrl(image: Blob) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") resolve(reader.result);
      else reject(new Error("cover unavailable"));
    };
    reader.onerror = () => reject(new Error("cover unavailable"));
    reader.readAsDataURL(image);
  });
}

export function DeckResult({ slides, brief, onChange, onNew }: { slides: DeckSlide[]; brief: Brief; onChange: (slides: DeckSlide[]) => void; onNew: () => void }) {
  const [selected, setSelected] = useState(0);
  const [show, setShow] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [cover, setCover] = useState<Cover>();
  const [preparingCover, setPreparingCover] = useState(false);
  const [notice, setNotice] = useState("");
  const [draft, setDraft] = useState<SlideDraft>();
  const [editError, setEditError] = useState("");
  const [previous, setPrevious] = useState<{ slides: DeckSlide[]; index: number }>();
  const coverInput = useRef<HTMLInputElement>(null);
  const titleField = useRef<HTMLInputElement>(null);
  const editButton = useRef<HTMLButtonElement>(null);
  const focusEditorButton = useRef(false);
  const index = Math.min(selected, Math.max(0, slides.length - 1));
  const editing = draft !== undefined;
  const current = slides[index];
  const adding = draft?.kind === "new";
  const previewIndex = adding ? index + 1 : index;
  const previewTotal = slides.length + (adding ? 1 : 0);
  const preview = draft ? { title: draft.title.trim(), points: slidePoints(draft.body) } : current;

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
    onChange(next);
    setSelected(selectedIndex);
    setNotice(feedback);
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
      : slides.map((item, i) => i === index ? slide : item);
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
    onChange(previous.slides);
    setSelected(previous.index);
    setPrevious(undefined);
    setNotice(copy.undone);
  }

  async function selectCover(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    if (!coverTypes.includes(file.type) || file.size > maxCoverBytes) {
      setNotice(copy.coverFileLimit);
      return;
    }
    const source = URL.createObjectURL(file);
    setPreparingCover(true);
    try {
      const jpeg = await coverJpeg(source);
      setCover({ jpeg, source: await coverDataUrl(jpeg) });
      setNotice(copy.coverChanged);
    } catch {
      setNotice(copy.coverFailed);
    } finally {
      URL.revokeObjectURL(source);
      setPreparingCover(false);
    }
  }

  async function savePowerPoint() {
    if (exporting) return;
    setExporting(true);
    try {
      const picture = cover?.jpeg ?? await coverJpeg(homePhotoSource("workflowStart")).catch(() => undefined);
      const jpeg = picture ? new Uint8Array(await picture.arrayBuffer()) : undefined;
      downloadFile(powerpoint(slides, brief.style, jpeg), "epifera-slides.pptx");
      setNotice(jpeg ? copy.exportReady : copy.exportReadyWithoutPhoto);
    } catch {
      setNotice(copy.exportFailed);
    } finally {
      setExporting(false);
    }
  }

  const navigation = <div className={styles.pager}>
    <Button variant="outline" aria-label={copy.previousSlide} disabled={index === 0} onClick={() => setSelected(index - 1)}><LandingArrow direction="left" /></Button>
    <span aria-live="polite">{slideNumber(index)} / {slideNumber(slides.length - 1)}</span>
    <Button variant="outline" aria-label={copy.nextSlide} disabled={index === slides.length - 1} onClick={() => setSelected(index + 1)}><LandingArrow /></Button>
  </div>;

  return (
    <div className={styles.result}>
      <input type="file" hidden ref={coverInput} accept={coverTypes.join(",")} onChange={selectCover} />
      <div className={styles.resultHeading}>
        <div>
          <h2>{brief.topic || copy.heading}</h2>
          <p>{copy.deckSummary(slides.length, brief.style)}</p>
        </div>
        <Button variant="outline" disabled={editing} onClick={onNew}>{copy.newPresentation}</Button>
      </div>
      <div className={styles.resultGrid}>
        <aside className={styles.sidebar}>
          <nav className={styles.outline} aria-label={copy.outlineLabel}>
            {slides.map((slide, i) => (
              <button type="button" key={i} disabled={editing} aria-current={index === i ? "step" : undefined} onClick={() => setSelected(i)}>
                <span aria-hidden="true">{slideNumber(i)}</span><span>{slide.title}</span>
              </button>
            ))}
          </nav>
          <Button variant="outline" disabled={editing || slides.length >= maxSlides} title={slides.length >= maxSlides ? copy.slideLimit : undefined} onClick={() => beginEdit("new")}>{copy.addSlide}</Button>
        </aside>
        <div className={styles.resultStage}>
          <div className={styles.preview}>
            <Slide slide={preview} index={previewIndex} total={previewTotal} style={brief.style} coverSource={cover?.source} />
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
                <Button ref={editButton} className={styles.primary} onClick={() => beginEdit("edit")}>{copy.editSlide}</Button>
                <Button variant="outline" aria-label={copy.moveEarlier} disabled={index === 0} onClick={() => moveSlide(-1)}>{copy.moveEarlierShort}</Button>
                <Button variant="outline" aria-label={copy.moveLater} disabled={index === slides.length - 1} onClick={() => moveSlide(1)}>{copy.moveLaterShort}</Button>
              </div>
              <div className={styles.actions}>
                {previous && <Button variant="outline" onClick={undo}>{copy.undo}</Button>}
                <Button variant="outline" className={styles.deleteButton} disabled={slides.length < 2} onClick={removeSlide}>{copy.deleteSlide}</Button>
              </div>
            </div>
          )}
          {index === 0 && !editing && (
            <div className={styles.actions}>
              <Button variant="outline" disabled={preparingCover} onClick={() => coverInput.current?.click()}>{preparingCover ? copy.coverPreparing : copy.changeCover}</Button>
              {cover && <Button variant="outline" disabled={preparingCover} onClick={() => { setCover(undefined); setNotice(copy.coverReset); }}>{copy.resetCover}</Button>}
            </div>
          )}
        </div>
      </div>
      <div className={styles.export}>
        <div className={styles.actions}>
          <Button className={styles.primary} onClick={savePowerPoint} disabled={editing || preparingCover || exporting}>{exporting ? copy.exportPending : copy.downloadPowerPoint}</Button>
          <Button variant="outline" disabled={editing || preparingCover} onClick={() => setShow(true)}>{copy.startShow}</Button>
          <Button variant="outline" disabled={editing || preparingCover} onClick={() => window.print()}>{copy.printPdf}</Button>
        </div>
        <p className={styles.feedback} role="status">{notice}</p>
      </div>
      {show && (
        <LandingDialog title={brief.topic || copy.heading} closeLabel={copy.closeShow} wide className={styles.showDialog} onClose={() => setShow(false)}>
          <Slide slide={current} index={index} total={slides.length} style={brief.style} coverSource={cover?.source} />
          {navigation}
        </LandingDialog>
      )}
      {typeof document !== "undefined" && createPortal(
        <div className={styles.printDeck} aria-hidden="true">
          {slides.map((slide, i) => <Slide key={i} slide={slide} index={i} total={slides.length} style={brief.style} coverSource={cover?.source} />)}
        </div>, document.body,
      )}
    </div>
  );
}
