"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { RetryAction } from "@/components/ui/AsyncState/RetryAction";
import { RichMessage } from "@/i18n/RichMessage";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";


import type { MessageKey } from "@/i18n/messages";

import { useMessages } from "@/i18n/LocaleProvider";


import Link from "@/i18n/Link";
import { useRouter } from "@/i18n/navigation";
import { useEffect, useRef, useState } from "react";
import { clearPendingPayment, clearPaymentAttempt, getPayment, PaymentRequestError, type Payment } from "./payments";
import styles from "./payments.module.css";

type Props = { paymentID: string; onDone?: () => void };
export function PaymentStatus({ paymentID, onDone }: Props) {
  const msg = useMessages();
  const { refresh } = useRouter();
  const refreshed = useRef<string | null>(null);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [message, setMessage] = useState<MessageKey | "">("");
  const [stopped, setStopped] = useState(false);
  const [notFound, setNotFound] = useState(false);
  const [retry, setRetry] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempts = 0;
    async function check() {
      try {
        const result = await getPayment(paymentID, controller.signal);
        if (controller.signal.aborted) return;
        setPayment(result);
        if (result.status === "succeeded") {
          clearPendingPayment(paymentID); clearPaymentAttempt();
          if (refreshed.current !== paymentID) { refreshed.current = paymentID; refresh(); }
          setStopped(true);
          return;
        }
        if (["canceled", "failed", "expired", "refunded", "partially_refunded"].includes(result.status)) {
          clearPendingPayment(paymentID); clearPaymentAttempt(); setStopped(true); return;
        }
        if (++attempts >= 40) {
          setMessage("paymentStatus.confirmationHasNotArrivedYetCheckThe"); setStopped(true); return;
        }
        timer = setTimeout(check, 3000);
      } catch (error) {
        if (controller.signal.aborted) return;
        if (error instanceof PaymentRequestError && error.status === 404) {
          setNotFound(true);
          clearPendingPayment(paymentID); clearPaymentAttempt();
          setMessage("paymentStatus.thisPaymentIsNotAvailableToThe");
        } else {
          setMessage("paymentStatus.couldNotCheckThePaymentPleaseTry");
        }
        setStopped(true);
      }
    }
    void check();
    return () => { controller.abort(); if (timer) clearTimeout(timer); };
  }, [paymentID, retry, refresh]);

  const succeeded = payment?.status === "succeeded";
  const canceled = payment && ["canceled", "failed", "expired"].includes(payment.status);
  const refunded = payment && ["refunded", "partially_refunded"].includes(payment.status);
  return (
    <div className={styles.status}>
      <StateNotice kind={succeeded ? "success" : stopped && !canceled && !refunded && !notFound ? "error" : canceled || refunded || notFound ? "info" : "loading"}>
        <h2>{succeeded ? msg("paymentStatus.paymentSuccessful") : msg("paymentStatus.paymentStatus")}</h2>
        <p>{succeeded ? <RichMessage id="paymentStatus.valueTokensCredited" values={{ value1: <CreditAmount value={payment.credits} /> }} />
          : canceled ? msg("paymentStatus.paymentCanceledNoTokensWereCredited")
          : refunded ? msg("paymentStatus.thisPaymentHasBeenRefunded")
          : (message ? msg(message) : null) || msg("paymentStatus.waitingForPaymentConfirmationYourBalanceWill")}</p>
      </StateNotice>
      {!succeeded && !canceled && !refunded && !notFound && stopped ? (
        <RetryAction label={msg("paymentStatus.checkPayment")} onClick={() => { setMessage(""); setStopped(false); setRetry((value) => value + 1); }} />
      ) : null}
      {payment?.status === "waiting_for_user" && payment.confirmation_url ? (
        <a className={styles.action} href={payment.confirmation_url}>{msg("paymentStatus.continuePaymentInYookassa")}</a>
      ) : null}
      {onDone && (succeeded || canceled || refunded || notFound) ? (
        <button className={styles.action} onClick={onDone} type="button">{msg("paymentStatus.viewTokenPacks")}</button>
      ) : <Link className={styles.action} href="/app">{msg("paymentStatus.returnToWorkspace")}</Link>}
    </div>
  );
}
