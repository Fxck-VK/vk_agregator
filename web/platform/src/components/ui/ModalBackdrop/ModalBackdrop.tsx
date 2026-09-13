"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./ModalBackdrop.module.css";

type ModalBackdropProps = {
  children: ReactNode | ((requestClose: () => void) => ReactNode);
  closeOnBackdropClick?: boolean;
  closeOnEscape?: boolean;
  onClose: () => void;
  testId?: string;
};

export function ModalBackdrop({
  children,
  closeOnBackdropClick = true,
  closeOnEscape = true,
  onClose,
  testId,
}: Readonly<ModalBackdropProps>) {
  const [state, setState] = useState<"open" | "closing">("open");
  const backdropRef = useRef<HTMLDivElement>(null);
  const scrollStateRef = useRef<{
    bodyOverflow: string;
    workspaceOverflow: string;
    workspaceScrollRegion: HTMLElement | null;
  } | null>(null);

  const restoreScrolling = useCallback(() => {
    const scrollState = scrollStateRef.current;
    if (scrollState === null) return;

    document.body.style.overflow = scrollState.bodyOverflow;
    if (scrollState.workspaceScrollRegion) {
      scrollState.workspaceScrollRegion.style.overflowY = scrollState.workspaceOverflow;
    }
    scrollStateRef.current = null;
  }, []);

  const finishClosing = useCallback(() => {
    restoreScrolling();
    onClose();
  }, [onClose, restoreScrolling]);

  const requestClose = useCallback(() => {
    setState((currentState) => (currentState === "open" ? "closing" : currentState));
  }, []);

  useEffect(() => {
    const workspaceScrollRegion = document.querySelector<HTMLElement>("[data-testid='workspace-scroll-region']");
    scrollStateRef.current = {
      bodyOverflow: document.body.style.overflow,
      workspaceOverflow: workspaceScrollRegion?.style.overflowY ?? "",
      workspaceScrollRegion,
    };

    document.body.style.overflow = "hidden";
    if (workspaceScrollRegion) {
      workspaceScrollRegion.style.overflowY = "hidden";
    }

    return () => {
      restoreScrolling();
    };
  }, [restoreScrolling]);

  useEffect(() => {
    const backdrop = backdropRef.current;
    if (!backdrop) return;

    const closeAfterAnimation = (event: globalThis.AnimationEvent) => {
      const isBackdropExit = !event.animationName || event.animationName.includes("modalBackdropOut");
      if (backdrop.dataset.state === "closing" && event.target === backdrop && isBackdropExit) {
        finishClosing();
      }
    };

    backdrop.addEventListener("animationend", closeAfterAnimation);
    return () => backdrop.removeEventListener("animationend", closeAfterAnimation);
  }, [finishClosing]);

  useEffect(() => {
    if (
      state === "closing" &&
      window.matchMedia?.("(prefers-reduced-motion: reduce)").matches
    ) {
      finishClosing();
    }
  }, [finishClosing, state]);

  useEffect(() => {
    if (!closeOnEscape) return;

    const closeFromEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation();
        requestClose();
      }
    };

    window.addEventListener("keydown", closeFromEscape);
    return () => window.removeEventListener("keydown", closeFromEscape);
  }, [closeOnEscape, requestClose]);

  const closeFromBackdrop = (event: MouseEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    const isBackdropSurface = event.target === event.currentTarget
      || target.dataset.modalBackdropViewport === "true";
    if (closeOnBackdropClick && isBackdropSurface) {
      requestClose();
    }
  };

  if (typeof document === "undefined") return null;

  return createPortal(
    <ScrollArea
      className={styles.backdrop}
      data-state={state}
      data-testid={testId}
      onMouseDown={closeFromBackdrop}
      ref={backdropRef}
      viewportClassName={styles.backdropViewport}
      viewportProps={{ "data-modal-backdrop-viewport": "true" }}
    >
      {typeof children === "function" ? children(requestClose) : children}
    </ScrollArea>,
    document.body,
  );
}
