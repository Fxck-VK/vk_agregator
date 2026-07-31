"use client";

import { type FormEvent, useState } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import { safeReturnPath } from "@/lib/auth/return-path";
import { webBrowserFetch } from "@/lib/web-api/browser";

import styles from "./LoginForm.module.css";

export function LoginForm({ returnTo }: Readonly<{ returnTo?: string }>) {
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
      <div className={styles.field}>
        <label htmlFor="login-email">{ru.login.emailLabel}</label>
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
        <label htmlFor="login-password">{ru.login.passwordLabel}</label>
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
        <p className={styles.error} role="alert">
          {ru.login.failure}
        </p>
      ) : null}
      <Button disabled={isPending} type="submit">
        {isPending ? ru.login.pending : ru.login.submitLabel}
      </Button>
    </form>
  );
}
