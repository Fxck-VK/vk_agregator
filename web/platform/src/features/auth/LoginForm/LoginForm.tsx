"use client";

import { type FormEvent, useEffect, useState } from "react";
import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { LanguageSwitcher } from "@/i18n/LanguageSwitcher";
import { useRouter } from "@/i18n/navigation";
import { safeReturnPath } from "@/lib/auth/return-path";
import { isPasswordTooLong } from "@/lib/auth/password";
import { defaultAuthMethods, providerNames, safeAuthorizationURL, type AuthMethods, type OAuthProvider } from "@/lib/auth/methods";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { CredentialField } from "../CredentialField";
import { EmailRegistration } from "../EmailRegistration";
import styles from "./LoginForm.module.css";

type Step = "login" | "email" | "code" | "complete";
export function LoginForm({ returnTo, methods = defaultAuthMethods, preview = false, oauthFailed = false }: Readonly<{ returnTo?: string; methods?: AuthMethods; preview?: boolean; oauthFailed?: boolean }>) {
  const t = useDictionary(); const locale = useLocale(); const router = useRouter();
  const [step, setStep] = useState<Step>("login");
  const [registering, setRegistering] = useState(false);
  const [email, setEmail] = useState(""); const [password, setPassword] = useState(""); const [code, setCode] = useState("");
  const [pending, setPending] = useState(false); const [error, setError] = useState(oauthFailed ? t.auth.providerFailure : "");
  const [resendSeconds, setResendSeconds] = useState(0);
  useEffect(() => {
    if (resendSeconds <= 0) return;
    const timer = window.setTimeout(() => setResendSeconds(value => Math.max(0, value - 1)), 1000);
    return () => window.clearTimeout(timer);
  }, [resendSeconds]);
  const move = (next: Step) => { setStep(next); setPassword(""); setCode(""); setError(""); };

  async function requestCode() {
    if (!preview) {
      const response = await webBrowserFetch("/web/v1/auth/password/request-reset", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email }) });
      if (!response.ok) throw new Error("recovery");
    }
    setResendSeconds(60); setStep("code");
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); if (pending) return;
    if (step === "code" && isPasswordTooLong(password)) { setError(t.auth.passwordTooLong); return; }
    setError(""); setPending(true);
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
      const response = await webBrowserFetch("/web/v1/auth/password/login", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email, password }) });
      if (!response.ok) throw new Error("login");
      router.replace(safeReturnPath(returnTo ?? "") ?? "/app");
    } catch { setError(step === "login" ? t.login.failure : t.auth.recoveryFailure); }
    finally { setPassword(""); setPending(false); }
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

  if (registering) return <section className={styles.form} aria-labelledby="login-title"><LanguageSwitcher /><EmailRegistration preview={preview} returnTo={returnTo} onBack={() => setRegistering(false)} /></section>;

  return <section className={styles.form} aria-labelledby="login-title">
    <LanguageSwitcher />
    <header className={styles.heading}><h1 id="login-title">{step === "login" ? t.auth.title : t.auth.recoveryTitle}</h1><p>{step === "login" ? t.auth.description : t.auth.recoveryDescription}</p></header>
    {preview ? <p className={styles.hint}>{t.auth.preview}</p> : null}
    {step === "complete" ? <><p role="status">{preview ? t.auth.previewComplete : t.auth.resetSuccess}</p><Button onClick={() => move("login")}>{t.auth.back}</Button></> : <form className={styles.fields} onSubmit={submit}>
      <CredentialField label={t.login.emailLabel} autoComplete="email" disabled={pending || step === "code"} id="login-email" name="email" onChange={e => setEmail(e.target.value)} required type="email" value={email} />
      {step === "code" ? <><p className={styles.hint} role="status">{t.auth.codeSent}</p><CredentialField label={t.auth.code} autoComplete="one-time-code" disabled={pending} id="recovery-code" inputMode="numeric" maxLength={6} pattern="[0-9]{6}" required onChange={e => setCode(e.target.value)} value={code} /></> : null}
      {step !== "email" ? <CredentialField label={step === "code" ? t.auth.newPassword : t.login.passwordLabel} hint={step === "code" ? t.auth.passwordHint : undefined} autoComplete={step === "code" ? "new-password" : "current-password"} disabled={pending} id="login-password" name="password" minLength={step === "code" ? 8 : undefined} maxLength={256} onChange={e => setPassword(e.target.value)} required type="password" value={password} /> : null}
      {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}
      <Button disabled={pending || (step === "login" && !methods.password)} type="submit">{pending ? <span aria-hidden="true"><LoadingIndicator label="" /></span> : null}{pending ? (step === "login" ? t.login.pending : t.auth.pending) : step === "login" ? t.login.submitLabel : step === "email" ? t.auth.sendCode : t.auth.savePassword}</Button>
      {step === "login" ? <button className={styles.textAction} disabled={pending || !methods.recovery} onClick={() => move("email")} type="button">{t.auth.forgot}</button> : <div className={styles.actions}><button className={styles.textAction} disabled={pending} onClick={() => move("login")} type="button">{t.auth.back}</button>{step === "code" ? <button className={styles.textAction} disabled={pending} onClick={() => move("email")} type="button">{t.auth.changeEmail}</button> : null}</div>}
      {step === "code" ? <button className={styles.textAction} disabled={pending || resendSeconds > 0} type="button" onClick={async () => {
        setPending(true); setError(""); try { await requestCode(); setCode(""); } catch { setError(t.auth.recoveryFailure); } finally { setPending(false); }
      }}>{t.auth.resend}{resendSeconds > 0 ? ` (${resendSeconds})` : ""}</button> : null}
    </form>}
    {step === "login" && methods.providers.length ? <div className={styles.external}><p>{t.auth.or}</p><div className={styles.providers}>{methods.providers.map(provider => <Button key={provider} variant="outline" disabled={pending || preview} onClick={() => oauth(provider)}>{providerNames[provider]}</Button>)}</div>{preview ? <small>{t.auth.configuredOnly}</small> : null}</div> : null}
    {step === "login" && !methods.recovery ? <small className={styles.hint}>{t.auth.recoveryUnavailable}</small> : null}
    {step === "login" && methods.registration ? <Button variant="outline" disabled={pending} onClick={() => { move("login"); setRegistering(true); }}>{t.auth.createAccount}</Button> : null}
  </section>;
}
