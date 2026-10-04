"use client";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { classifySignInFailure, completeSignIn, reportSignInFailure, signInFailureText, type SignInStage } from "@/lib/auth/sign-in-completion";
import { isPasswordTooLong } from "@/lib/auth/password";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { CredentialField } from "./CredentialField";
import styles from "./LoginForm/LoginForm.module.css";

type Step = "email" | "code" | "password" | "session" | "preview-complete";
class RegistrationFailure extends Error { constructor(readonly status: number) { super("registration"); } }

export function EmailRegistration({ preview, returnTo, onBack }: Readonly<{ preview: boolean; returnTo?: string; onBack: () => void }>) {
  const t = useDictionary(); const locale = useLocale();
  const [step, setStep] = useState<Step>("email");
  const [email, setEmail] = useState(""); const [code, setCode] = useState("");
  const [password, setPassword] = useState(""); const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false); const [error, setError] = useState("");
  const [passwordRetry, setPasswordRetry] = useState(false);
  const [accepted, setAccepted] = useState<Response | null>(null);
  const [diagnosticId, setDiagnosticId] = useState("");
  const [signInStage, setSignInStage] = useState<SignInStage>("session");
  const [resendSeconds, setResendSeconds] = useState(0);
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  useEffect(() => {
    if (!resendSeconds) return;
    const timer = window.setTimeout(() => setResendSeconds(value => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [resendSeconds]);

  async function action(path: string, body: Record<string, string>, signal: AbortSignal) {
    if (preview) return;
    const response = await webBrowserFetch(`/web/v1/auth/email/${path}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal });
    if (!response.ok) throw new RegistrationFailure(response.status);
    return response;
  }
  async function perform(resend = false) {
    if (pending || request.current) return;
    if (step === "password") {
      if (isPasswordTooLong(password)) { setError(t.auth.passwordTooLong); return; }
      if (password.length < 8 || !password.trim()) { setError(t.auth.passwordHint); return; }
      if (password !== confirmation) { setError(t.auth.passwordMismatch); return; }
    }
    if (step === "code" && !resend && !/^\d{6}$/.test(code)) { setError(t.auth.registrationCodeFailure); return; }
    setPending(true); setError(""); setDiagnosticId("");
    const controller = new AbortController(); request.current = controller;
    const started = performance.now();
    let completionResponse = accepted;
    let completionStage: SignInStage = "session";
    let clearPassword = true;
    try {
      if (step === "email" || resend) {
        await action("request-code", { email }, controller.signal);
        if (controller.signal.aborted) return;
        setCode(""); setResendSeconds(60); setStep("code");
      } else if (step === "code") {
        await action("verify-code", { email, code }, controller.signal);
        if (controller.signal.aborted) return;
        setCode(""); setStep("password");
      } else if (step === "password") {
        completionResponse = await action("register", { email, password }, controller.signal) ?? null;
        if (controller.signal.aborted) return;
        if (preview) setStep("preview-complete");
        else { setAccepted(completionResponse); setStep("session"); setPassword(""); setConfirmation(""); }
      }
      if (completionResponse && (step === "password" || step === "session")) {
        await completeSignIn(completionResponse, { locale, returnTo, signal: controller.signal, onStage: next => { completionStage = next; setSignInStage(next); } });
      }
    } catch (failure) {
      if (controller.signal.aborted) return;
      if (completionResponse) {
        const diagnostic = classifySignInFailure(failure, completionStage);
        reportSignInFailure(diagnostic, started);
        setError(signInFailureText(diagnostic, t.auth, t.login.failure)); setDiagnosticId(diagnostic.diagnosticId);
        return;
      }
      const status = failure instanceof RegistrationFailure ? failure.status : 0;
      if (step === "password" && (status === 0 || status >= 500 || status === 429)) {
        // The account may already exist while session issuance failed. Keep the
        // first submitted password for retry; never imply a new one replaces it.
        clearPassword = false; setPasswordRetry(true);
        setError(status === 429 ? t.auth.registrationRateLimited : t.auth.registrationRetryPassword);
        return;
      }
      setError(status === 429 ? t.auth.registrationRateLimited : status === 409 ? t.auth.registrationExisting : step === "code" && status === 400 ? t.auth.registrationCodeFailure : step === "password" && status === 400 ? t.auth.registrationExpired : t.auth.registrationFailure);
    } finally {
      request.current = null;
      if (step === "password" && clearPassword) { setPassword(""); setConfirmation(""); setPasswordRetry(false); }
      if (!controller.signal.aborted) setPending(false);
    }
  }
  function restart() { setStep("email"); setCode(""); setPassword(""); setConfirmation(""); setPasswordRetry(false); setError(""); setAccepted(null); setDiagnosticId(""); }
  function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); void perform(); }

  return <>
    <header className={styles.heading}><h1 id="login-title">{t.auth.registrationTitle}</h1><p>{t.auth.registrationDescription}</p></header>
    {preview ? <p className={styles.hint}>{t.auth.preview}</p> : null}
    {step === "preview-complete" ? <><p role="status">{t.auth.registrationPreviewComplete}</p><Button onClick={onBack}>{t.auth.back}</Button></> : <form className={styles.fields} onSubmit={submit}>
      <CredentialField label={t.login.emailLabel} id="signup-email" autoComplete="email" type="email" maxLength={254} required disabled={pending || step !== "email"} value={email} onChange={event => setEmail(event.target.value)} />
      {step === "code" ? <><p className={styles.hint} role="status">{preview ? t.auth.registrationPreviewCode : t.auth.registrationCodeSent}</p><CredentialField label={t.auth.code} id="signup-code" autoComplete="one-time-code" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required disabled={pending} value={code} onChange={event => setCode(event.target.value)} /></> : null}
      {step === "password" ? <><p className={styles.hint}>{preview ? t.auth.registrationPreviewPassword : t.auth.registrationEmailVerified}</p><CredentialField label={t.login.passwordLabel} hint={t.auth.passwordHint} id="signup-password" autoComplete="new-password" type="password" minLength={8} maxLength={256} required disabled={pending} readOnly={passwordRetry} value={password} onChange={event => setPassword(event.target.value)} /><CredentialField label={t.auth.confirmPassword} id="signup-confirm-password" autoComplete="new-password" type="password" minLength={8} maxLength={256} required disabled={pending} readOnly={passwordRetry} value={confirmation} onChange={event => setConfirmation(event.target.value)} /></> : null}
      {error ? <StateNotice inline kind="error">{error}{diagnosticId ? <small className={styles.diagnostic}>{t.auth.loginDiagnostic} {diagnosticId}</small> : null}</StateNotice> : null}
      <Button type="submit" disabled={pending}>{pending ? <><span aria-hidden="true"><LoadingIndicator label="" /></span>{accepted ? signInStage === "navigation" ? t.auth.loginOpening : t.auth.loginConfirming : t.auth.pending}</> : step === "session" ? t.auth.loginRetry : step === "email" ? t.auth.sendCode : step === "code" ? t.auth.verifyRegistrationEmail : t.auth.createAccount}</Button>
      <div className={styles.actions}><button type="button" className={styles.textAction} disabled={pending} onClick={onBack}>{t.auth.back}</button>{step !== "email" && step !== "session" ? <button type="button" className={styles.textAction} disabled={pending} onClick={restart}>{step === "password" ? t.auth.registrationRestart : t.auth.changeEmail}</button> : null}</div>
      {step === "code" ? <button type="button" className={styles.textAction} disabled={pending || resendSeconds > 0} onClick={() => void perform(true)}>{t.auth.resend}{resendSeconds ? ` (${resendSeconds})` : ""}</button> : null}
    </form>}
  </>;
}
