"use client";

import { BrandText } from "@/components/brand/BrandName/BrandName";
import type { LegalMetadata } from "@/features/legal/model";
import { LegalNotice } from "@/features/legal/LegalLinks";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { classifySignInFailure, completeSignIn, reportSignInFailure, signInFailureText, type SignInStage } from "@/lib/auth/sign-in-completion";
import { isPasswordTooLong } from "@/lib/auth/password";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { CredentialField } from "./CredentialField";
import { PasswordCreationField } from "./PasswordCreationField";
import { VerificationCodeField } from "./VerificationCodeField";
import styles from "./LoginForm/LoginForm.module.css";

type Step = "email" | "code" | "password" | "session" | "preview-complete";
class RegistrationFailure extends Error { constructor(readonly status: number, readonly documentsChanged = false) { super("registration"); } }

export function EmailRegistration({ preview, returnTo, onBack, onSignIn, onEmailChange, title, legalMetadata, initialEmail = "", passwordFirst = false }: Readonly<{ preview: boolean; returnTo?: string; onBack: () => void; onSignIn?: (email: string) => void; onEmailChange?: (email: string) => void; title?: string; legalMetadata?: LegalMetadata; initialEmail?: string; passwordFirst?: boolean }>) {
  const t = useDictionary(); const locale = useLocale();
  const [step, setStep] = useState<Step>("email");
  const [email, setEmail] = useState(initialEmail); const [code, setCode] = useState("");
  const [password, setPassword] = useState(""); const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false); const [error, setError] = useState("");
  const [passwordRetry, setPasswordRetry] = useState(false);
  const [accepted, setAccepted] = useState<Response | null>(null);
  const [diagnosticId, setDiagnosticId] = useState("");
  const [signInStage, setSignInStage] = useState<SignInStage>("session");
  const [resendSeconds, setResendSeconds] = useState(0);
  const request = useRef<AbortController | null>(null);
  const credentialsValid = password.length >= 8 && Boolean(password.trim()) && !isPasswordTooLong(password) && /^[^\s@]+@[^\s@]+\.[^\s@]+$/u.test(email.trim());
  useEffect(() => () => request.current?.abort(), []);
  useEffect(() => {
    if (!resendSeconds) return;
    const timer = window.setTimeout(() => setResendSeconds(value => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [resendSeconds]);

  async function action(path: string, body: Record<string, unknown>, signal: AbortSignal) {
    if (preview) return;
    const response = await webBrowserFetch(`/web/v1/auth/email/${path}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal: AbortSignal.any([signal, AbortSignal.timeout(15_000)]) });
    if (!response.ok) throw new RegistrationFailure(response.status, response.headers.get("X-NeiroHub-Legal-Revision") === "changed");
    return response;
  }
  async function perform(resend = false, submittedCode = code) {
    if (pending || request.current) return;
    if (step === "password" || (passwordFirst && !resend && (step === "email" || step === "code"))) {
      if (isPasswordTooLong(password)) { setError(t.auth.passwordTooLong); return; }
      if (password.length < 8 || !password.trim()) { setError(t.auth.passwordHint); return; }
      if (!passwordFirst && password !== confirmation) { setError(t.auth.passwordMismatch); return; }
    }
    if (step === "code" && !resend && !/^\d{6}$/.test(submittedCode)) { setError(t.auth.registrationCodeFailure); return; }
    setPending(true); setError(""); setDiagnosticId("");
    const controller = new AbortController(); request.current = controller;
    const started = performance.now();
    let completionResponse = accepted;
    let completionStage: SignInStage = "session";
    let clearPassword = true;
    let registrationAttempted = false;
    let shouldRegister = step === "password";
    try {
      if (step === "email" || resend) {
        await action("request-code", { email }, controller.signal);
        if (controller.signal.aborted) return;
        setCode(""); setResendSeconds(60); setStep("code");
      } else if (step === "code") {
        await action("verify-code", { email, code: submittedCode }, controller.signal);
        if (controller.signal.aborted) return;
        setCode(""); setStep("password");
        shouldRegister = passwordFirst;
      }
      if (shouldRegister) {
        registrationAttempted = true;
        completionResponse = await action("register", { email, password, ...(legalMetadata?.published ? { registration: { version: legalMetadata.version, checksum: legalMetadata.checksum, locale } } : {}) }, controller.signal) ?? null;
        if (controller.signal.aborted) return;
        if (preview) setStep("preview-complete");
        else { setAccepted(completionResponse); setStep("session"); setPassword(""); setConfirmation(""); }
      }
      if (completionResponse && (shouldRegister || step === "session")) {
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
      if (failure instanceof RegistrationFailure && failure.documentsChanged) { setError(t.legal.changed); return; }
      if (registrationAttempted && (status === 0 || status >= 500 || status === 429)) {
        // The account may already exist while session issuance failed. Keep the
        // first submitted password for retry; never imply a new one replaces it.
        clearPassword = false; setPasswordRetry(true);
        setError(status === 429 ? t.auth.registrationRateLimited : t.auth.registrationRetryPassword);
        return;
      }
      setError(status === 429 ? t.auth.registrationRateLimited : status === 409 ? t.auth.registrationExisting : registrationAttempted && status === 400 ? t.auth.registrationExpired : step === "code" && status === 400 ? t.auth.registrationCodeFailure : t.auth.registrationFailure);
    } finally {
      request.current = null;
      if (registrationAttempted && clearPassword) {
        setPassword(""); setConfirmation(""); setPasswordRetry(false);
        if (passwordFirst && !completionResponse && !preview && !controller.signal.aborted) setStep("email");
      }
      if (!controller.signal.aborted) setPending(false);
    }
  }
  function restart() { setStep("email"); setCode(""); setPassword(""); setConfirmation(""); setPasswordRetry(false); setError(""); setAccepted(null); setDiagnosticId(""); }
  function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); void perform(); }

  return <>
    <header className={styles.heading}><h1 id="login-title" tabIndex={-1}><BrandText>{title ?? t.auth.registrationTitle}</BrandText></h1>{passwordFirst ? null : <p>{t.auth.registrationDescription}</p>}</header>
    {preview ? <p className={passwordFirst ? styles.previewNote : styles.hint}>{t.auth.preview}</p> : null}
    {step === "preview-complete" ? <><p role="status">{t.auth.registrationPreviewComplete}</p><Button onClick={onBack}>{t.auth.back}</Button></> : <form className={styles.fields} onSubmit={submit}>
      <CredentialField label={t.login.emailLabel} id="signup-email" autoComplete="email" type="email" maxLength={254} required disabled={pending || step !== "email"} value={email} onChange={event => {setEmail(event.target.value);onEmailChange?.(event.target.value);}} />
      {passwordFirst && step === "email" ? <PasswordCreationField id="signup-password" email={email} value={password} disabled={pending} onChange={setPassword} /> : null}
      {step === "code" ? <><p className={styles.hint} role="status">{preview ? t.auth.registrationPreviewCode : t.auth.registrationCodeSent}</p><VerificationCodeField id="signup-code" value={code} onChange={setCode} onComplete={value=>void perform(false,value)} disabled={pending} invalid={Boolean(error)} /></> : null}
      {step === "password" && passwordFirst && pending ? <p className={styles.hint} role="status">{t.auth.registrationFinishing}</p> : null}
      {step === "password" && !passwordFirst ? <><p className={styles.hint}>{preview ? t.auth.registrationPreviewPassword : t.auth.registrationEmailVerified}</p><CredentialField label={t.login.passwordLabel} hint={t.auth.passwordHint} id="signup-password" autoComplete="new-password" type="password" minLength={8} maxLength={256} required disabled={pending} readOnly={passwordRetry} value={password} onChange={event => setPassword(event.target.value)} /><CredentialField label={t.auth.confirmPassword} id="signup-confirm-password" autoComplete="new-password" type="password" minLength={8} maxLength={256} required disabled={pending} readOnly={passwordRetry} value={confirmation} onChange={event => setConfirmation(event.target.value)} /></> : null}
      {error ? <StateNotice inline kind="error">{error}{diagnosticId ? <small className={styles.diagnostic}>{t.auth.loginDiagnostic} {diagnosticId}</small> : null}</StateNotice> : null}
      {error === t.auth.registrationExisting && onSignIn ? <button type="button" className={styles.textAction} onClick={()=>onSignIn(email)}>{t.login.submitLabel}</button> : null}
      {error === t.legal.changed ? <button type="button" className={styles.textAction} onClick={() => window.location.reload()}>{t.legal.reload}</button> : null}
      <Button type="submit" disabled={pending || (step === "code" && !/^\d{6}$/.test(code)) || (passwordFirst && step === "email" && !credentialsValid)}>{pending ? <><span aria-hidden="true"><LoadingIndicator label="" /></span>{accepted ? signInStage === "navigation" ? t.auth.loginOpening : t.auth.loginConfirming : t.auth.pending}</> : step === "session" ? t.auth.loginRetry : step === "email" ? passwordFirst ? t.auth.minimalSignupAction : t.auth.sendCode : step === "code" ? t.auth.verifyRegistrationEmail : t.auth.createAccount}</Button>
      <LegalNotice />
      <div className={styles.actions}><button type="button" className={styles.textAction} disabled={pending} onClick={onBack}>{t.auth.back}</button>{step !== "email" && step !== "session" ? <button type="button" className={styles.textAction} disabled={pending} onClick={restart}>{step === "password" ? t.auth.registrationRestart : t.auth.changeEmail}</button> : null}</div>
      {step === "code" ? <><button type="button" className={styles.textAction} disabled={pending || resendSeconds > 0} onClick={() => void perform(true)}>{t.auth.resend}{resendSeconds ? ` (${resendSeconds})` : ""}</button><p className={styles.accountPrompt}>{t.auth.mailHelpShort} <a href="mailto:support@epifera.com">{t.legal.support}</a></p></> : null}
    </form>}
  </>;
}
