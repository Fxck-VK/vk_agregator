"use client";

import Link from "next/link";
import { useState, useSyncExternalStore } from "react";
import { z } from "zod";
import { PaymentStatus } from "./PaymentStatus";
import { readPendingPayment } from "./payments";
import styles from "./payments.module.css";

const subscribe = () => () => {};
const clientReady = () => true;
const serverReady = () => false;

export function PaymentReturn({ paymentID }: { paymentID?: string }) {
  const ready = useSyncExternalStore(subscribe, clientReady, serverReady);
  return <section className={styles.page} aria-label="Результат оплаты">
    {ready ? <PaymentReturnReady key={paymentID ?? "stored"} paymentID={paymentID} /> : <p role="status">Проверяем оплату…</p>}
  </section>;
}

function PaymentReturnReady({ paymentID }: { paymentID?: string }) {
  const [id] = useState(() => {
    const parsed = z.string().uuid().safeParse(paymentID);
    // The URL only identifies what to read. The Go API establishes ownership and status.
    return parsed.success ? parsed.data : readPendingPayment();
  });
  return (
    <>
      {id ? <PaymentStatus paymentID={id} /> : (
        <><h1>Нет ожидающей оплаты</h1><p>Баланс доступен в верхней панели сайта.</p><Link className={styles.action} href="/app">Вернуться к работе</Link></>
      )}
    </>
  );
}
