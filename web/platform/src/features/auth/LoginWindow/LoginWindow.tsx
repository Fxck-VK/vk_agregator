"use client";

import { type ReactNode, useEffect } from "react";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { useDictionary } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";
import { LoginShowcase } from "./LoginShowcase";
import styles from "./LoginWindow.module.css";

export function LoginWindow({ children }: Readonly<{ children: ReactNode }>) {
  const t = useDictionary();
  const router = useRouter();
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !event.defaultPrevented) router.replace("/app");
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [router]);

  return <div className={styles.window} data-testid="login-window">
    <ModalCloseButton className={styles.close} aria-label={t.auth.closeWindow} onClick={() => router.replace("/app")} />
    <div className={styles.account}>
      <div className={styles.formArea}>{children}</div>
    </div>
    <LoginShowcase />
  </div>;
}
