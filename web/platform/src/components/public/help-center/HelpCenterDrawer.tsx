"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import styles from "./HelpCenter.module.css";

function DrawerSurface({ title, closeLabel, children, requestClose, id }: {
  title: string; closeLabel: string; id: string; children: (onNavigate: () => void) => ReactNode; requestClose: () => void;
}) {
  const surface = useRef<HTMLElement>(null);
  const close = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const background = document.querySelector<HTMLElement>("[data-help-center]");
    const previousInert = background?.inert ?? false;
    if (background) background.inert = true;
    close.current?.focus();
    const trap = (event: KeyboardEvent) => {
      if (event.key !== "Tab") return;
      const elements = Array.from(surface.current?.querySelectorAll<HTMLElement>("*") ?? []).filter(element => {
        if (!element.matches('button:not(:disabled),input:not(:disabled),a[href],summary,[tabindex="0"]')) return false;
        let ancestor = element.parentElement;
        while (ancestor && ancestor !== surface.current) {
          if (ancestor.tagName === "DETAILS" && !ancestor.hasAttribute("open") && !(element.tagName === "SUMMARY" && element.parentElement === ancestor)) return false;
          ancestor = ancestor.parentElement;
        }
        return true;
      });
      const first = elements[0], last = elements.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    const element = surface.current;
    element?.addEventListener("keydown", trap);
    return () => {
      element?.removeEventListener("keydown", trap);
      if (background) background.inert = previousInert;
      if (previous?.isConnected) previous.focus();
    };
  }, []);
  return <section id={id} ref={surface} className={styles.drawer} role="dialog" aria-modal="true" aria-labelledby={`${id}-title`}>
    <div className={styles.drawerHeading}><h2 id={`${id}-title`}>{title}</h2><ModalCloseButton ref={close} aria-label={closeLabel} onClick={requestClose} /></div>
    {children(requestClose)}
  </section>;
}

export function HelpCenterDrawer({ onClose, testId, ...props }: {
  title: string; closeLabel: string; id: string; children: (onNavigate: () => void) => ReactNode; onClose: () => void; testId: string;
}) {
  return <ModalBackdrop onClose={onClose} testId={testId}>{requestClose => <DrawerSurface {...props} requestClose={requestClose} />}</ModalBackdrop>;
}
