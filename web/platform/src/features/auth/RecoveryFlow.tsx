"use client";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import Link from "@/i18n/Link";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { isPasswordTooLong } from "@/lib/auth/password";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { CredentialField } from "./CredentialField";
import { PasswordField } from "./PasswordField";
import formStyles from "./LoginForm/LoginForm.module.css";
import styles from "./RecoveryFlow.module.css";

type Stage = "request" | "sent" | "checking" | "ready" | "expired" | "unavailable" | "success";
class RecoveryFailure extends Error {
  constructor(readonly status: number, readonly retrySeconds = 60) { super("recovery"); }
}

function retryDelay(response: Response) {
  const header = response.headers.get("Retry-After");
  if (!header) return 60;
  const seconds = /^\d+$/.test(header) ? Number(header) : Math.ceil((Date.parse(header) - Date.now()) / 1000);
  return Number.isFinite(seconds) ? Math.min(3600, Math.max(1, seconds)) : 60;
}

const bindingPattern = /^[a-f0-9]{64}$/;
function historyResetBinding() {
  const value = window.history.state?.epiferaResetBinding;
  return typeof value === "string" && bindingPattern.test(value) ? value : "";
}
function saveResetBinding(binding = "") {
  // This public digest is only a per-tab concurrency guard. The authenticated
  // reset proof stays in the server-owned HttpOnly cookie.
  const current = window.history.state;
  const state = current && typeof current === "object" && !Array.isArray(current) ? { ...current } : {};
  delete state.epiferaResetBinding;
  if (binding) state.epiferaResetBinding = binding;
  window.history.replaceState(state, "", window.location.pathname + window.location.search);
}
async function responseResetBinding(response: Response) {
  const data: unknown = await response.json();
  const binding = data && typeof data === "object" && "binding" in data ? data.binding : undefined;
  if (typeof binding !== "string" || !bindingPattern.test(binding)) throw new RecoveryFailure(503);
  return binding;
}

function consumeResetFragment() {
  const fragment = window.location.hash;
  if (!fragment) return { token: "", malformed: false };
  // Only the opaque token enters memory. Remove the fragment before any API
  // request, history navigation or rendering of the reset form.
  saveResetBinding();
  const token = new URLSearchParams(fragment.slice(1)).get("token") ?? "";
  return { token, malformed: !token || token.length > 4096 };
}

export function RecoveryFlow({ mode, preview, onBack, initialEmail = "", onEmailChange, supportHref = "mailto:support@epifera.com" }: {
  mode: "request" | "reset";
  preview: boolean;
  onBack: () => void;
  initialEmail?: string;
  onEmailChange?: (email: string) => void;
  supportHref?: string;
}) {
  const { auth: t, login, legal } = useDictionary();
  const locale = useLocale();
  const [stage, setStage] = useState<Stage>(mode === "request" ? "request" : preview ? "ready" : "checking");
  const [email, setEmail] = useState(initialEmail);
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState(false);
  const [samePasswordRetry, setSamePasswordRetry] = useState(false);
  const [error, setError] = useState("");
  const [resendSeconds, setResendSeconds] = useState(0);
  const [validationAttempt, setValidationAttempt] = useState(0);
  const incoming = useRef<ReturnType<typeof consumeResetFragment> | null>(null);
  const binding = useRef("");
  const mutation = useRef<AbortController | null>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const emailValid = email.trim().length <= 254 && /^[^\s@]+@[^\s@]+\.[^\s@]+$/u.test(email.trim());
  const mismatch = Boolean(confirmation) && confirmation !== password;
  const passwordValid = password.length >= 8 && Boolean(password.trim()) && !isPasswordTooLong(password) && password === confirmation;

  useEffect(() => () => mutation.current?.abort(), []);
  useEffect(() => { heading.current?.focus({ preventScroll: true }); }, [stage]);
  useEffect(() => {
    if (!resendSeconds) return;
    const timer = window.setTimeout(() => setResendSeconds(value => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [resendSeconds]);
  useEffect(() => {
    if (mode !== "reset") return;
    incoming.current ??= consumeResetFragment();
    if (preview) { incoming.current.token = ""; return; }
    binding.current ||= historyResetBinding();
    const controller = new AbortController();
    const signal = AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]);
    void (async () => {
      try {
        if (incoming.current?.malformed) throw new RecoveryFailure(400);
        if (incoming.current?.token) {
          const response = await webBrowserFetch("/web/v1/auth/password/prepare-reset", {
            method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ token: incoming.current.token }), signal,
          });
          if (controller.signal.aborted) return;
          if (!response.ok) throw new RecoveryFailure(response.status);
          const preparedBinding = await responseResetBinding(response);
          if (controller.signal.aborted) return;
          binding.current = preparedBinding;
          saveResetBinding(preparedBinding);
          incoming.current.token = "";
        }
        const response = await webBrowserFetch("/web/v1/auth/password/reset-status", {
          cache: "no-store", signal,
          ...(binding.current ? { headers: { "X-NeiroHub-Reset-Binding": binding.current } } : {}),
        });
        if (controller.signal.aborted) return;
        if (!response.ok) throw new RecoveryFailure(response.status);
        const currentBinding = await responseResetBinding(response);
        if (controller.signal.aborted) return;
        if (binding.current && binding.current !== currentBinding) throw new RecoveryFailure(400);
        if (!binding.current) { binding.current = currentBinding; saveResetBinding(currentBinding); }
        if (!controller.signal.aborted) setStage("ready");
      } catch (failure) {
        if (controller.signal.aborted) return;
        const expired = failure instanceof RecoveryFailure && [400, 410].includes(failure.status);
        if (expired) { if (incoming.current) incoming.current.token = ""; binding.current = ""; saveResetBinding(); }
        setStage(expired ? "expired" : "unavailable");
      }
    })();
    return () => controller.abort();
  }, [mode, preview, validationAttempt]);

  async function sendLink(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (pending || mutation.current || !emailValid || resendSeconds > 0) return;
    setPending(true); setError("");
    const controller = new AbortController(); mutation.current = controller;
    try {
      if (!preview) {
        const response = await webBrowserFetch("/web/v1/auth/password/request-link", {
          method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: email.trim(), locale }),
          signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]),
        });
        if (!response.ok) throw new RecoveryFailure(response.status, retryDelay(response));
      }
      if (!controller.signal.aborted) { setStage("sent"); setResendSeconds(60); }
    } catch (failure) {
      if (controller.signal.aborted) return;
      const limited = failure instanceof RecoveryFailure && failure.status === 429;
      setError(limited ? t.registrationRateLimited : t.recoveryLinkUnavailable);
      if (limited && failure instanceof RecoveryFailure) setResendSeconds(failure.retrySeconds);
    } finally { mutation.current = null; if (!controller.signal.aborted) setPending(false); }
  }
  async function resetPassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending || mutation.current || !passwordValid) return;
    if (!preview && !binding.current) { setStage("expired"); return; }
    setPending(true); setError("");
    const controller = new AbortController(); mutation.current = controller;
    try {
      if (!preview) {
        const response = await webBrowserFetch("/web/v1/auth/password/reset-link", {
          method: "POST", headers: { "Content-Type": "application/json", "X-NeiroHub-Reset-Binding": binding.current }, body: JSON.stringify({ new_password: password }),
          signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]),
        });
        if (!response.ok) throw new RecoveryFailure(response.status);
      }
      if (!controller.signal.aborted) { setPassword(""); setConfirmation(""); setStage("success"); setSamePasswordRetry(false); binding.current = ""; saveResetBinding(); }
    } catch (failure) {
      if (controller.signal.aborted) return;
      const status = failure instanceof RecoveryFailure ? failure.status : 0;
      if ([400, 410].includes(status)) { setPassword(""); setConfirmation(""); setStage("expired"); setSamePasswordRetry(false); binding.current = ""; saveResetBinding(); }
      else if (status === 0 || status >= 500 || status === 429) {
        setSamePasswordRetry(true); setError(status === 429 ? t.registrationRateLimited : t.recoveryResetRetrySame);
      } else { setError(t.recoveryLinkUnavailable); }
    } finally { mutation.current = null; if (!controller.signal.aborted) setPending(false); }
  }
  const title = stage === "sent" ? t.recoveryLinkSentTitle : stage === "ready" ? t.recoverySetPasswordTitle : stage === "expired" ? t.recoveryLinkExpiredTitle : stage === "success" ? t.recoveryResetSuccessTitle : mode === "reset" ? t.recoveryCheckingTitle : t.recoveryTitle;
  return <section className={`${formStyles.minimalForm} ${styles.recovery}`} aria-labelledby="recovery-title">
    <header className={formStyles.heading}><h1 ref={heading} id="recovery-title" tabIndex={-1}>{title}</h1></header>
    {preview ? <p className={formStyles.previewNote}>{t.preview}</p> : null}
    {stage === "checking" ? <StateNotice inline kind="loading">{t.recoveryChecking}</StateNotice> : null}
    {stage === "unavailable" ? <><StateNotice inline kind="error">{t.recoveryLinkUnavailable}</StateNotice><Button onClick={() => { setStage("checking"); setValidationAttempt(value => value + 1); }}>{t.retry}</Button></> : null}
    {stage === "expired" ? <><p className={styles.message}>{t.recoveryLinkExpired}</p><Link className={styles.replacement} href="/login/recover">{t.recoveryRequestNewLink}</Link></> : null}
    {stage === "request" ? <><p className={styles.message}>{t.recoveryLinkDescription}</p><form className={formStyles.fields} onSubmit={sendLink}>
      <CredentialField id="recovery-email" label={login.emailLabel} name="email" type="email" autoComplete="email" maxLength={254} required disabled={pending} value={email} onChange={event => {setEmail(event.target.value);onEmailChange?.(event.target.value);}} />
      {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
      <Button type="submit" disabled={pending || !emailValid || resendSeconds > 0}>{pending ? <span aria-hidden="true"><LoadingIndicator label="" /></span> : null}{pending ? t.sending : t.recoverySendLink}{resendSeconds ? ` (${resendSeconds})` : ""}</Button>
    </form></> : null}
    {stage === "sent" ? <><p className={styles.message} role="status">{t.recoveryLinkSent}</p><p className={styles.address}>{email.trim()}</p><p className={styles.message}>{t.recoveryLinkDeliveryHint}</p>
      {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
      <div className={styles.actions}><Button disabled={pending || resendSeconds > 0} onClick={() => void sendLink()}>{pending ? t.sending : t.recoveryResendLink}{resendSeconds ? ` (${resendSeconds})` : ""}</Button><button type="button" className={formStyles.textAction} disabled={pending} onClick={() => { setStage("request"); setResendSeconds(0); setError(""); }}>{t.changeEmail}</button></div>
    </> : null}
    {stage === "ready" ? <><p className={styles.message}>{t.recoveryNewPasswordDescription}</p><form className={formStyles.fields} onSubmit={resetPassword}>
      <PasswordField id="recovery-new-password" label={t.newPassword} name="new-password" autoComplete="new-password" minLength={8} showStrength value={password} onChange={setPassword} disabled={pending} readOnly={samePasswordRetry} />
      <PasswordField id="recovery-confirm-password" label={t.confirmPassword} name="confirm-password" autoComplete="new-password" minLength={8} value={confirmation} onChange={setConfirmation} disabled={pending} readOnly={samePasswordRetry} aria-invalid={mismatch || undefined} aria-describedby={mismatch ? "recovery-password-mismatch" : undefined} />
      {mismatch ? <StateNotice id="recovery-password-mismatch" inline kind="error">{t.passwordMismatch}</StateNotice> : null}
      {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
      <Button type="submit" disabled={pending || !passwordValid}>{pending ? <span aria-hidden="true"><LoadingIndicator label="" /></span> : null}{pending ? t.pending : t.savePassword}</Button>
    </form></> : null}
    {stage === "success" ? <><span className={styles.successIcon} aria-hidden="true">✓</span><p className={styles.message} role="status">{t.resetSuccess}</p></> : null}
    <div className={styles.support}>{stage === "sent" || stage === "request" ? <Link href="/ai/docs/receiving-emails">{t.recoveryEmailHelp}</Link> : null}<a href={supportHref}>{legal.support}</a></div>
    <button className={formStyles.textAction} type="button" disabled={pending} onClick={() => { setPassword(""); setConfirmation(""); setSamePasswordRetry(false); onBack(); }}>{t.back}</button>
  </section>;
}
