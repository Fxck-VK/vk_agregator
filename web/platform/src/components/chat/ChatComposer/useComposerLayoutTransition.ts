"use client";

import { useEffect, useLayoutEffect, useRef, type RefObject } from "react";
import { composerLayoutDuration, composerLayoutEasing } from "./composer-motion";

const partSelectors = ['[data-ui="chat-composer-field"]', '[data-ui="chat-composer-trailing-controls"]'];
type Position = { x: number; y: number };
type Layout = { compact: boolean; controls: Element[]; height: number; value: string; positions: Position[] };
type Transition = { surface: Animation; parts: Animation[] };

function measure(surface: HTMLElement) {
  const rect = surface.getBoundingClientRect();
  return {
    height: rect.height,
    positions: partSelectors.map((selector, index) => {
      const element = surface.querySelector(selector);
      const part = (index === 0 ? element?.querySelector("textarea") : element)?.getBoundingClientRect();
      return { x: (part?.left ?? rect.left) - rect.left, y: (part?.top ?? rect.top) - rect.top };
    }),
  };
}

function cancel(transition: Transition | null) {
  transition?.surface.cancel();
  transition?.parts.forEach(animation => animation.cancel());
}

function currentLayout(surface: HTMLElement, previous: Layout) {
  return {
    height: surface.getBoundingClientRect().height,
    // React has already changed the grid. Recover the visible position from
    // the previous grid's target plus its still-running translation.
    positions: partSelectors.map((selector, index) => {
      const part = surface.querySelector(selector);
      const translate = part ? getComputedStyle(part).translate.split(" ") : [];
      return {
        x: previous.positions[index].x + (Number.parseFloat(translate[0]) || 0),
        y: previous.positions[index].y + (Number.parseFloat(translate[1]) || 0),
      };
    }),
  };
}

export function useComposerLayoutTransition(
  surfaceRef: RefObject<HTMLDivElement | null>,
  leadingRef: RefObject<HTMLDivElement | null>,
  additionalRef: RefObject<HTMLDivElement | null>,
  hasAttachments: boolean,
  value: string,
) {
  const previousRef = useRef<Layout | null>(null);
  const animationRef = useRef<Transition | null>(null);

  useLayoutEffect(() => {
    const surface = surfaceRef.current;
    if (!surface) return;
    const controls = [
      ...Array.from(leadingRef.current?.children ?? []).filter(child => child.getAttribute("data-state") !== "exiting"),
      ...Array.from(additionalRef.current?.children ?? []),
    ];
    const compact = !hasAttachments && controls.length === 0;
    const previous = previousRef.current;
    const changed = previous !== null && previous.value === value && (previous.compact !== compact
      || controls.length !== previous.controls.length || controls.some((control, index) => control !== previous.controls[index]));
    // Placeholder updates must not interrupt the layout animation. Actual input
    // keeps its existing immediate resizing behavior and cancels a running tween.
    if (!changed && previous?.value === value && animationRef.current) return;
    const from = animationRef.current && previous ? currentLayout(surface, previous) : previous;
    const active = animationRef.current;
    animationRef.current = null;
    cancel(active);
    const target = measure(surface);
    previousRef.current = { compact, controls, value, ...target };
    if (!changed || !from || Math.abs(target.height - from.height) < 1
      || typeof surface.animate !== "function"
      || window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;

    const timing = { duration: composerLayoutDuration, easing: composerLayoutEasing };
    const transition: Transition = {
      surface: surface.animate([
        { blockSize: `${from.height}px`, overflow: "hidden" },
        { blockSize: `${target.height}px`, overflow: "hidden" },
      ], timing),
      parts: [],
    };
    // Keep the text and send action continuous when the grid changes rows.
    partSelectors.forEach((selector, index) => {
      const part = surface.querySelector<HTMLElement>(selector);
      const x = from.positions[index].x - target.positions[index].x;
      const y = from.positions[index].y - target.positions[index].y;
      if (part && (Math.abs(x) >= 1 || Math.abs(y) >= 1)) {
        transition.parts.push(part.animate([{ translate: `${x}px ${y}px` }, { translate: "0px 0px" }], timing));
      }
    });
    animationRef.current = transition;
    const finish = () => {
      if (animationRef.current !== transition) return;
      animationRef.current = null;
      if (previousRef.current) Object.assign(previousRef.current, measure(surface));
    };
    transition.surface.onfinish = finish;
  });

  useEffect(() => {
    const surface = surfaceRef.current;
    const observer = surface && typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => {
      if (!animationRef.current && previousRef.current) Object.assign(previousRef.current, measure(surface));
    }) : null;
    if (surface) observer?.observe(surface);
    return () => { observer?.disconnect(); cancel(animationRef.current); };
  }, [surfaceRef]);
}
