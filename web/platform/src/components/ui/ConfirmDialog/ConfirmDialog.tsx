"use client";

import { useId, useLayoutEffect, useRef, type ReactNode, type RefObject } from "react";

import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";

import styles from "./ConfirmDialog.module.css";

type ConfirmDialogProps = {
  title: ReactNode;
  lead?: ReactNode;
  description: ReactNode;
  cancelLabel: string;
  confirmLabel: string;
  pendingLabel: string;
  isPending?: boolean;
  confirmDisabled?: boolean;
  intent?: "destructive" | "primary";
  errorMessage?: string;
  onCancel: () => void;
  onConfirm: () => void;
  cancelRef?: RefObject<HTMLButtonElement | null>;
  returnFocusRef?: RefObject<HTMLElement | null>;
  restoreFocus?: boolean;
};

export function ConfirmDialog({
  title,
  lead,
  description,
  cancelLabel,
  confirmLabel,
  pendingLabel,
  isPending = false,
  confirmDisabled = false,
  intent = "destructive",
  errorMessage,
  onCancel,
  onConfirm,
  cancelRef,
  returnFocusRef,
  restoreFocus = true,
}: Readonly<ConfirmDialogProps>) {
  const titleID = useId();
  const leadID = useId();
  const descriptionID = useId();
  const dialogRef = useRef<HTMLElement>(null);
  const internalCancelRef = useRef<HTMLButtonElement>(null);
  const cancelButtonRef = cancelRef ?? internalCancelRef;
  const describedBy = [lead != null && leadID, descriptionID].filter(Boolean).join(" ");

  useLayoutEffect(() => {
    if (!restoreFocus) return;
    const previousFocus = document.activeElement;
    const dialog = dialogRef.current;
    const returnTarget = returnFocusRef?.current ?? previousFocus;
    return () => {
      const focusStillOwned = document.activeElement === document.body
        || (document.activeElement !== null && dialog?.contains(document.activeElement));
      if (focusStillOwned && returnTarget instanceof HTMLElement && returnTarget.isConnected) returnTarget.focus();
    };
  }, [restoreFocus, returnFocusRef]);

  useLayoutEffect(() => {
    if (isPending) dialogRef.current?.focus();
    else cancelButtonRef.current?.focus();
  }, [cancelButtonRef, isPending]);

  return (
    <ModalBackdrop closeOnBackdropClick={!isPending} closeOnEscape={!isPending} onClose={onCancel}>
      {(requestClose) => (
        <section
          aria-busy={isPending || undefined}
          aria-describedby={describedBy}
          aria-labelledby={titleID}
          aria-modal="true"
          className={styles.dialog}
          data-intent={intent}
          onKeyDown={(event) => {
            if (event.key !== "Tab") return;
            const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("*")).filter(control =>
              control instanceof HTMLButtonElement ? !control.disabled : control instanceof HTMLAnchorElement && control.hasAttribute("href"));
            const first = controls[0];
            const last = controls.at(-1);
            if (!first || !last) {
              event.preventDefault();
              dialogRef.current?.focus();
            } else if (event.shiftKey && (document.activeElement === first || document.activeElement === dialogRef.current)) {
              event.preventDefault();
              last.focus();
            } else if (!event.shiftKey && (document.activeElement === last || document.activeElement === dialogRef.current)) {
              event.preventDefault();
              first.focus();
            }
          }}
          ref={dialogRef}
          role="dialog"
          tabIndex={-1}
        >
          <h2 id={titleID}>{title}</h2>
          {lead != null ? <p id={leadID}>{lead}</p> : null}
          <p className={lead != null ? styles.description : undefined} id={descriptionID}>{description}</p>
          {errorMessage ? <div className={styles.error}><StateNotice inline kind="error">{errorMessage}</StateNotice></div> : null}
          <div className={styles.actions}>
            <Button className={styles.cancel} disabled={isPending} onClick={requestClose} ref={cancelButtonRef} variant="outline">
              {cancelLabel}
            </Button>
            <Button aria-describedby={describedBy} className={styles.confirm} disabled={isPending || confirmDisabled} onClick={onConfirm}>
              {isPending ? <><span aria-hidden="true"><LoadingIndicator label="" /></span>{pendingLabel}</> : confirmLabel}
            </Button>
          </div>
        </section>
      )}
    </ModalBackdrop>
  );
}
