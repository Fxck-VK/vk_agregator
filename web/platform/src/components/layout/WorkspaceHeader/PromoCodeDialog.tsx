"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";


import { useEffect, useRef, useState, type KeyboardEvent, type RefObject } from "react";

import { Button } from "@/components/ui/Button/Button";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";

import styles from "./PromoCodeDialog.module.css";

type PromoCodeDialogProps = {
  onClose: () => void;
  returnFocusRef: RefObject<HTMLButtonElement | null>;
};

export function PromoCodeDialog({ onClose, returnFocusRef }: PromoCodeDialogProps) {
  const msg = useMessages();
  const inputRef = useRef<HTMLInputElement>(null);
  const [code, setCode] = useState("");
  const [submitted, setSubmitted] = useState(false);

  useEffect(() => {
    const trigger = returnFocusRef.current;
    inputRef.current?.focus();
    return () => trigger?.focus();
  }, [returnFocusRef]);

  const keepFocusInside = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== "Tab") return;
    const controls = event.currentTarget.querySelectorAll<HTMLElement>("button:not(:disabled), input:not(:disabled)");
    const first = controls[0];
    const last = controls[controls.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  };

  return (
    <ModalBackdrop onClose={onClose} testId="promo-code-backdrop">
      {(requestClose) => (
        <section aria-labelledby="promo-code-title" aria-modal="true" className={styles.dialog} onKeyDown={keepFocusInside} role="dialog">
          <ModalCloseButton aria-label={msg("promoCodeDialog.closePromoCodeEntry")} className={styles.closeButtonPlacement} onClick={requestClose} />
          <h2 id="promo-code-title">{msg("promoCodeDialog.iHaveAPromoCode")}</h2>
          <form className={styles.form} onSubmit={(event) => {
            event.preventDefault();
            if (code.trim()) setSubmitted(true);
          }}>
            <InputSurface>
              <input
                aria-label={msg("promoCodeDialog.promoCode")}
                aria-describedby={submitted ? "promo-code-status" : undefined}
                autoComplete="off"
                autoCapitalize="off"
                className={styles.input}
                name="promo-code"
                onChange={(event) => {
                  setCode(event.target.value);
                  setSubmitted(false);
                }}
                placeholder={msg("promoCodeDialog.enterPromoCode")}
                ref={inputRef}
                spellCheck={false}
                type="text"
                value={code}
              />
            </InputSurface>
            {submitted ? <StateNotice inline kind="empty" id="promo-code-status">{msg("promoCodeDialog.promoCodeActivationIsNotAvailableYet")}</StateNotice> : null}
            <Button className={styles.activateButton} disabled={!code.trim()} type="submit">{msg("promoCodeDialog.activate")}</Button>
          </form>
        </section>
      )}
    </ModalBackdrop>
  );
}
