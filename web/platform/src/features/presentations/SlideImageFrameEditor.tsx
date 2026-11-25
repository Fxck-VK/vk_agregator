"use client";

import { useEffect, useId, useRef, useState, type KeyboardEvent, type PointerEvent, type ReactNode } from "react";
import { Button } from "@/components/ui/Button/Button";
import { slideImageGenerationCopy } from "@/i18n/presentation-slide-image";
import { dragSlideImageFrame, setSlideImageFrameField, slideImageFrameCSS, slideImageFramePoint, type SlideImageFrame, type SlideImageFrameGesture } from "./slide-image-frame";
import styles from "./SlideImageFrameEditor.module.css";

const copy = slideImageGenerationCopy.frame;
const corners = ["nw", "ne", "sw", "se"] as const;
const frameFields = ["x", "y", "width", "height"] as const;
const keyboardMoves = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] } as const;

type Gesture = {
  pointer: number;
  frame: SlideImageFrame;
  mode: SlideImageFrameGesture;
  start: ReturnType<typeof slideImageFramePoint>;
  bounds: DOMRect;
};

export function SlideImageFrameEditor({ value, onChange, onNext, onCancel, children }: Readonly<{
  value: SlideImageFrame;
  onChange: (value: SlideImageFrame) => void;
  onNext: () => void;
  onCancel: () => void;
  children: ReactNode;
}>) {
  const canvas = useRef<HTMLDivElement>(null);
  const gesture = useRef<Gesture | null>(null);
  const [draft, setDraft] = useState<SlideImageFrame>();
  const instructionsID = useId();
  useEffect(() => { canvas.current?.focus(); }, []);

  function begin(event: PointerEvent<HTMLDivElement>) {
    if (event.button !== 0 || gesture.current) return;
    const surface = event.currentTarget;
    const bounds = surface.getBoundingClientRect();
    if (bounds.width === 0 || bounds.height === 0) return;
    const anchor = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-frame-mode]") : null;
    const mode = (anchor?.dataset.frameMode ?? "new") as SlideImageFrameGesture;
    gesture.current = { pointer: event.pointerId, frame: value, mode, bounds, start: slideImageFramePoint(event.clientX, event.clientY, bounds) };
    event.preventDefault();
    surface.focus({ preventScroll: true });
    surface.setPointerCapture?.(event.pointerId);
  }

  function readGesture(event: PointerEvent<HTMLDivElement>) {
    const active = gesture.current;
    if (!active || active.pointer !== event.pointerId) return;
    const end = slideImageFramePoint(event.clientX, event.clientY, active.bounds);
    const distance = Math.max(Math.abs(end.x - active.start.x), Math.abs(end.y - active.start.y));
    return distance < 50 ? active.frame : dragSlideImageFrame(active.frame, active.mode, active.start, end);
  }

  function cancelGesture() {
    gesture.current = null;
    setDraft(undefined);
  }

  function cancelPointer(event: PointerEvent<HTMLDivElement>) {
    if (gesture.current?.pointer === event.pointerId) cancelGesture();
  }

  function moveWithKeyboard(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      if (gesture.current) cancelGesture();
      else onCancel();
      return;
    }
    const direction = keyboardMoves[event.key as keyof typeof keyboardMoves];
    if (!direction) return;
    event.preventDefault();
    const step = event.shiftKey ? 500 : 100;
    onChange(dragSlideImageFrame(value, "move", { x: 0, y: 0 }, { x: step * direction[0], y: step * direction[1] }));
  }

  return <div className={styles.editor}>
    <div className={styles.preview}>
      {children}
      <div ref={canvas} className={styles.canvas} role="group" aria-label={copy.area} aria-describedby={instructionsID} tabIndex={0}
        onPointerDown={begin} onPointerMove={event => {
          const next = readGesture(event);
          if (next) setDraft(next);
        }} onPointerUp={event => {
          const next = readGesture(event);
          if (!next) return;
          cancelGesture();
          onChange(next);
        }} onLostPointerCapture={cancelPointer} onPointerCancel={cancelPointer} onKeyDown={moveWithKeyboard}>
        <div className={styles.selection} style={slideImageFrameCSS(draft ?? value)} data-frame-mode="move" aria-hidden="true">
          {corners.map(corner => <span key={corner} className={styles.handle} data-frame-mode={corner} />)}
        </div>
      </div>
    </div>
    <p id={instructionsID}>{copy.instructions}</p>
    <details>
      <summary>{copy.details}</summary>
      <div className={styles.fields}>
        {frameFields.map(field => <FrameField key={field} field={field} value={value[field]} onChange={next => onChange(setSlideImageFrameField(value, field, next))} />)}
      </div>
    </details>
    <div className={styles.actions}>
      <Button onClick={onNext}>{copy.next}</Button>
      <Button variant="outline" onClick={onCancel}>{copy.cancel}</Button>
    </div>
  </div>;
}

function FrameField({ field, value, onChange }: Readonly<{ field: keyof SlideImageFrame; value: number; onChange: (value: number) => void }>) {
  const [draft, setDraft] = useState<string>();
  return <label>
    <span>{copy.fields[field]}</span>
    <input type="number" min={field === "width" || field === "height" ? 5 : 0} max={100} step="any" value={draft ?? String(value / 100)}
      onChange={event => setDraft(event.currentTarget.value)} onBlur={event => {
        if (draft !== undefined) onChange(event.currentTarget.valueAsNumber * 100);
        setDraft(undefined);
      }} onKeyDown={event => { if (event.key === "Enter") event.currentTarget.blur(); }} />
  </label>;
}
