"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import { useSubscriptionPromotion } from "@/features/subscriptions/SubscriptionPromotion";


import { useEffect, useRef, useState, type KeyboardEvent, type RefObject } from "react";

import { Button } from "@/components/ui/Button/Button";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";

import styles from "./PromoCodeDialog.module.css";

type PromoCodeDialogProps = {
  onClose: () => void;
  returnFocusRef: RefObject<HTMLButtonElement | null>;
  preview?: boolean;
};

export function PromoCodeDialog({ onClose, returnFocusRef, preview = false }: PromoCodeDialogProps) {
  const msg = useMessages();
  const t = useDictionary().promotions, promotion = useSubscriptionPromotion();
  const cancel = promotion.cancel;
  const inputRef = useRef<HTMLInputElement>(null);
  const [code, setCode] = useState(promotion.quote?.code ?? "");
  const allowed = promotion.enabled && !preview;

  useEffect(() => {
    const trigger = returnFocusRef.current;
    inputRef.current?.focus();
    return () => { cancel(); trigger?.focus(); };
  }, [returnFocusRef, cancel]);

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
            if (allowed && !promotion.pending && code.trim()) void promotion.apply(code).then(applied => { if (applied) onClose(); });
          }}>
            <InputSurface>
              <input
                aria-label={msg("promoCodeDialog.promoCode")}
                aria-describedby="promo-code-status"
                autoComplete="off"
                autoCapitalize="off"
                className={styles.input}
                name="promo-code"
                maxLength={64}
                disabled={promotion.pending}
                onChange={(event) => {
                  setCode(event.target.value);
                  promotion.cancel();
                }}
                placeholder={msg("promoCodeDialog.enterPromoCode")}
                ref={inputRef}
                spellCheck={false}
                type="text"
                value={code}
              />
            </InputSurface>
            <div id="promo-code-status">
              {promotion.pending ? <StateNotice inline kind="loading">{t.checking}</StateNotice> : promotion.error ? <StateNotice inline kind="error">{t[promotion.error]}</StateNotice> : !allowed ? <StateNotice inline>{preview || promotion.preview ? t.preview : t.signIn}</StateNotice> : <p className={styles.status}>{t.terms}</p>}
            </div>
            <Button className={styles.activateButton} disabled={!allowed || promotion.pending || !/^[A-Za-z0-9_-]{3,64}$/.test(code.trim())} type="submit">{promotion.pending ? t.checking : msg("promoCodeDialog.activate")}</Button>
            {promotion.quote ? <Button variant="outline" disabled={promotion.pending} onClick={() => { promotion.clear(); onClose(); }}>{t.remove}</Button> : null}
          </form>
        </section>
      )}
    </ModalBackdrop>
  );
}
