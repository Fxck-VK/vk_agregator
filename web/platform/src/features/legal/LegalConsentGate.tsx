"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button/Button";
import { LoadingIndicator, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";
import { usePathname } from "@/i18n/navigation";
import Link from "@/i18n/Link";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";
import { useOptionalWorkspaceLogout } from "@/features/session/WorkspaceLogout/WorkspaceLogoutBoundary";
import { legalStatusSchema, sameLegalRevision, type LegalMetadata, type LegalStatus } from "./model";
import styles from "./Legal.module.css";

type Props = { metadata: LegalMetadata; accountId: string; preview?: boolean; children?: ReactNode; mode?: "gate" | "preferences" };
export function LegalConsentGate({ metadata, accountId, preview = false, children, mode = "gate" }: Props) {
  return <LegalChoice key={`${accountId}:${metadata.checksum}:${mode}`} metadata={metadata} accountId={accountId} preview={preview} mode={mode}>{children}</LegalChoice>;
}
function LegalChoice({ metadata, accountId, preview = false, children, mode = "gate" }: Props) {
  const { legal: t } = useDictionary();
  const locale = useLocale();
  const pathname = usePathname();
  const logout = useOptionalWorkspaceLogout();
  const [status, setStatus] = useState<LegalStatus | null>(null);
  const [failed, setFailed] = useState(false);
  const [pending, setPending] = useState(false);
  const [optIn, setOptIn] = useState(false);
  const [saved, setSaved] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const request = useRef<{ key: string; body: string; path: string } | null>(null);
  const currentRequest = useRef<AbortController | null>(null);
  useEffect(() => {
    if (preview || !metadata.published) return;
    const controller = new AbortController();
    void webBrowserFetch("/web/v1/account/legal", { signal: controller.signal }).then(async response => {
      if (!response.ok) throw new Error("legal unavailable");
      const result = legalStatusSchema.parse(await response.json());
      if (!controller.signal.aborted) {
        setStatus(result);
        setOptIn(mode === "preferences" && result.marketing_opt_in);
      }
    }).catch(() => { if (!controller.signal.aborted) setFailed(true); });
    return () => controller.abort();
  }, [accountId, metadata.published, preview, attempt, mode]);
  useEffect(() => () => currentRequest.current?.abort(), []);

  // Security, email changes and withdrawal remain accessible without accepting.
  if (mode === "gate" && (preview || !metadata.published || pathname === "/app/profile")) return children;
  if (mode === "gate" && status?.accepted && sameLegalRevision(metadata, status)) return children;
  if (mode === "gate" && (pathname === "/app/files" || pathname === "/app/chats" || pathname?.startsWith("/app/chat/"))) {
    return <>{status || failed ? <aside className={styles.reminder}><p>{t.existingContent}</p><Link href="/app">{t.reviewTerms}</Link></aside> : null}{children}</>;
  }
  const drift = status !== null && !sameLegalRevision(metadata, status);
  async function save(withdraw = false) {
    if (pending || preview || (!withdraw && (!metadata.published || !status || drift))) return;
    setPending(true); setFailed(false); setSaved(false);
    const accepting = mode === "gate" && !withdraw;
    const body = JSON.stringify(accepting ? {
      version: metadata.version, checksum: metadata.checksum, accepted: true,
      marketing_version: metadata.marketing_version, marketing_checksum: metadata.marketing_checksum,
      marketing_opt_in: optIn, recipient_version: status?.recipient_version ?? "", locale,
    } : {
      version: metadata.marketing_version, checksum: metadata.marketing_checksum,
      opt_in: withdraw ? false : optIn, recipient_version: status?.recipient_version ?? "", locale,
    });
    const path = accepting ? "/web/v1/account/legal/accept" : "/web/v1/account/legal/marketing";
    // Lost-response retries keep the exact key and payload. A different action,
    // recipient or document revision gets a new key.
    if (request.current?.body !== body || request.current.path !== path) {
      request.current = { body, path, key: crypto.randomUUID() };
    }
    const controller = new AbortController();
    currentRequest.current = controller;
    try {
      const response = await webBrowserMutation(path, {
        method: "POST", headers: { "Content-Type": "application/json", "X-Idempotency-Key": request.current.key },
        body, signal: controller.signal,
      });
      if (!response.ok) throw new Error("legal choice failed");
      const result = legalStatusSchema.parse(await response.json());
      if (!withdraw && !sameLegalRevision(metadata, result)) throw new Error("legal documents changed");
      if (!controller.signal.aborted) {
        setStatus(result); setOptIn(result.marketing_opt_in); setSaved(true); request.current = null;
      }
    } catch { if (!controller.signal.aborted) setFailed(true); }
    finally { if (!controller.signal.aborted) setPending(false); }
  }
  const disabled = preview || !metadata.published;
  return <section className={mode === "gate" ? styles.gate : styles.preferences} aria-labelledby={`legal-${mode}-title`}>
    <div className={styles.choice}>
      <h2 id={`legal-${mode}-title`}>{mode === "gate" ? t.choiceTitle : t.preferences}</h2>
      {mode === "gate" ? <><p>{t.choiceDescription}</p><p className={styles.muted}>{t.personalData}</p></> : null}
      <div className={styles.documentLinks}>{["user-agreement", "privacy-policy", "paid-access", "payment-details"].map(slug => <Link key={slug} href={`/legal/${slug}`} target="_blank" rel="noopener noreferrer">{t.titles[slug as keyof typeof t.titles]}</Link>)}</div>
      {disabled ? <p>{preview ? t.preview : t.paused}</p> : drift ? <StateNotice inline kind="error">{t.changed}<Button variant="outline" onClick={() => window.location.reload()}>{t.reload}</Button></StateNotice> : !status && !failed ? <LoadingIndicator label={t.loading} /> : <>
        {status ? <>
          <label className={styles.check}>
            <input type="checkbox" checked={optIn} disabled={pending || !status.marketing_available || !status.recipient_version} onChange={event => { setOptIn(event.target.checked); setSaved(false); }} />
            <span>{t.advertising}<small>{t.optional} <Link href="/legal/marketing-consent" target="_blank" rel="noopener noreferrer">{t.titles["marketing-consent"]}</Link></small></span>
          </label>
          {!status.marketing_available ? <p className={styles.muted}>{t.noEmail}</p> : null}
          <Button disabled={pending} onClick={() => void save()}>{pending ? t.pending : mode === "gate" ? t.accept : t.save}</Button>
        </> : null}
      </>}
      {mode === "preferences" && !preview ? <Button variant="outline" disabled={pending} onClick={() => void save(true)}>{t.withdraw}</Button> : null}
      {failed ? <StateNotice inline kind="error">{t.error}{!status && !disabled ? <Button variant="outline" onClick={() => { setFailed(false); setAttempt(value => value + 1); }}>{t.retry}</Button> : null}</StateNotice> : null}
      {saved ? <p role="status">{t.saved}</p> : null}
      <div className={styles.actions}><a href="mailto:support@epifera.com">{t.support}</a>{mode === "gate" && logout ? <Button variant="outline" onClick={logout.logout}>{t.logout}</Button> : null}</div>
    </div>
  </section>;
}
