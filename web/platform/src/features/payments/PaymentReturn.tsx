"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages } from "@/i18n/LocaleProvider";


import Link from "@/i18n/Link";
import { useState, useSyncExternalStore } from "react";
import { z } from "zod";
import { PaymentStatus } from "./PaymentStatus";
import { readPendingPayment } from "./payments";
import styles from "./payments.module.css";

const subscribe = () => () => {};
const clientReady = () => true;
const serverReady = () => false;

export function PaymentReturn({ paymentID }: { paymentID?: string }) {
  const msg = useMessages();
  const ready = useSyncExternalStore(subscribe, clientReady, serverReady);
  return <section className={styles.page} aria-label={msg("paymentReturn.paymentResult")}>
    {ready ? <PaymentReturnReady key={paymentID ?? "stored"} paymentID={paymentID} /> : <StateNotice inline kind="loading">{msg("paymentReturn.checkingPayment")}</StateNotice>}
  </section>;
}

function PaymentReturnReady({ paymentID }: { paymentID?: string }) {
  const msg = useMessages();
  const [id] = useState(() => {
    const parsed = z.string().uuid().safeParse(paymentID);
    // The URL only identifies what to read. The Go API establishes ownership and status.
    return parsed.success ? parsed.data : readPendingPayment();
  });
  return (
    <>
      {id ? <PaymentStatus paymentID={id} /> : (
        <><h1>{msg("paymentReturn.noPendingPayment")}</h1><p>{msg("paymentReturn.yourBalanceIsAvailableInTheSite")}</p><Link className={styles.action} href="/app">{msg("paymentReturn.returnToWorkspace")}</Link></>
      )}
    </>
  );
}
