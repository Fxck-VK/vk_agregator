"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { AccountMenu } from "@/features/account/AccountMenu/AccountMenu";
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
    <div className={styles.control}>
      <AccountMenu
        identityLabel={label ?? ru.account.unavailableLabel}
        isLogoutPending={isPending}
        logoutFailure={hasError ? ru.account.logoutFailure : undefined}
        onLogout={logout}
      />
    </div>
  );
}
