"use client";

import { RetryAction } from "@/components/ui/AsyncState/RetryAction";
import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import type { MessageKey } from "@/i18n/messages";
import { formatCurrency } from "@/i18n/format";

import { useMessages } from "@/i18n/LocaleProvider";


import Image from "next/image";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { assetPaths } from "@/assets/asset-paths";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { CreditAmount, getCreditAmountLabel } from "@/components/ui/CreditAmount/CreditAmount";
import selectableStyles from "@/components/ui/selectable-control.module.css";
import { PaymentStatus } from "@/features/payments/PaymentStatus";
import { checkoutNavigation } from "@/features/payments/checkout-navigation";
import { clearPaymentAttempt, createPayment, formatPaymentPrice, loadPaymentProducts, paymentAttemptKey, PaymentRequestError, readPendingPayment, savePendingPayment, type PaymentCatalog, type PaymentRequest } from "@/features/payments/payments";
import styles from "./TokenTopUpDialog.module.css";

type TokenTopUpDialogProps = { onClose: () => void };
function packageArtwork(credits: number) {
  const images = assetPaths.images.credits.packages;
  if (credits >= 20000) return images.tokens20000;
  if (credits >= 10000) return images.tokens10000;
  if (credits >= 3000) return images.tokens3000;
  if (credits >= 1500) return images.tokens1500;
  return images.tokens800;
}

export function TokenTopUpDialog({ onClose }: Readonly<TokenTopUpDialogProps>) {
  const msg = useMessages();
  const [catalog, setCatalog] = useState<PaymentCatalog | null>(null);
  const [selectedCode, setSelectedCode] = useState("");
  const [error, setError] = useState<MessageKey | "">("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [pendingID, setPendingID] = useState<string | null>(() => readPendingPayment());
  const [reload, setReload] = useState(0);
  const attempt = useRef<{ request: PaymentRequest; key: string } | null>(null);
  const submitting = useRef(false);
  const alive = useRef(true);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const titleId = useId();
  const selectedPackage = catalog?.items.find((item) => item.code === selectedCode);

  useEffect(() => {
    alive.current = true;
    const previouslyFocusedElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    closeButtonRef.current?.focus();
    return () => { alive.current = false; previouslyFocusedElement?.focus(); };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    loadPaymentProducts(controller.signal).then((result) => {
      if (controller.signal.aborted) return;
      setCatalog(result); setSelectedCode(result.items[0]?.code ?? ""); setLoading(false);
    }).catch(() => {
      if (controller.signal.aborted) return;
      setLoading(false); setError("tokenTopUpDialog.couldNotLoadTokenPacksPleaseTry");
    });
    return () => controller.abort();
  }, [reload]);

  function changeDetails() {
    if (attempt.current) clearPaymentAttempt();
    attempt.current = null;
    setError("");
  }
  async function purchase(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || !selectedPackage || !catalog?.checkout_available) return;
    submitting.current = true; setBusy(true); setError("");
    const request = { product_code: selectedPackage.code };
    attempt.current ??= { request, key: paymentAttemptKey(selectedPackage.code) };
    try {
      const payment = await createPayment(attempt.current.request, attempt.current.key);
      savePendingPayment(payment.id); clearPaymentAttempt();
      if (!alive.current) return;
      if (payment.status === "waiting_for_user" && payment.confirmation_url) {
        checkoutNavigation.open(payment.confirmation_url);
      } else {
        setPendingID(payment.id);
      }
    } catch (cause) {
      if (!alive.current) return;
      const status = cause instanceof PaymentRequestError ? cause.status : 0;
      setError(status === 401 || status === 403 ? "tokenTopUpDialog.signInAgainAndRetryThePurchase"
        : status === 409 ? "tokenTopUpDialog.purchaseDetailsHaveChangedSelectThePack"
        : status === 422 ? "tokenTopUpDialog.aVerifiedAccountEmailIsRequiredFor"
        : status === 429 ? "tokenTopUpDialog.tooManyRequestsRetryThePurchaseIn"
        : "tokenTopUpDialog.couldNotOpenCheckoutTryAgainThe");
    } finally {
      submitting.current = false;
      if (alive.current) setBusy(false);
    }
  }

  return (
    <ModalBackdrop onClose={onClose} testId="token-top-up-backdrop">
      {(requestClose) => (
        <div aria-labelledby={titleId} aria-modal="true" className={styles.dialogLayer} role="dialog">
          <ModalCloseButton
            aria-label={msg("tokenTopUpDialog.closeBalanceTopUp")}
            className={styles.closeButtonPlacement}
            onClick={requestClose}
            ref={closeButtonRef}
          />
          <section className={styles.dialog}>
            <header className={styles.header}>
              <div><h2 id={titleId}>{msg("tokenTopUpDialog.topUpTokens")}</h2><p>{msg("tokenTopUpDialog.chooseATokenPack")}</p></div>
            </header>
            {pendingID ? <PaymentStatus paymentID={pendingID} onDone={() => setPendingID(null)} /> : (
              <form className={styles.checkoutForm} onSubmit={purchase}>
                {loading ? <StateNotice inline kind="loading">{msg("tokenTopUpDialog.loadingTokenPacks")}</StateNotice> : null}
                <ScrollArea className={styles.packageScroll} viewportClassName={styles.packageViewport}>
                  <fieldset aria-label={msg("tokenTopUpDialog.tokenPacks")} className={styles.packageList} disabled={busy}>
                  {catalog?.items.map((item) => (
                    <label className={`${selectableStyles.control} ${styles.packageCard}`} key={item.code}>
                      <input aria-label={msg("tokenTopUpDialog.valueTokensForValue", { value1: getCreditAmountLabel(item.credits, undefined, msg), value2: formatPaymentPrice(item.amount, msg.locale) })} checked={item.code === selectedCode} className={styles.radioInput} name="token-package"
                        onChange={() => { changeDetails(); setSelectedCode(item.code); }} type="radio" value={item.code} />
                      <Image alt="" aria-hidden="true" className={styles.packageMark} height={48} sizes="48px" src={packageArtwork(item.credits)} width={48} />
                      <span className={styles.packageDetails}>
                        <strong className={styles.tokenLine}><CreditAmount value={item.credits} /></strong>
                        <span className={styles.priceLine}>{msg("billing.packagePrice", { price: formatCurrency(msg.locale, item.amount / 100, item.currency) })}</span>
                      </span>
                    </label>
                  ))}
                  </fieldset>
                </ScrollArea>
                {catalog?.items.length === 0 ? <StateNotice inline kind="empty">{msg("tokenTopUpDialog.noTokenPacksAreCurrentlyAvailable")}</StateNotice> : null}
                {catalog && !catalog.checkout_available ? <StateNotice inline kind="empty">{msg("tokenTopUpDialog.testPaymentsAreNotAvailableYet")}</StateNotice> : null}
                {selectedPackage ? <>
                  <button className={styles.purchaseButton} disabled={busy || !catalog?.checkout_available} type="submit">
                    {busy ? msg("tokenTopUpDialog.openingYookassa") : msg("tokenTopUpDialog.buyForValue", { value1: formatPaymentPrice(selectedPackage.amount, msg.locale) })}
                  </button>
                </> : null}
                {error ? <StateNotice inline kind="error">{msg(error)}</StateNotice> : null}
                {!catalog && !loading ? <RetryAction label={msg("tokenTopUpDialog.loadTokenPacks")} onClick={() => { setError(""); setLoading(true); setReload((value) => value + 1); }} /> : null}
              </form>
            )}
          </section>
        </div>
      )}
    </ModalBackdrop>
  );
}
