"use client";

import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button/Button";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { CredentialField } from "@/features/auth/CredentialField";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";
import { providerNames, safeAuthorizationURL, type AuthMethods, type OAuthProvider } from "@/lib/auth/methods";
import { accountSessionsSchema, type AccountSession } from "@/lib/auth/sessions";
import { isPasswordTooLong } from "@/lib/auth/password";
import { accountProfileSchema, safeIdentityRefSchema, type AccountProfile } from "@/lib/web-api/contracts";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { announceAccountChange } from "@/lib/web-api/browser-session";
import { requestWorkspaceLogout } from "@/features/session/WorkspaceLogout/workspace-logout-request";
import { clearPrivateBrowserState } from "@/features/session/WorkspaceLogout/private-browser-state";
import styles from "./AccountSecurity.module.css";

type FormKind = "email" | "phone" | "password";
export function AccountSecurity({ profile: initialProfile, methods, preview = false, oauthStatus }: { profile: AccountProfile; methods: AuthMethods; preview?: boolean; oauthStatus?: string }) {
  const t = useDictionary(); const locale = useLocale(); const router = useRouter();
  const [profile, setProfile] = useState(initialProfile);
  const [sessions, setSessions] = useState<AccountSession[] | null>(null);
  const [sessionsFailed, setSessionsFailed] = useState(false); const [attempt, setAttempt] = useState(0);
  const [pending, setPending] = useState(false); const [error, setError] = useState(oauthStatus === "failed" ? t.auth.failure : "");
  const [message, setMessage] = useState(oauthStatus === "linked" ? t.auth.linked : "");
  const [form, setForm] = useState<FormKind | null>(null); const [sent, setSent] = useState(false);
  const [address, setAddress] = useState(""); const [code, setCode] = useState(""); const [password, setPassword] = useState("");
  const [currentPassword, setCurrentPassword] = useState(""); const [passwordConfirmation, setPasswordConfirmation] = useState("");
  const [confirm, setConfirm] = useState<{ kind: "identity" | "session"; id: string } | null>(null);
  useEffect(() => {
    const abort = new AbortController();
    webBrowserFetch("/web/v1/account/sessions", { signal: abort.signal }).then(async response => {
      if (!response.ok) throw new Error("sessions");
      const data = accountSessionsSchema.parse(await response.json());
      if (data.items.some(item => item.account_id !== initialProfile.account_id)) throw new Error("account");
      if (!abort.signal.aborted) { setSessions(data.items); setSessionsFailed(false); }
    }).catch(() => { if (!abort.signal.aborted) setSessionsFailed(true); });
    return () => abort.abort();
  }, [initialProfile.account_id, attempt]);

  async function mutation(path: `/web/v1/${string}`, body?: object, method = "POST") {
    const response = await webBrowserMutation(path, { method, headers: body ? { "Content-Type": "application/json" } : undefined, body: body ? JSON.stringify(body) : undefined });
    if (!response.ok) {
      if (path === "/web/v1/account/password/set") {
        const data: unknown = await response.json().catch(() => null);
        const code = typeof data === "object" && data !== null && "error" in data ? data.error : null;
        if (code === "password_confirmation_required") setProfile(value => ({ ...value, password_set: true }));
        setError(code === "current_password_invalid" ? t.auth.currentPasswordInvalid : code === "password_confirmation_required" ? t.auth.passwordConfirmationRequired : response.status === 429 ? t.auth.registrationRateLimited : t.auth.failure);
      } else setError(response.status === 409 ? (method === "DELETE" ? t.auth.lastIdentity : t.auth.conflict) : t.auth.failure);
      throw new Error("action");
    }
    return response;
  }
  async function refreshProfile() {
    const response = await webBrowserFetch("/web/v1/me"); if (!response.ok) throw new Error("profile");
    const next = accountProfileSchema.parse(await response.json()); if (next.account_id !== initialProfile.account_id) throw new Error("account");
    setProfile(next); router.refresh();
  }
  async function run(work: () => Promise<void>) {
    if (pending || preview) return; setPending(true); setError(""); setMessage("");
    try { await work(); } catch { setError(value => value || t.auth.failure); } finally { setPending(false); setPassword(""); setCurrentPassword(""); setPasswordConfirmation(""); }
  }
  const openForm = (kind: FormKind) => { setForm(kind); setAddress(""); setCode(""); setPassword(""); setCurrentPassword(""); setPasswordConfirmation(""); setSent(false); setError(""); setMessage(""); };
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (form === "password" && (isPasswordTooLong(password) || isPasswordTooLong(currentPassword))) { setError(t.auth.passwordTooLong); return; }
    if (form === "password" && password !== passwordConfirmation) { setError(t.auth.passwordMismatch); return; }
    void run(async () => {
      if (form === "password") {
        await mutation("/web/v1/account/password/set", { email: address, password, ...(profile.password_set ? { current_password: currentPassword } : {}) });
        setProfile(value => ({ ...value, password_set: true })); setMessage(t.auth.passwordSaved); setForm(null); router.refresh(); return;
      }
      const kind = form === "phone" ? "phone" : "email";
      const payload = kind === "phone" ? { phone: address } : { email: address };
      if (!sent) { await mutation(`/web/v1/account/identities/${kind}/${kind === "email" ? "request-code" : "request-otp"}`, payload); setSent(true); return; }
      const response = await mutation(`/web/v1/account/identities/${kind}/verify`, { ...payload, code });
      if (kind === "email") {
        const identity = safeIdentityRefSchema.parse(await response.json());
        if (identity.account_id !== initialProfile.account_id || identity.provider !== "email" || !identity.verified) throw new Error("identity");
        if (profile.identity_refs.some(item => item.id === identity.id)) {
          setSent(false); setCode(""); setError(t.auth.emailAlreadyAdded); await refreshProfile(); return;
        }
      }
      setCode(""); setForm(null); setMessage(kind === "email" && hasVerifiedEmail ? t.auth.backupEmailLinked : t.auth.linked); await refreshProfile();
    });
  };
  const linkProvider = (provider: OAuthProvider) => run(async () => {
    const response = await mutation(`/web/v1/account/oauth/${provider}/start`, { locale });
    const data: unknown = await response.json();
    const target = safeAuthorizationURL(typeof data === "object" && data !== null && "authorization_url" in data ? data.authorization_url : null, provider);
    if (!target) throw new Error("oauth"); window.location.assign(target);
  });
  async function remove() {
    if (!confirm) return;
    await run(async () => {
      if (confirm.kind === "identity") { await mutation(`/web/v1/account/identities/${confirm.id}`, undefined, "DELETE"); await refreshProfile(); }
      else {
        const current = sessions?.find(item => item.id === confirm.id)?.current;
        if (current) { await requestWorkspaceLogout(); clearPrivateBrowserState(); announceAccountChange(); router.replace("/login"); router.refresh(); return; }
        await mutation(`/web/v1/account/sessions/${confirm.id}/revoke`);
        setSessions(value => value?.filter(item => item.id !== confirm.id) ?? null);
      }
      setConfirm(null); setMessage(t.auth.done);
    });
  }
  const verified = profile.identity_refs.filter(item => item.verified);
  const hasVerifiedEmail = verified.some(item => item.provider === "email");
  const canUnlink = (id: string, provider: string) => provider === "phone" || verified.some(other => other.id !== id && other.provider !== "phone");
  const date = (value: string) => new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
  return <div className={styles.security}>
    {preview ? <StateNotice inline kind="info">{t.auth.preview}</StateNotice> : null}
    {error ? <StateNotice inline kind="error">{error}</StateNotice> : null}{message ? <StateNotice inline kind="success">{message}</StateNotice> : null}
    {pending ? <LoadingIndicator label={t.auth.pending} /> : null}
    <section className={styles.card} aria-labelledby="security-identities"><h2 id="security-identities">{t.auth.identities}</h2>
      <ul className={styles.list}>{verified.map(identity => <li className={styles.row} key={identity.id}><div className={styles.details}><strong>{Object.hasOwn(providerNames, identity.provider) ? providerNames[identity.provider as OAuthProvider] : identity.provider === "phone" ? t.auth.phone : identity.provider === "password" ? t.login.passwordLabel : t.login.emailLabel}</strong><span>{identity.label}</span></div><Button variant="outline" disabled={pending || preview || !canUnlink(identity.id, identity.provider)} onClick={() => setConfirm({ kind: "identity", id: identity.id })}>{t.auth.unlink}</Button></li>)}</ul>
      {verified.filter(identity => identity.provider !== "phone").length <= 1 ? <p>{t.auth.lastIdentity}</p> : null}
      <div className={styles.actions}>
        {methods.email_link ? <Button variant="outline" disabled={pending} onClick={() => openForm("email")}>{hasVerifiedEmail ? t.auth.backupEmailAdd : t.auth.emailLink}</Button> : null}
        {methods.phone_link ? <Button variant="outline" disabled={pending} onClick={() => openForm("phone")}>{t.auth.phoneLink}</Button> : null}
        {methods.providers.filter(provider => !verified.some(identity => identity.provider === provider)).map(provider => <Button variant="outline" disabled={pending || preview} key={provider} onClick={() => linkProvider(provider)}>{t.auth.link} {providerNames[provider]}</Button>)}
        {methods.password && typeof profile.password_set === "boolean" && verified.some(item => item.provider === "email") ? <Button variant="outline" disabled={pending} onClick={() => openForm("password")}>{profile.password_set ? t.auth.passwordChange : t.auth.passwordSetup}</Button> : null}
      </div>
      {form ? <form className={styles.form} onSubmit={submit}>
        {form === "email" && hasVerifiedEmail ? <p>{t.auth.backupEmailDescription}</p> : null}
        {form === "password" ? <p>{profile.password_set ? t.auth.passwordChangeDescription : t.auth.passwordSetupDescription}</p> : null}
        <CredentialField label={form === "phone" ? t.auth.phone : t.login.emailLabel} id="security-address" type={form === "phone" ? "tel" : "email"} autoComplete={form === "phone" ? "tel" : "email"} required disabled={pending || sent} value={address} onChange={e => setAddress(e.target.value)} />
        {sent ? <><p role="status">{t.auth.verificationSent}</p><CredentialField label={form === "phone" ? t.auth.smsCode : t.auth.code} id="security-code" autoComplete="one-time-code" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} required disabled={pending} value={code} onChange={e => setCode(e.target.value)} /></> : null}
        {form === "password" ? <>
          {profile.password_set ? <CredentialField label={t.auth.currentPassword} id="security-current-password" type="password" autoComplete="current-password" maxLength={256} required disabled={pending} value={currentPassword} onChange={e => setCurrentPassword(e.target.value)} /> : null}
          <CredentialField label={t.auth.newPassword} hint={t.auth.passwordHint} id="security-password" type="password" autoComplete="new-password" minLength={8} maxLength={256} required disabled={pending} value={password} onChange={e => setPassword(e.target.value)} />
          <CredentialField label={t.auth.confirmPassword} id="security-password-confirmation" type="password" autoComplete="new-password" minLength={8} maxLength={256} required disabled={pending} value={passwordConfirmation} onChange={e => setPasswordConfirmation(e.target.value)} />
        </> : null}
        <div className={styles.actions}><Button type="submit" disabled={pending || preview}>{form === "password" ? t.auth.savePassword : sent ? t.auth.verify : t.auth.sendCode}</Button><Button variant="outline" disabled={pending} onClick={() => { setForm(null); setPassword(""); setCurrentPassword(""); setPasswordConfirmation(""); setCode(""); setAddress(""); }}>{t.auth.cancel}</Button></div>
      </form> : null}
    </section>
    <section className={styles.card} aria-labelledby="security-sessions"><h2 id="security-sessions">{t.auth.sessions}</h2>
      {sessionsFailed ? <StateNotice inline kind="error" action={{ label: t.auth.retry, onClick: () => setAttempt(value => value + 1) }}>{t.auth.failure}</StateNotice> : sessions === null ? <LoadingIndicator label={t.auth.pending} /> : sessions.length === 0 ? <p>{t.auth.noSessions}</p> : <ul className={styles.list}>{sessions.map(session => <li className={styles.row} key={session.id}><div className={styles.details}><strong>{session.current ? t.auth.currentSession : t.auth.otherSession}</strong><span>{t.auth.sessionCreated}: <time dateTime={session.created_at}>{date(session.created_at)}</time></span><span>{t.auth.sessionExpires}: <time dateTime={session.expires_at}>{date(session.expires_at)}</time></span></div><Button variant="outline" disabled={pending || preview} onClick={() => setConfirm({ kind: "session", id: session.id })}>{t.auth.revoke}</Button></li>)}</ul>}
    </section>
    {confirm ? <StateNotice inline kind="info"><div className={styles.confirm}><p>{confirm.kind === "identity" ? t.auth.confirmUnlink : t.auth.confirmRevoke}</p><div className={styles.actions}><Button disabled={pending || preview} onClick={remove}>{confirm.kind === "identity" ? t.auth.unlink : t.auth.revoke}</Button><Button variant="outline" disabled={pending} onClick={() => setConfirm(null)}>{t.auth.cancel}</Button></div></div></StateNotice> : null}
  </div>;
}
