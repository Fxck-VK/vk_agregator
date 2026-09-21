"use client";

import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";
import { LanguageSwitcher } from "@/i18n/LanguageSwitcher";


import { type FormEvent, useState } from "react";
import { useRouter } from "@/i18n/navigation";

import { Button } from "@/components/ui/Button/Button";
import { safeReturnPath } from "@/lib/auth/return-path";
import { webBrowserFetch } from "@/lib/web-api/browser";

import styles from "./LoginForm.module.css";

export function LoginForm({ returnTo }: Readonly<{ returnTo?: string }>) {
  const t = useDictionary();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isPending, setIsPending] = useState(false);
  const [hasError, setHasError] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setHasError(false);
    setIsPending(true);

    try {
      const response = await webBrowserFetch("/web/v1/auth/password/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });

      if (response.ok) {
        router.replace(safeReturnPath(returnTo ?? "") ?? "/app");
      } else {
        setHasError(true);
      }
    } catch {
      setHasError(true);
    } finally {
      setPassword("");
      setIsPending(false);
    }
  };

  return (
    <form className={styles.form} onSubmit={submit}>
      <LanguageSwitcher />
      <div className={styles.field}>
        <label htmlFor="login-email">{t.login.emailLabel}</label>
        <input
          autoComplete="email"
          disabled={isPending}
          id="login-email"
          name="email"
          onChange={(event) => setEmail(event.target.value)}
          required
          type="email"
          value={email}
        />
      </div>
      <div className={styles.field}>
        <label htmlFor="login-password">{t.login.passwordLabel}</label>
        <input
          autoComplete="current-password"
          disabled={isPending}
          id="login-password"
          name="password"
          onChange={(event) => setPassword(event.target.value)}
          required
          type="password"
          value={password}
        />
      </div>
      {hasError ? (
        <StateNotice inline kind="error">
          {t.login.failure}
        </StateNotice>
      ) : null}
      <Button disabled={isPending} type="submit">
        {isPending ? <span aria-hidden="true"><LoadingIndicator label="" /></span> : null}
        {isPending ? t.login.pending : t.login.submitLabel}
      </Button>
    </form>
  );
}
