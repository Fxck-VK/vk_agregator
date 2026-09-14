"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { clearPendingPayment, clearPaymentAttempt, formatPaymentNumber, getPayment, PaymentRequestError, type Payment } from "./payments";
import styles from "./payments.module.css";

type Props = { paymentID: string; onDone?: () => void };
export function PaymentStatus({ paymentID, onDone }: Props) {
  const { refresh } = useRouter();
  const refreshed = useRef<string | null>(null);
  const [payment, setPayment] = useState<Payment | null>(null);
  const [message, setMessage] = useState("");
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
          setMessage("Подтверждение ещё не получено. Проверьте оплату немного позже."); setStopped(true); return;
        }
        timer = setTimeout(check, 3000);
      } catch (error) {
        if (controller.signal.aborted) return;
        if (error instanceof PaymentRequestError && error.status === 404) {
          setNotFound(true);
          clearPendingPayment(paymentID); clearPaymentAttempt();
          setMessage("Этот платёж недоступен для текущего аккаунта.");
        } else {
          setMessage("Не удалось проверить оплату. Попробуйте ещё раз.");
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
      <div aria-live="polite" role="status">
        <h2>{succeeded ? "Оплата прошла" : "Статус оплаты"}</h2>
        <p>{succeeded ? `Начислено ${formatPaymentNumber(payment.credits)} токенов.`
          : canceled ? "Оплата отменена. Токены не начислены."
          : refunded ? "По платежу оформлен возврат."
          : message || "Ожидаем подтверждения оплаты. Баланс обновится после зачисления."}</p>
      </div>
      {!succeeded && !canceled && !refunded && !notFound && stopped ? (
        <button className={styles.action} onClick={() => { setMessage(""); setStopped(false); setRetry((value) => value + 1); }} type="button">Проверить оплату</button>
      ) : null}
      {payment?.status === "waiting_for_user" && payment.confirmation_url ? (
        <a className={styles.action} href={payment.confirmation_url}>Продолжить оплату в ЮKassa</a>
      ) : null}
      {onDone && (succeeded || canceled || refunded || notFound) ? (
        <button className={styles.action} onClick={onDone} type="button">К пакетам токенов</button>
      ) : <Link className={styles.action} href="/app">Вернуться к работе</Link>}
    </div>
  );
}
