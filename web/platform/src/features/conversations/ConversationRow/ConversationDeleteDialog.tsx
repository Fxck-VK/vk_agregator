"use client";

import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";


import {
  type JSX,
  type RefObject,
  useEffect,
  useId,
  useRef,
} from "react";
import { createPortal } from "react-dom";

import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";

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
  const t = useDictionary();
  const titleID = useId();
  const leadID = useId();
  const descriptionID = useId();
  const dialogRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (isPending) return;

    const handleKeyDown = (event: KeyboardEvent) => {
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
  }, [isPending]);

  if (typeof document === "undefined") return null;

  return createPortal(
    <ModalBackdrop closeOnBackdropClick={!isPending} closeOnEscape={!isPending} onClose={onCancel}>
      {(requestClose) => <section
        aria-describedby={`${leadID} ${descriptionID}`}
        aria-labelledby={titleID}
        aria-modal="true"
        className={styles.deleteDialog}
        ref={dialogRef}
        role="dialog"
      >
        <h2 id={titleID}>{t.conversations.archiveDialogTitle}</h2>
        <p id={leadID}>{t.conversations.archiveDialogLead} <strong>{conversationTitle}</strong>.</p>
        <p className={styles.dialogDescription} id={descriptionID}>{t.conversations.archiveConfirmation}</p>
        {errorMessage === undefined ? null : <StateNotice inline kind="error">{errorMessage}</StateNotice>}
        <div className={styles.dialogActions}>
          <button className={styles.dialogCancel} disabled={isPending} onClick={requestClose} type="button">
            {t.conversations.cancelLabel}
          </button>
          <button
            aria-describedby={`${leadID} ${descriptionID}`}
            className={styles.dialogDelete}
            disabled={isPending}
            onClick={onConfirm}
            ref={confirmRef}
            type="button"
          >
            {isPending ? <><span aria-hidden="true"><LoadingIndicator label="" /></span>{t.conversations.archivePending}</> : t.conversations.archiveConfirmLabel}
          </button>
        </div>
      </section>}
    </ModalBackdrop>,
    document.body,
  );
}
