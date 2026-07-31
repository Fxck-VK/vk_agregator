"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import type { AccountProfile } from "@/lib/web-api/contracts";
import { webBrowserMutation } from "@/lib/web-api/browser";

import styles from "./AccountControl.module.css";

type AccountControlProps = {
  profile: AccountProfile;
};

export function AccountControl({ profile }: AccountControlProps) {
  const router = useRouter();
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);
  const identity = profile.identity_refs.find((candidate) => candidate.verified && candidate.label.trim() !== "");
  const label = identity?.label.trim();

  const logout = async () => {
    setHasError(false);
    setIsPending(true);

    try {
      const response = await webBrowserMutation("/web/v1/auth/logout", { method: "POST" });
      if (response.status === 204) {
        router.replace("/login");
      } else {
        setHasError(true);
      }
    } catch {
      setHasError(true);
    } finally {
      setIsPending(false);
    }
  };

  return (
    <section aria-labelledby="account-control-title" className={styles.control}>
      <h2 id="account-control-title">{ru.account.heading}</h2>
      <p className={styles.identity}>{label ?? ru.account.unavailableLabel}</p>
      {hasError ? (
        <p className={styles.error} role="alert">
          {ru.account.logoutFailure}
        </p>
      ) : null}
      <Button disabled={isPending} onClick={logout}>
        {isPending ? ru.account.logoutPending : ru.account.logoutLabel}
      </Button>
    </section>
  );
}
