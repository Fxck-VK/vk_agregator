"use client";

import {
  type JSX,
  type MouseEvent,
  type RefObject,
  useEffect,
  useId,
  useRef,
} from "react";
import { createPortal } from "react-dom";

import { ru } from "@/i18n/ru";

import styles from "./ConversationRow.module.css";

type ConversationDeleteDialogProps = {
  confirmRef: RefObject<HTMLButtonElement | null>;
  conversationTitle: string;
  errorMessage?: string;
  isPending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConversationDeleteDialog({
  confirmRef,
  conversationTitle,
  errorMessage,
  isPending,
  onCancel,
  onConfirm,
}: ConversationDeleteDialogProps): JSX.Element | null {
  const titleID = useId();
  const leadID = useId();
  const descriptionID = useId();
  const dialogRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, []);

  useEffect(() => {
    if (isPending) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation();
        onCancel();
        return;
      }
      if (event.key !== "Tab") return;

      const focusable = Array.from(dialogRef.current?.querySelectorAll<HTMLButtonElement>("button:not(:disabled)") ?? []);
      const first = focusable[0];
      const last = focusable.at(-1);
      if (first === undefined || last === undefined) return;
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isPending, onCancel]);

  if (typeof document === "undefined") return null;

  const closeFromBackdrop = (event: MouseEvent<HTMLDivElement>) => {
    if (!isPending && event.currentTarget === event.target) onCancel();
  };

  return createPortal(
    <div className={styles.dialogBackdrop} onMouseDown={closeFromBackdrop}>
      <section
        aria-describedby={`${leadID} ${descriptionID}`}
        aria-labelledby={titleID}
        aria-modal="true"
        className={styles.deleteDialog}
        ref={dialogRef}
        role="dialog"
      >
        <h2 id={titleID}>{ru.conversations.archiveDialogTitle}</h2>
        <p id={leadID}>{ru.conversations.archiveDialogLead} <strong>{conversationTitle}</strong>.</p>
        <p className={styles.dialogDescription} id={descriptionID}>{ru.conversations.archiveConfirmation}</p>
        {errorMessage === undefined ? null : <p className={styles.dialogError} role="alert">{errorMessage}</p>}
        <div className={styles.dialogActions}>
          <button className={styles.dialogCancel} disabled={isPending} onClick={onCancel} type="button">
            {ru.conversations.cancelLabel}
          </button>
          <button
            aria-describedby={`${leadID} ${descriptionID}`}
            className={styles.dialogDelete}
            disabled={isPending}
            onClick={onConfirm}
            ref={confirmRef}
            type="button"
          >
            {isPending ? ru.conversations.archivePending : ru.conversations.archiveConfirmLabel}
          </button>
        </div>
      </section>
    </div>,
    document.body,
  );
}
