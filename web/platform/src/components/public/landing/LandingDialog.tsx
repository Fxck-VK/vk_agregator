"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import styles from "./LandingChrome.module.css";

type Presentation = "content" | "media";

function DialogSurface({ title, closeLabel, children, requestClose, wide, className, presentation }: { title: string; closeLabel: string; children: ReactNode; requestClose: () => void; wide: boolean; className?: string; presentation: Presentation }) {
  const ref = useRef<HTMLElement>(null);
  const id = useId();
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const focusable = () => Array.from(ref.current?.querySelectorAll<HTMLElement>('input,button,a[href],[tabindex="0"]') ?? []).filter(element => !element.hasAttribute("disabled"));
    (ref.current?.querySelector<HTMLElement>("input") ?? focusable()[0] ?? ref.current)?.focus();
    const trap = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const elements = focusable(); const first = elements[0]; const last = elements.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    const element = ref.current;
    element?.addEventListener("keydown", trap);
    return () => { element?.removeEventListener("keydown", trap); if (previous?.isConnected) previous.focus(); };
  }, []);
  return <section role="dialog" aria-modal="true" aria-label={presentation === "media" ? title : undefined} aria-labelledby={presentation === "content" ? id : undefined} tabIndex={-1} ref={ref} className={[styles.dialog, className].filter(Boolean).join(" ")} data-wide={wide || undefined} data-presentation={presentation}>
    {presentation === "media" ? <ModalCloseButton aria-label={closeLabel} onClick={requestClose} className={styles.dialogClose} /> : <div className={styles.dialogHeading}><h2 id={id}>{title}</h2><ModalCloseButton aria-label={closeLabel} onClick={requestClose} className={styles.dialogClose} /></div>}
    {children}
  </section>;
}

export function LandingDialog({ title, closeLabel, onClose, children, wide = false, className, presentation = "content" }: { title: string; closeLabel: string; onClose: () => void; children: ReactNode; wide?: boolean; className?: string; presentation?: Presentation }) {
  return <ModalBackdrop onClose={onClose} testId="landing-dialog-backdrop" className={presentation === "media" ? styles.mediaBackdrop : undefined}>{requestClose => <DialogSurface title={title} closeLabel={closeLabel} requestClose={requestClose} wide={wide} className={className} presentation={presentation}>{children}</DialogSurface>}</ModalBackdrop>;
}
