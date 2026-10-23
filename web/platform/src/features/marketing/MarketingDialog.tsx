"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { useDictionary } from "@/i18n/LocaleProvider";
import styles from "./MarketingHome.module.css";

export function MarketingDialog({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  const { marketing: t } = useDictionary();
  const surface = useRef<HTMLElement>(null);
  useEffect(() => {
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const dialog = surface.current;
    const controls = () => Array.from(dialog?.querySelectorAll<HTMLElement>('input, button:not(:disabled), a[href], video[controls]') ?? []);
    (dialog?.querySelector<HTMLElement>("input") ?? controls()[0] ?? dialog)?.focus();
    const containFocus = (event: FocusEvent) => {
      if (event.target instanceof Node && dialog && !dialog.contains(event.target)) (controls()[0] ?? dialog).focus();
    };
    document.addEventListener("focusin", containFocus);
    return () => { document.removeEventListener("focusin", containFocus); if (trigger?.isConnected) trigger.focus(); };
  }, []);
  return <ModalBackdrop onClose={onClose} testId="marketing-dialog-backdrop">{requestClose =>
    <section ref={surface} className={styles.dialog} role="dialog" aria-modal="true" aria-label={title} tabIndex={-1} onKeyDown={event => {
      if (event.key !== "Tab") return;
      const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>('input, button:not(:disabled), a[href], video[controls]'));
      const first = controls[0], last = controls.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }}>
      <ModalCloseButton className={styles.dialogClose} aria-label={t.close} onClick={requestClose} />
      <h2 className={styles.dialogTitle}>{title}</h2>{children}
    </section>
  }</ModalBackdrop>;
}
