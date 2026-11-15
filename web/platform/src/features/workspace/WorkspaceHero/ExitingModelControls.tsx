"use client";

import { useEffect, useLayoutEffect, useRef, type CSSProperties } from "react";
import { composerLayoutDuration, composerLayoutEasing } from "@/components/chat/ChatComposer/composer-motion";

import {
  ImageGenerationControls,
  type ImageGenerationControlsProps,
} from "@/features/image-generation/ImageGenerationComposer/ImageGenerationControls";

import styles from "./WorkspaceHero.module.css";

type ControlPosition = { left: number; top: number; width: number; height: number };
export type ExitingControlsSnapshot = {
  props: ImageGenerationControlsProps;
  left: number;
  top: number;
  positions: ControlPosition[];
};

export function captureExitingControls(
  controls: HTMLDivElement | null,
  props: ImageGenerationControlsProps,
): ExitingControlsSnapshot | null {
  const row = controls?.parentElement;
  const surface = controls?.closest<HTMLElement>('[data-ui="input-surface"]');
  if (!controls || !row || !surface || window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return null;

  const rowRect = row.getBoundingClientRect();
  const surfaceRect = surface.getBoundingClientRect();
  return {
    props,
    left: rowRect.left - surfaceRect.left - surface.clientLeft,
    top: rowRect.top - surfaceRect.top - surface.clientTop,
    positions: Array.from(controls.children, (control) => {
      const rect = control.getBoundingClientRect();
      return { left: rect.left - rowRect.left, top: rect.top - rowRect.top, width: rect.width, height: rect.height };
    }),
  };
}

export function ExitingModelControls({ snapshot, onComplete }: {
  snapshot: ExitingControlsSnapshot;
  onComplete: () => void;
}) {
  const rootRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    // Freeze each button at its previous position, including wrapped rows.
    // Fade out of flow while the surface contracts in the same transition.
    Array.from(rootRef.current?.children ?? []).forEach((control, index) => {
      const position = snapshot.positions[index];
      if (!(control instanceof HTMLElement) || !position) return;
      Object.assign(control.style, {
        position: "absolute",
        left: `${position.left}px`,
        top: `${position.top}px`,
        width: `${position.width}px`,
        height: `${position.height}px`,
        margin: "0",
      });
    });
  }, [snapshot]);

  useEffect(() => {
    const timer = window.setTimeout(onComplete, composerLayoutDuration);
    return () => window.clearTimeout(timer);
  }, [onComplete]);

  return (
    <div
      aria-hidden="true"
      className={styles.exitingControls}
      data-state="exiting"
      inert
      ref={rootRef}
      style={{
        left: snapshot.left,
        top: snapshot.top,
        "--controls-exit-duration": `${composerLayoutDuration}ms`,
        "--controls-exit-easing": composerLayoutEasing,
      } as CSSProperties}
    >
      <ImageGenerationControls {...snapshot.props} />
    </div>
  );
}
