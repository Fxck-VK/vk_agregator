"use client";

import { useEffect, useId, useRef } from "react";
import { Button } from "@/components/ui/Button/Button";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { useMessages } from "@/i18n/LocaleProvider";
import styles from "./ChatAttachmentNotice.module.css";

export function ChatAttachmentNotice({ onClose }: { onClose: () => void }) {
  const msg = useMessages();
  const titleId = useId();
  const descriptionId = useId();
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const previous = document.activeElement;
    buttonRef.current?.focus();
    const keepFocus = (event: FocusEvent) => {
      if (event.target !== buttonRef.current) buttonRef.current?.focus();
    };
    document.addEventListener("focusin", keepFocus);
    return () => {
      document.removeEventListener("focusin", keepFocus);
      if (previous instanceof HTMLElement && previous.isConnected) previous.focus();
    };
  }, []);

  return (
    <ModalBackdrop onClose={onClose}>
      {requestClose => (
        <section
          aria-describedby={descriptionId}
          aria-labelledby={titleId}
          aria-modal="true"
          className={styles.dialog}
          onKeyDown={event => {
            if (event.key === "Tab") {
              event.preventDefault();
              buttonRef.current?.focus();
            }
          }}
          role="alertdialog"
        >
          <h2 id={titleId}>{msg("chatAttachmentNotice.alreadyAttached")}</h2>
          <p id={descriptionId}>{msg("chatAttachmentNotice.chooseAnother")}</p>
          <Button className={styles.dismiss} onClick={requestClose} ref={buttonRef} variant="outline">
            {msg("chatAttachmentNotice.dismiss")}
          </Button>
        </section>
      )}
    </ModalBackdrop>
  );
}
