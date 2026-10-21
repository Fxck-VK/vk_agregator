"use client";

import { BrandText } from "@/components/brand/BrandName/BrandName";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { useDictionary, useLocale, useMessages } from "@/i18n/LocaleProvider";
import { LanguageSwitcher } from "@/i18n/LanguageSwitcher";
import { classifySignInFailure, completeSignIn, reportSignInFailure, signInFailureText, signInResponseFailure, type SignInStage } from "@/lib/auth/sign-in-completion";
import { isPasswordTooLong } from "@/lib/auth/password";
import { defaultAuthMethods, providerNames, safeAuthorizationURL, visibleOAuthProvidersForUI, type AuthMethods, type OAuthProvider } from "@/lib/auth/methods";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { CredentialField } from "../CredentialField";
import { EmailRegistration } from "../EmailRegistration";
import styles from "./LoginForm.module.css";

type Step = "login" | "email" | "code" | "complete";
const providerOrder: Record<OAuthProvider, number> = { yandex: 0, vk: 1, apple: 2, telegram: 3, google: 4 };
export function LoginForm({ returnTo, methods = defaultAuthMethods, preview = false, oauthFailed = false, entry = "email" }: Readonly<{ returnTo?: string; methods?: AuthMethods; preview?: boolean; oauthFailed?: boolean; entry?: "email" | "choice" }>) {
  const t = useDictionary(); const locale = useLocale();
  const msg = useMessages();
  const providers = visibleOAuthProvidersForUI(methods.providers);
  const [step, setStep] = useState<Step>("login");
  const [registering, setRegistering] = useState(false);
  const [emailEntry, setEmailEntry] = useState(entry === "email");
  const formRoot = useRef<HTMLElement>(null);
  useEffect(() => {
    if (entry === "choice") formRoot.current?.querySelector<HTMLHeadingElement>("h1")?.focus({ preventScroll: true });
  }, [entry, emailEntry, registering, step]);
  const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [code, setCode] = useState("");
  const [pending, setPending] = useState(false); const [error, setError] = useState(oauthFailed ? t.auth.providerFailure : "");
  const [diagnosticId, setDiagnosticId] = useState("");
  const [accepted, setAccepted] = useState<Response | null>(null);
  const [signInStage, setSignInStage] = useState<SignInStage>("credentials");
  const request = useRef<AbortController | null>(null);
  useEffect(() => () => request.current?.abort(), []);
  const [resendSeconds, setResendSeconds] = useState(0);
  useEffect(() => {
    if (resendSeconds <= 0) return;
    const timer = window.setTimeout(() => setResendSeconds(value => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [resendSeconds]);
  const move = (next: Step) => { setStep(next); setPassword(""); setCode(""); setError(""); setAccepted(null); setDiagnosticId(""); setSignInStage("credentials"); };

  async function requestCode() {
    if (!preview) {
      const response = await webBrowserFetch("/web/v1/auth/password/request-reset", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email }) });
      if (!response.ok) throw new Error("recovery");
    }
    setResendSeconds(60); setStep("code");
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); if (pending || request.current) return;
    if (step === "code" && isPasswordTooLong(password)) { setError(t.auth.passwordTooLong); return; }
    setError(""); setDiagnosticId(""); setPending(true);
    const controller = new AbortController(); request.current = controller;
    const started = performance.now();
    let stage: SignInStage = accepted ? "session" : "credentials";
    try {
      if (step === "email") { await requestCode(); return; }
      if (step === "code") {
        if (!preview) {
          const response = await webBrowserFetch("/web/v1/auth/password/reset", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, code, new_password: password }) });
          if (!response.ok) throw new Error("recovery");
        }
        move("complete"); return;
      }
      if (preview) { setError(t.auth.preview); return; }
      let response = accepted;
      if (!response) {
        setSignInStage("credentials");
        response = await webBrowserFetch("/web/v1/auth/password/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, password }), signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15_000)]) });
        if (!response.ok) throw signInResponseFailure(response, "credentials");
        if (controller.signal.aborted) return;
        setAccepted(response); setPassword("");
      }
      await completeSignIn(response, { locale, returnTo, signal: controller.signal, onStage: next => { stage = next; setSignInStage(next); } });
    } catch (value) {
      if (controller.signal.aborted) return;
      if (step === "login") {
        const failure = classifySignInFailure(value, stage);
        reportSignInFailure(failure, started);
        setError(signInFailureText(failure, t.auth, t.login.failure)); setDiagnosticId(failure.diagnosticId);
      } else setError(t.auth.recoveryFailure);
    } finally {
      request.current = null;
      if (!controller.signal.aborted) { setPassword(""); setPending(false); }
    }
  };

  const oauth = async (provider: OAuthProvider) => {
    if (pending || preview) return; setError(""); setPending(true);
    try {
      const response = await webBrowserFetch(`/web/v1/auth/oauth/${provider}/start`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ locale }) });
      if (!response.ok) throw new Error("oauth");
      const value: unknown = await response.json();
      const target = safeAuthorizationURL(typeof value === "object" && value !== null && "authorization_url" in value ? value.authorization_url : null, provider);
      if (!target) throw new Error("oauth"); window.location.assign(target);
    } catch { setError(t.auth.providerFailure); setPending(false); }
  };

  const formClass = entry === "choice" ? `${styles.form} ${styles.windowForm}` : styles.form;
  const backToChoice = () => { move("login"); setEmailEntry(false); };

  if (entry === "choice" && !emailEntry && !registering) return <div className={styles.choiceEntry}>
    <section ref={formRoot} className={styles.choiceCard} aria-labelledby="login-title">
      <h1 id="login-title" tabIndex={-1}>{t.auth.choiceTitle}</h1>
      <div className={styles.choiceProviders}>
        {[...providers].sort((a, b) => providerOrder[a] - providerOrder[b]).map(provider => <Button className={styles.providerPill} key={provider} disabled={pending || preview} onClick={() => oauth(provider)}>
          {provider === "yandex" ? (
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" className={styles.providerMark} xmlns="http://www.w3.org/2000/svg" aria-hidden="true" focusable="false">
              <path d="M2.53992 12C2.53992 6.75315 6.79212 2.5 12.0399 2.5C17.2858 2.5 21.5399 6.75315 21.5399 12C21.5399 17.2469 17.2858 21.5 12.0399 21.5C6.79212 21.5 2.53992 17.2469 2.53992 12Z" fill="#FC3F1D" />
              <path d="M13.256 7.88312H12.3782C10.7689 7.88312 9.92248 8.69822 9.92248 9.89997C9.92248 11.2585 10.5077 11.895 11.7094 12.711L12.7022 13.3798L9.84933 17.6425H7.71753L10.2778 13.8292C8.80528 12.7737 7.97878 11.7487 7.97878 10.0149C7.97878 7.84132 9.49403 6.35742 12.3678 6.35742H15.2206V17.632H13.256V7.88312Z" fill="white" />
            </svg>
          ) : provider === "vk" ? (
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" className={styles.providerMark} xmlns="http://www.w3.org/2000/svg" aria-hidden="true" focusable="false">
              <path d="M3 11.62C3 7.32078 3 5.17119 4.33559 3.83559C5.67119 2.5 7.82078 2.5 12.12 2.5H12.88C17.1792 2.5 19.3288 2.5 20.6644 3.83559C22 5.17119 22 7.32078 22 11.62V12.38C22 16.6792 22 18.8288 20.6644 20.1644C19.3288 21.5 17.1792 21.5 12.88 21.5H12.12C7.82078 21.5 5.67119 21.5 4.33559 20.1644C3 18.8288 3 16.6792 3 12.38V11.62Z" fill="#0077FF" />
              <path d="M13.1096 16.1861C8.77918 16.1861 6.30922 13.2173 6.2063 8.27734H8.37547C8.44672 11.9032 10.0458 13.439 11.3125 13.7557V8.27734H13.3551V11.4044C14.6059 11.2698 15.9199 9.84484 16.3633 8.27734H18.4058C18.0654 10.209 16.6404 11.634 15.6271 12.2198C16.6404 12.6948 18.2634 13.9378 18.8809 16.1861H16.6325C16.1496 14.6819 14.9463 13.5182 13.3551 13.3598V16.1861H13.1096Z" fill="white" />
            </svg>
          ) : <span aria-hidden="true" className={styles.providerMark}>{providerNames[provider].slice(0, 1)}</span>}
          {provider === "yandex" ? t.auth.yandexEntry : provider === "vk" ? t.auth.vkEntry : `${t.auth.providerEntry} ${providerNames[provider]}`}
        </Button>)}
      </div>
      {providers.length ? <div className={styles.divider}><span>{t.auth.choiceOr}</span></div> : null}
      <Button className={styles.emailPill} disabled={pending || !(methods.password || methods.registration || methods.recovery)} onClick={() => { move("login"); setEmailEntry(true); }}>{t.auth.emailEntry}</Button>
      {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
      <p className={styles.choiceNote}>{t.auth.choiceNote}</p>
      {preview ? <p className={styles.previewNote}>{t.auth.preview}</p> : null}
    </section>
    <aside className={styles.helpCard}>
      <span className={styles.helpIcon} aria-hidden="true">?</span><div><h2>{t.auth.helpTitle}</h2><details><summary>{t.auth.helpAction}</summary><p>{t.auth.helpDescription}</p></details></div>
    </aside>
  </div>;

  if (registering) return <section ref={formRoot} className={formClass} aria-labelledby="login-title">{entry === "email" ? <LanguageSwitcher /> : null}<EmailRegistration preview={preview} returnTo={returnTo} title={entry === "choice" ? t.auth.registrationEntry : undefined} onBack={() => setRegistering(false)} /></section>;

  return <section ref={formRoot} className={formClass} aria-labelledby="login-title">
    {entry === "email" ? <LanguageSwitcher /> : <button className={`${styles.textAction} ${styles.backAction}`} disabled={pending} type="button" onClick={backToChoice}><span aria-hidden="true">← </span>{t.auth.otherMethods}</button>}
    <header className={styles.heading}><h1 id="login-title" tabIndex={-1}><BrandText>{step === "login" ? entry === "choice" ? t.auth.emailEntry : t.auth.title : t.auth.recoveryTitle}</BrandText></h1><p>{step === "login" ? t.auth.description : t.auth.recoveryDescription}</p></header>
    {preview ? <p className={styles.hint}>{t.auth.preview}</p> : null}
    {step === "complete" ? <><p role="status">{preview ? t.auth.previewComplete : t.auth.resetSuccess}</p><Button onClick={() => move("login")}>{t.auth.back}</Button></> : <form className={styles.fields} onSubmit={submit}>
      <CredentialField label={t.login.emailLabel} autoComplete="email" disabled={pending || step === "code" || Boolean(accepted)} id="login-email" name="email" onChange={e => setEmail(e.target.value)} required type="email" value={email} />
      {step === "code" ? <><p className={styles.hint} role="status">{t.auth.codeSent}</p><CredentialField label={t.auth.code} autoComplete="one-time-code" disabled={pending} id="recovery-code" inputMode="numeric" maxLength={6} pattern="[0-9]{6}" required onChange={e => setCode(e.target.value)} value={code} /></> : null}
      {step !== "email" && !accepted ? <CredentialField label={step === "code" ? t.auth.newPassword : t.login.passwordLabel} hint={step === "code" ? t.auth.passwordHint : undefined} autoComplete={step === "code" ? "new-password" : "current-password"} disabled={pending} id="login-password" name="password" minLength={step === "code" ? 8 : undefined} maxLength={256} onChange={e => setPassword(e.target.value)} required type="password" value={password} /> : null}
      {error ? <StateNotice inline kind="error">{error}{diagnosticId ? <small className={styles.diagnostic}>{t.auth.loginDiagnostic} {diagnosticId}</small> : null}</StateNotice> : null}
      <Button disabled={pending || (step === "login" && !methods.password)} type="submit">{pending ? <span aria-hidden="true"><LoadingIndicator label="" /></span> : null}{pending ? (step === "login" ? signInStage === "session" ? t.auth.loginConfirming : signInStage === "navigation" ? t.auth.loginOpening : t.login.pending : t.auth.pending) : step === "login" ? accepted ? t.auth.loginRetry : t.login.submitLabel : step === "email" ? t.auth.sendCode : t.auth.savePassword}</Button>
      {step === "login" && !accepted ? <button className={styles.textAction} disabled={pending || !methods.recovery} onClick={() => move("email")} type="button">{t.auth.forgot}</button> : <div className={styles.actions}><button className={styles.textAction} disabled={pending} onClick={() => move("login")} type="button">{t.auth.back}</button>{step === "code" ? <button className={styles.textAction} disabled={pending} onClick={() => move("email")} type="button">{t.auth.changeEmail}</button> : null}</div>}
      {step === "code" ? <button className={styles.textAction} disabled={pending || resendSeconds > 0} type="button" onClick={async () => {
        setPending(true); setError(""); try { await requestCode(); setCode(""); } catch { setError(t.auth.recoveryFailure); } finally { setPending(false); }
      }}>{t.auth.resend}{resendSeconds > 0 ? ` (${resendSeconds})` : ""}</button> : null}
    </form>}
    {entry === "email" && step === "login" && providers.length ? <div className={styles.external}><p>{t.auth.or}</p><div className={styles.providers}>{providers.map(provider => <Button key={provider} variant="outline" disabled={pending || preview} onClick={() => oauth(provider)}>{provider === "yandex" ? msg("profileLoginMethods.yandex") : providerNames[provider]}</Button>)}</div>{preview ? <small>{t.auth.configuredOnly}</small> : null}</div> : null}
    {step === "login" && !methods.recovery ? <small className={styles.hint}>{t.auth.recoveryUnavailable}</small> : null}
    {step === "login" && methods.registration ? <Button variant="outline" disabled={pending} onClick={() => { move("login"); setRegistering(true); }}>{t.auth.createAccount}</Button> : null}
  </section>;
}
