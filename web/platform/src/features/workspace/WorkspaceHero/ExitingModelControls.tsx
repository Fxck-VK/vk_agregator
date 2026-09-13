"use client";

import { useEffect, useLayoutEffect, useRef, type CSSProperties } from "react";

import {
  ImageGenerationControls,
  type ImageGenerationControlsProps,
} from "@/features/image-generation/ImageGenerationComposer/ImageGenerationControls";

import styles from "./WorkspaceHero.module.css";

const exitDuration = 220;

type ControlPosition = { left: number; top: number; width: number; height: number };
export type ExitingControlsSnapshot = {
  props: ImageGenerationControlsProps;
  height: number;
  restingHeight: number;
  positions: ControlPosition[];
};

export function captureExitingControls(
  controls: HTMLDivElement | null,
  props: ImageGenerationControlsProps,
): ExitingControlsSnapshot | null {
  const row = controls?.parentElement;
  if (!controls || !row || window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return null;

  const rowRect = row.getBoundingClientRect();
  return {
    props,
    height: rowRect.height,
    restingHeight: row.firstElementChild?.getBoundingClientRect().height ?? 0,
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
    // The spacer can then shrink without reflowing the fading buttons.
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
    const timer = window.setTimeout(onComplete, exitDuration);
    return () => window.clearTimeout(timer);
  }, [onComplete]);

  return (
    <div
      aria-hidden="true"
      className={styles.exitingControls}
      data-state="exiting"
      inert
      onAnimationEnd={(event) => {
        if (event.target === event.currentTarget) onComplete();
      }}
      ref={rootRef}
      style={{
        "--controls-exit-duration": `${exitDuration}ms`,
        "--controls-exit-height": `${snapshot.height}px`,
        "--controls-resting-height": `${snapshot.restingHeight}px`,
      } as CSSProperties}
    >
      <ImageGenerationControls {...snapshot.props} />
    </div>
  );
}
